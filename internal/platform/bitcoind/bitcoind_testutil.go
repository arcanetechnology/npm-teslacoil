package bitcoind

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcutil"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/asyncutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
)

const (
	retryAttempts      = 7
	retrySleepDuration = time.Millisecond * 100
)

// GetBitcoindConfig returns a bitcoind config suitable for testing purposes
func GetBitcoindConfig(t *testing.T) Config {
	return Config{
		P2pPort:        testutil.GetPortOrFail(t),
		RpcPort:        testutil.GetPortOrFail(t),
		User:           "rpc_user_for_tests",
		Password:       "rpc_pass_for_tests",
		ZmqPubRawTx:    fmt.Sprintf("tcp://0.0.0.0:%d", testutil.GetPortOrFail(t)),
		ZmqPubRawBlock: fmt.Sprintf("tcp://0.0.0.0:%d", testutil.GetPortOrFail(t)),
	}

}

// StartBitcoindOrFail starts a bitcoind node with the given configuration,
// with the data directory set to the users temporary directory. The function
// returns the created client, as well as a function that cleans up the operation
// (stopping the node and deleting the data directory).
func StartBitcoindOrFail(t *testing.T, conf Config) (bitcoin TeslacoilBitcoind, cleanup func() error) {
	tempDir, err := ioutil.TempDir("", "teslacoil-bitcoind-")
	if err != nil {
		testutil.FatalMsgf(t, "Could not create temporary bitcoind dir: %v", err)
	}
	args := []string{
		// "-printtoconsole", // if you want to see output of bitcoind, uncomment this
		"-datadir=" + tempDir,
		"-server",
		"-regtest",
		"-daemon",
		fmt.Sprintf("-port=%d", conf.P2pPort),
		"-rpcuser=" + conf.User,
		"-rpcpassword=" + conf.Password,
		fmt.Sprintf("-rpcport=%d", conf.RpcPort),
		"-txindex",
		"-debug=rpc",
		"-debug=zmq",
		"-addresstype=bech32", // default addresstype, necessary for using GetNewAddress()
		"-zmqpubrawtx=" + conf.ZmqPubRawTx,
		"-zmqpubrawblock=" + conf.ZmqPubRawBlock,
	}

	log.Debugf("Executing command: bitcoind %s", strings.Join(args, " "))
	cmd := exec.Command("bitcoind", args...)

	// pass bitcoind output to test log, wrapepd with a label
	cmd.Stderr = testutil.LogWriter{Label: "bitcoind", Level: logrus.ErrorLevel}
	cmd.Stdout = testutil.LogWriter{Label: "bitcoind", Level: logrus.DebugLevel}
	if err := cmd.Run(); err != nil {
		testutil.FatalMsgf(t, "Could not start bitcoind: %v", err)
	}

	pidFile := filepath.Join(tempDir, "regtest", "bitcoind.pid")

	readPidFile := func() error {
		_, err := os.Stat(pidFile)
		return err
	}
	if err := asyncutil.Retry(retryAttempts, retrySleepDuration, readPidFile); err != nil {
		testutil.FatalMsgf(t, "Could not read bitcoind pid file after %d attempts",
			retryAttempts)
	}

	pidBytes, err := ioutil.ReadFile(pidFile)
	if err != nil {
		testutil.FatalMsgf(t, "Couldn't read bitcoind pid: %s", err)
	}

	pidLines := strings.Split(string(pidBytes), "\n")
	pid, err := strconv.Atoi(pidLines[0])
	if err != nil {
		testutil.FatalMsgf(t, "Could not convert bitcoind pid bytes to int: %s", err)
	}

	log.Debugf("Started bitcoind client with pid %d", pid)

	retry := func() error {
		var err error
		bitcoin, err = NewConn(conf, 100*time.Millisecond)
		return err
	}
	if err := asyncutil.Retry(retryAttempts, retrySleepDuration, retry); err != nil {
		testutil.FatalMsg(t, err)
	}
	client := bitcoin.Btcctl()

	// await bitcoind startup
	if err := asyncutil.Retry(retryAttempts, retrySleepDuration, client.Ping); err != nil {
		testutil.FatalMsgf(t, "Could not communicate with bitcoind after %d attempts",
			retryAttempts)
	}

	cleanup = func() error {
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
			return errors.Wrap(err, "couldn't kill bitcoind process")
		}

		negativePing := func() error {
			err := client.Ping()
			if err == nil {
				return errors.New("was able to ping client")
			}
			return nil
		}

		// await bitcoind shutdown
		if err := asyncutil.Retry(retryAttempts, retrySleepDuration, negativePing); err != nil {
			return fmt.Errorf("could communicate with stopped bitcoind after %d attempts",
				retryAttempts)
		}

		log.Debug("Stopped bitcoind process")
		if err := os.RemoveAll(tempDir); err != nil {
			return errors.Wrapf(err, "could not delete bitcoind tmp directory %s", tempDir)
		}
		log.Debugf("Deleted bitcoind tmp directory %s", tempDir)

		bitcoin.StopZmq()
		return nil
	}
	return bitcoin, cleanup
}

// SendTxToSelf is a helper function for sending a tx easily to
// our own address
func SendTxToSelf(bitcoin TeslacoilBitcoind, amountBtc float64) (*chainhash.Hash, error) {
	b := bitcoin.Btcctl()
	address, err := b.GetNewAddress("")
	if err != nil {
		return nil, fmt.Errorf("could not GetNewAddress: %+v", err)
	}

	balance, err := b.GetBalance("*")
	if err != nil {
		return nil, fmt.Errorf("could not get balance: %+v", err)
	}
	if balance.ToBTC() <= amountBtc {
		return nil, fmt.Errorf("not enough balance, try using GenerateToSelf() first")
	}

	amount, _ := btcutil.NewAmount(amountBtc)
	txHash, err := b.SendToAddress(address, amount)
	if err != nil {
		return nil, fmt.Errorf("could not send to address %v: %v", address, err)
	}

	return txHash, nil
}

// ConvertToAddressOrFail converts a string address into a
// btcutil.Address type. If the conversion fails - the string
// is not an address for the given chain - we panic
func ConvertToAddressOrFail(address string, params chaincfg.Params) btcutil.Address {

	addr, err := btcutil.DecodeAddress(address, &params)
	if err != nil {
		panic(err)
	}

	return addr
}

// GenerateToSelf is a helper function for easily generating a block
// with the coinbase going to us
func GenerateToSelf(numBlocks uint32, bitcoin TeslacoilBitcoind) ([]*chainhash.Hash, error) {
	b := bitcoin.Btcctl()
	address, err := b.GetNewAddress("")
	if err != nil {
		return nil, errors.Wrap(err, "could not GetNewAddress")
	}

	hash, err := GenerateToAddress(bitcoin, numBlocks, address)
	if err != nil {
		return nil, errors.Wrap(err, "could not GenerateToAddress")
	}

	return hash, nil
}

// RunWithBitcoind lets you test functionality that requires actual bitcoind
// node by creating starting up bitcoind, running the test and then running
// the necessary cleanup.
func RunWithBitcoind(t *testing.T, test func(bitcoin TeslacoilBitcoind)) {
	bitcoindConf := GetBitcoindConfig(t)
	bitcoin, cleanupBitcoind := StartBitcoindOrFail(t, bitcoindConf)

	cleanup := func() error {
		bitcoindErr := cleanupBitcoind()
		if bitcoindErr != nil {
			return fmt.Errorf("failed to cleanup bitcoind: %s",
				bitcoindErr.Error())
		}
		return nil
	}

	test(bitcoin)

	if err := cleanup(); err != nil {
		t.Fatalf("Couldn't clean up after %q: %v", t.Name(), err)
	}
}
