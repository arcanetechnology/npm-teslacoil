package lntestutil

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/asyncutil"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/ln"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
)

var (
	log = build.Log
)

const (
	retryAttempts      = 7
	retrySleepDuration = time.Millisecond * 100
)

func GetRandomLightningMockClient() LightningMockClient {
	invoicePreimage := make([]byte, 32)
	_, _ = rand.Read(invoicePreimage)

	paymentResponsePreimage := make([]byte, 32)
	_, _ = rand.Read(paymentResponsePreimage)

	decodePayReqPreimage := make([]byte, 32)
	_, _ = rand.Read(decodePayReqPreimage)

	doubleHash := func(bytes []byte) []byte {
		first := sha256.Sum256(bytes)
		again := sha256.Sum256(first[:])
		return again[:]
	}

	return LightningMockClient{
		InvoiceResponse: lnrpc.Invoice{
			PaymentRequest: fmt.Sprintf("SomePayRequest%d", gofakeit.Number(0, 10000)),
			RHash:          doubleHash(invoicePreimage),
			RPreimage:      invoicePreimage,
			Settled:        true,
			Value:          int64(gofakeit.Number(1, ln.MaxAmountSatPerInvoice)),
		},
		SendPaymentSyncResponse: lnrpc.SendResponse{
			PaymentPreimage: paymentResponsePreimage,
			PaymentHash:     doubleHash(paymentResponsePreimage),
		},
		DecodePayReqResponse: lnrpc.PayReq{
			PaymentHash: hex.EncodeToString(doubleHash(decodePayReqPreimage)),
			NumSatoshis: int64(gofakeit.Number(1, ln.MaxAmountSatPerInvoice)),
			Description: "HelloPayment",
		},
	}
}

// GetLightningMockClient returns a basic LN client you can use where the
// content of the response does not matter at all
func GetLightningMockClient() LightningMockClient {

	var (
		SamplePreimageHex = "0123456789abcdef0123456789abcdef"
		SamplePreimage    = func() []byte {
			encoded, _ := hex.DecodeString(SamplePreimageHex)
			return encoded
		}()
		SampleHash = func() [32]byte {
			first := sha256.Sum256(SamplePreimage)
			return sha256.Sum256(first[:])
		}()
		SampleHashHex = hex.EncodeToString(SampleHash[:])
	)

	return LightningMockClient{
		InvoiceResponse: lnrpc.Invoice{
			PaymentRequest: "SomePayRequest1",
			RHash:          SampleHash[:],
			RPreimage:      SamplePreimage,
			Settled:        true,
			Value:          int64(271),
		},
		SendPaymentSyncResponse: lnrpc.SendResponse{
			PaymentPreimage: SamplePreimage,
			PaymentHash:     SampleHash[:],
		},
		DecodePayReqResponse: lnrpc.PayReq{
			PaymentHash: SampleHashHex,
			NumSatoshis: int64(1823472358),
			Description: "HelloPayment",
		},
	}
}

// GetLightingConfig returns a LN config that's suitable for testing purposes.
func GetLightingConfig(t *testing.T) ln.LightningConfig {
	port := getPortOrFail(t)
	tempDir, err := ioutil.TempDir("", "teslacoil-lnd-")
	if err != nil {
		testutil.FatalMsgf(t, "Could not create temp lnd dir: %v", err)
	}
	return ln.LightningConfig{
		LndDir:    tempDir,
		Network:   chaincfg.RegressionNetParams,
		RPCServer: fmt.Sprintf("localhost:%d", port),
		P2pPort:   getPortOrFail(t),
	}
}

// Returns a unused port
func getPortOrFail(t *testing.T) int {
	const minPortNumber = 1024
	const maxPortNumber = 40000
	rand.Seed(time.Now().UnixNano())
	port := rand.Intn(maxPortNumber)
	// port is reserved, try again
	if port < minPortNumber {
		return getPortOrFail(t)
	}

	listener, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	// port is busy, try again
	if err != nil {
		return getPortOrFail(t)
	}
	if err := listener.Close(); err != nil {
		testutil.FatalMsgf(t, "Couldn't close port: %sl", err)
	}
	return port

}

// GetBitcoindConfig returns a bitcoind config suitable for testing purposes
func GetBitcoindConfig(t *testing.T) bitcoind.Config {
	return bitcoind.Config{
		P2pPort:        getPortOrFail(t),
		RpcPort:        getPortOrFail(t),
		User:           "rpc_user_for_tests",
		Password:       "rpc_pass_for_tests",
		ZmqPubRawTx:    fmt.Sprintf("tcp://0.0.0.0:%d", getPortOrFail(t)),
		ZmqPubRawBlock: fmt.Sprintf("tcp://0.0.0.0:%d", getPortOrFail(t)),
	}

}

// GetBitcoindClientOrFail returns a bitcoind RPC client, corresponding to
// the given configuration.
func GetBitcoindClientOrFail(t *testing.T, conf bitcoind.Config) *rpcclient.Client {
	// Bitcoin Core doesn't do notifications
	var notificationHandler *rpcclient.NotificationHandlers = nil

	client, err := rpcclient.New(conf.ToConnConfig(), notificationHandler)
	if err != nil {
		testutil.FatalMsg(t, err)
	}

	return client
}

// StartBitcoindOrFail starts a bitcoind node with the given configuration,
// with the data directory set to the users temporary directory. The function
// returns the created client, as well as a function that cleans up the operation
// (stopping the node and deleting the data directory).
func StartBitcoindOrFail(t *testing.T, conf bitcoind.Config) (client *rpcclient.Client, cleanup func() error) {
	tempDir, err := ioutil.TempDir("", "teslacoil-bitcoind-")
	if err != nil {
		testutil.FatalMsgf(t, "Could not create temporary bitcoind dir: %v", err)
	}
	args := []string{
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
		"-zmqpubrawtx=" + conf.ZmqPubRawTx,
		"-zmqpubrawblock=" + conf.ZmqPubRawBlock,
	}

	log.Debugf("Executing command: bitcoind %s", strings.Join(args, " "))
	cmd := exec.Command("bitcoind", args...)

	// pass bitcoind output to test log, wrapepd with a label
	cmd.Stderr = logWriter{"bitcoind", logrus.ErrorLevel}
	cmd.Stdout = logWriter{"bitcoind", logrus.DebugLevel}
	if err := cmd.Run(); err != nil {
		testutil.FatalMsgf(t, "Could not start bitcoind: %v", err)
	}

	pidFile := filepath.Join(tempDir, "regtest", "bitcoind.pid")

	readPidFile := func() error {
		_, err := os.Stat(pidFile)
		return err
	}
	if err := asyncutil.Retry(retryAttempts, retrySleepDuration, readPidFile); err != nil {
		testutil.FatalMsg(t, errors.Wrap(err, "could not read bitcoind pid file"))
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

	client = GetBitcoindClientOrFail(t, conf)

	// await bitcoind startup
	if err := asyncutil.Retry(retryAttempts, retrySleepDuration, client.Ping); err != nil {
		testutil.FatalMsg(t, errors.Wrap(err, "could not communicate with bitcoind"))
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
			return err
		}

		log.Debug("Stopped bitcoind process")
		if err := os.RemoveAll(tempDir); err != nil {
			return errors.Wrapf(err, "could not delete bitcoind tmp directory %s", tempDir)
		}
		log.Debugf("Deleted bitcoind tmp directory %s", tempDir)
		return nil
	}
	return client, cleanup
}

type logWriter struct {
	label string
	level logrus.Level
}

func (p logWriter) Write(data []byte) (n int, err error) {
	log.Logf(p.level, "[%s] %s", p.label, string(data))
	return len(data), nil
}

// StartLndOrFail starts a lnd node with the given configuration,
// The function returns the created client, as well as a function that cleans
// up the operation (stopping the node and deleting the data directory).
func StartLndOrFail(t *testing.T,
	bitcoindConfig bitcoind.Config,
	lndConfig ln.LightningConfig) (client lnrpc.LightningClient, cleanup func() error) {
	if lndConfig.RPCServer == "" {
		testutil.FatalMsg(t, "lndConfig.RPCServer needs to be set, was empty")
	}
	if lndConfig.LndDir == "" {
		testutil.FatalMsg(t, "lndConfig.LndDir needs to be set, was empty")
	}
	if lndConfig.Network.Name != chaincfg.RegressionNetParams.Name {
		testutil.FatalMsg(t, "lndConfig.Network was not regtest! Support for this is not implemented")
	}

	if lndConfig.MacaroonPath == "" {
		lndConfig.MacaroonPath = filepath.Join(
			lndConfig.LndDir, ln.DefaultRelativeMacaroonPath(lndConfig.Network),
		)
	}
	if lndConfig.TLSCertPath == "" {
		lndConfig.TLSCertPath = filepath.Join(lndConfig.LndDir, "tls.cert")
	}
	if lndConfig.TLSKeyPath == "" {
		lndConfig.TLSKeyPath = filepath.Join(lndConfig.LndDir, "tls.key")
	}

	args := []string{
		"--noseedbackup",
		"--bitcoin.active",
		"--bitcoin.regtest",
		"--bitcoin.node=bitcoind",
		"--datadir=" + filepath.Join(lndConfig.LndDir, "data"),
		"--logdir=" + filepath.Join(lndConfig.LndDir, "logs"),
		"--configfile=" + filepath.Join(lndConfig.LndDir, "lnd.conf"),
		"--tlscertpath=" + lndConfig.TLSCertPath,
		"--tlskeypath=" + lndConfig.TLSKeyPath,
		"--adminmacaroonpath=" + lndConfig.MacaroonPath,
		"--rpclisten=" + lndConfig.RPCServer,
		fmt.Sprintf("--listen=%d", lndConfig.P2pPort),
		fmt.Sprintf("--restlisten=%d", getPortOrFail(t)),
		"--bitcoind.rpcuser=" + bitcoindConfig.User,
		"--bitcoind.rpcpass=" + bitcoindConfig.Password,
		fmt.Sprintf("--bitcoind.rpchost=localhost:%d", +bitcoindConfig.RpcPort),
		"--bitcoind.zmqpubrawtx=" + bitcoindConfig.ZmqPubRawTx,
		"--bitcoind.zmqpubrawblock=" + bitcoindConfig.ZmqPubRawBlock,
		"--debuglevel=trace",
	}

	cmd := exec.Command("lnd", args...)

	// pass LND output to test output, logged with a label
	cmd.Stderr = logWriter{"LND", logrus.ErrorLevel}
	cmd.Stdout = logWriter{"LND", logrus.DebugLevel}

	log.Debugf("Executing command: %s", strings.Join(cmd.Args, " "))
	if err := cmd.Start(); err != nil {
		testutil.FatalMsgf(t, "Could not start lnd: %v", err)
	}
	pid := cmd.Process.Pid
	log.Debugf("Started lnd with pid %d", pid)

	// await LND startup
	isReady := func() error {
		certFile := filepath.Join(lndConfig.LndDir, "tls.cert")
		if _, err := os.Stat(certFile); err != nil {
			return err
		}

		if _, err := os.Stat(lndConfig.MacaroonPath); err != nil {
			return err
		}

		return nil
	}

	if err := asyncutil.Retry(retryAttempts, retrySleepDuration, isReady); err != nil {
		testutil.FatalMsg(t, errors.Wrap(err, "lnd cert and macaroon file was not created"))
	}
	log.Debugf("lnd cert file and macaroon file exists")

	var lnd lnrpc.LightningClient
	var err error
	getLnd := func() error {
		lnd, err = ln.NewLNDClient(lndConfig)
		return err
	}
	if err := asyncutil.Retry(retryAttempts, retrySleepDuration, getLnd); err != nil {
		testutil.FatalMsgf(t, "Could not get lnd with config %v after trying %d times: %s ",
			lndConfig, retryAttempts, err)
	}

	cleanup = func() error {

		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
			return errors.Wrap(err, "couldn't kill lnd process")
		}
		negativeGetInfo := func() error {
			_, err := client.GetInfo(context.Background(), &lnrpc.GetInfoRequest{})
			if err == nil {
				return errors.New("was able to getinfo from client")
			}
			return nil
		}

		// await lnd shutdown
		if err := asyncutil.Retry(retryAttempts, retrySleepDuration, negativeGetInfo); err != nil {
			return err
		}
		log.Debug("Stopped lnd process")

		if err := os.RemoveAll(lndConfig.LndDir); err != nil {
			return errors.Wrapf(err, "could not delete lnd tmp directory %s", lndConfig.LndDir)
		}
		log.Debugf("Deleted lnd tmp directory %s", lndConfig.LndDir)

		return nil
	}

	return lnd, cleanup
}

// RunWithLnd lets you test functionality that requires actual LND/bitcoind
// nodes by creating the nodes, running your tests, and then performs the
// necessary cleanup.
func RunWithLnd(t *testing.T, test func(lnd lnrpc.LightningClient)) {
	bitcoindConf := GetBitcoindConfig(t)
	lndConf := GetLightingConfig(t)
	_, cleanupBitcoind := StartBitcoindOrFail(t, bitcoindConf)

	lnd, cleanupLnd := StartLndOrFail(t, bitcoindConf, lndConf)

	cleanup := func() error {
		bitcoindErr := cleanupBitcoind()
		lndErr := cleanupLnd()
		if bitcoindErr != nil && lndErr != nil {
			return fmt.Errorf("failed to cleanup bitcoind: %s and lnd: %s",
				bitcoindErr.Error(), lndErr.Error())
		} else if bitcoindErr != nil {
			return bitcoindErr
		} else if lndErr != nil {
			return lndErr
		}
		return nil
	}

	test(lnd)

	if err := cleanup(); err != nil {
		t.Fatalf("Couldn't clean up after %q: %v", t.Name(), err)
	}
}
