// Package nodetestutil provides functionality for running tests with
// actual Bitcoin and Lightning nodes

package nodetestutil

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/asyncutil"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/ln"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/bitcoindtestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/lntestutil"
)

const (
	retryAttempts      = 7
	retrySleepDuration = time.Millisecond * 100
)

var (
	log         = build.Log
	_   Cleaner = nodeCleaner{}
)

type nodeCleaner struct {
	hasBeenCleaned bool
	clean          func() error
}

func (b nodeCleaner) HasBeenCleaned() bool {
	return b.hasBeenCleaned
}

func (b nodeCleaner) Clean() error {
	err := b.clean()
	b.hasBeenCleaned = true
	return err
}

// Cleaner can clean up after a node has been spun up. It keeps track of
// whether or not the cleanup action has been performed.
type Cleaner interface {
	// HasBeenCleaned returns whether or not the cleanup action has been performed
	HasBeenCleaned() bool
	// Clean performs the cleanup action
	Clean() error
}

var nodeCleaners []Cleaner

// RegisterCleaner appends the given cleanup action to our local list of
// actions that should be performed.
func RegisterCleaner(cleaner Cleaner) {
	nodeCleaners = append(nodeCleaners, cleaner)
}

// CleanupNodes performs all the pending cleanup actions we have registered
func CleanupNodes() error {
	for _, cleaner := range nodeCleaners {
		if cleaner.HasBeenCleaned() {
			continue
		}
		if err := cleaner.Clean(); err != nil {
			return err
		}
	}
	return nil
}

func RunWithBitcoindAndLndPair(t *testing.T, test func(lnd1 lnrpc.LightningClient, lnd2 lnrpc.LightningClient, bitcoin bitcoind.TeslacoilBitcoind)) {
	prevLen := len(nodeCleaners)

	bitcoindConf := bitcoindtestutil.GetBitcoindConfig(t)
	bitcoin := StartBitcoindOrFail(t, bitcoindConf)

	var wg sync.WaitGroup
	// wait for two lnd nodes
	wg.Add(2)
	// start two lnd nodes simultaneously to save time
	var lnd1, lnd2 lnrpc.LightningClient
	lndConf := lntestutil.GetLightingConfig(t)
	go func() {
		lnd1 = StartLndOrFailAsync(t, bitcoindConf, lndConf, &wg)
	}()

	lndConf2 := lntestutil.GetLightingConfig(t)
	go func() {
		lnd2 = StartLndOrFailAsync(t, bitcoindConf, lndConf2, &wg)
	}()

	wg.Wait()
	afterLen := len(nodeCleaners)
	if afterLen-prevLen < 2 {
		testutil.FatalMsgf(t, "Node cleaners weren't registered correctly!: %d", afterLen-prevLen)
	}

	// Create new address for node 1 and fund it with a lot of money
	addr, err := lnd1.NewAddress(context.Background(), &lnrpc.NewAddressRequest{
		Type: 0,
	})
	if err != nil {
		testutil.FatalMsg(t, "could not get new address from lnd")
	}
	address := bitcoindtestutil.ConvertToAddressOrFail(addr.Address, bitcoindConf.Network)
	_, err = bitcoind.GenerateToAddress(bitcoin, 101, address)
	if err != nil {
		testutil.FatalMsg(t, "could not generate to address")
	}

	// get info to open channels with the node
	lnd2Info, err := lnd2.GetInfo(context.Background(), &lnrpc.GetInfoRequest{})
	if err != nil {
		testutil.FatalMsgf(t, "could not get node info from lnd2: %v", err)
	}

	retry1 := func() bool {

		lnAddress := lnrpc.LightningAddress{
			Pubkey: lnd2Info.IdentityPubkey,
			Host:   fmt.Sprintf("127.0.0.1:%d", lndConf2.P2pPort),
		}
		_, err := lnd1.ConnectPeer(context.Background(), &lnrpc.ConnectPeerRequest{
			Addr: &lnAddress,
		})

		return err == nil
	}

	err = asyncutil.Await(15, time.Second, retry1)
	if err != nil {
		testutil.FatalMsgf(t, "could not connect nodes %v", err)
	}

	_, err = lnd1.ListPeers(context.Background(), &lnrpc.ListPeersRequest{})
	if err != nil {
		testutil.FatalMsgf(t, "could not list peers: %+v", err)
	}

	retry2 := func() bool {

		_, err = lnd1.OpenChannelSync(context.Background(), &lnrpc.OpenChannelRequest{
			NodePubkeyString:   lnd2Info.IdentityPubkey,
			LocalFundingAmount: 10000000, // 10 000 000
			PushSat:            5000000,  // 5 000 000
			SpendUnconfirmed:   true,
		})
		if err != nil {
			t.Logf("could not open channel: %v", err)
			return false
		}

		return true
	}

	err = asyncutil.Await(15, time.Second, retry2)
	if err != nil {
		testutil.FatalMsgf(t, "could not open channel %v", err)
	}

	// retry paying an invoice from lnd1 to lnd2
	invoice, err := lnd2.AddInvoice(context.Background(), &lnrpc.Invoice{
		Value: 5000,
	})
	if err != nil {
		testutil.FailMsgf(t, "could not add invoice: %v", err)
	}

	// we retry confirming the channel and sending a payment
	retry := func() bool {

		// we generate to address to be able to spend the funds
		_, err = bitcoind.GenerateToAddress(bitcoin, 10, address)
		if err != nil {
			return false
		}
		// attempt to pay the invoice
		payment, err := lnd1.SendPaymentSync(context.Background(), &lnrpc.SendRequest{
			PaymentRequest: invoice.PaymentRequest,
		})
		if err != nil || payment.PaymentError != "" {
			return false
		}

		return true
	}

	err = asyncutil.Await(20, 100*time.Millisecond, retry)
	if err != nil {
		testutil.FatalMsgf(t, "could not send payment %v", err)
	}

	test(lnd1, lnd2, bitcoin)
}

// RunWithBitcoindAndLnd lets you test functionality that requires actual LND/bitcoind
// nodes by creating the nodes, running your tests, and then performs the
// necessary cleanup.
func RunWithBitcoindAndLnd(t *testing.T, giveInitialBalance bool, test func(lnd lnrpc.LightningClient, bitcoin bitcoind.TeslacoilBitcoind)) {
	prevLen := len(nodeCleaners)

	bitcoindConf := bitcoindtestutil.GetBitcoindConfig(t)
	bitcoin := StartBitcoindOrFail(t, bitcoindConf)

	lndConf := lntestutil.GetLightingConfig(t)
	lnd := StartLndOrFail(t, bitcoindConf, lndConf)

	afterLen := len(nodeCleaners)
	if afterLen-prevLen < 2 {
		testutil.FatalMsg(t, "Node cleaners weren't registered correctly!")
	}

	if giveInitialBalance {

		addr, err := lnd.NewAddress(context.Background(), &lnrpc.NewAddressRequest{
			Type: 0,
		})
		if err != nil {
			testutil.FatalMsg(t, "could not get new address from lnd")
		}

		address := bitcoindtestutil.ConvertToAddressOrFail(addr.Address, bitcoindConf.Network)

		_, err = bitcoind.GenerateToAddress(bitcoin, 101, address)
		if err != nil {
			testutil.FatalMsg(t, "could not generate to address")
		}
	}

	test(lnd, bitcoin)

}

// RunWithLnd lets you test functionality that requires actual LND/bitcoind
// nodes by creating the nodes, running your tests, and then performs the
// necessary cleanup.
func RunWithLnd(t *testing.T, giveInitialBalance bool, test func(lnd lnrpc.LightningClient)) {
	RunWithBitcoindAndLnd(t, giveInitialBalance, func(lnd lnrpc.LightningClient, _ bitcoind.TeslacoilBitcoind) {
		test(lnd)
	})
}

// RunWithBitcoind lets you test functionality that requires actual bitcoind
// node by creating starting up bitcoind, running the test and then running
// the necessary cleanup.
func RunWithBitcoind(t *testing.T, giveInitialBalance bool, test func(bitcoin bitcoind.TeslacoilBitcoind)) {
	RunWithBitcoindAndLnd(t, giveInitialBalance, func(_ lnrpc.LightningClient, bitcoin bitcoind.TeslacoilBitcoind) {
		test(bitcoin)
	})
}

// The function returns the created client, and register a cleanup action
// that can be performed during test teardown.
func StartLndOrFail(t *testing.T, bitcoindConfig bitcoind.Config,
	lndConfig ln.LightningConfig) lnrpc.LightningClient {
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
		fmt.Sprintf("--restlisten=%d", testutil.GetPortOrFail(t)),
		"--bitcoind.rpcuser=" + bitcoindConfig.User,
		"--bitcoind.rpcpass=" + bitcoindConfig.Password,
		fmt.Sprintf("--bitcoind.rpchost=localhost:%d", +bitcoindConfig.RpcPort),
		"--bitcoind.zmqpubrawtx=" + bitcoindConfig.ZmqPubRawTx,
		"--bitcoind.zmqpubrawblock=" + bitcoindConfig.ZmqPubRawBlock,
		"--debuglevel=trace",
	}

	cmd := exec.Command("lnd", args...)

	// pass LND output to test output, logged with a label
	cmd.Stderr = testutil.LogWriter{Label: "LND", Level: logrus.ErrorLevel}
	cmd.Stdout = testutil.LogWriter{Label: "LND", Level: logrus.DebugLevel}

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

	cleanup := nodeCleaner{}
	cleanup.clean = func() error {
		cleanup.hasBeenCleaned = true
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
			return errors.Wrap(err, "couldn't kill lnd process")
		}
		negativeGetInfo := func() error {
			_, err := lnd.GetInfo(context.Background(), &lnrpc.GetInfoRequest{})
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
	// pointer so we can mutate the object
	RegisterCleaner(&cleanup)

	return lnd
}

// The function returns the created client, and register a cleanup action
// that can be performed during test teardown.
func StartLndOrFailAsync(t *testing.T, bitcoindConfig bitcoind.Config,
	lndConfig ln.LightningConfig, wg *sync.WaitGroup) lnrpc.LightningClient {
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
		fmt.Sprintf("--restlisten=%d", testutil.GetPortOrFail(t)),
		"--bitcoind.rpcuser=" + bitcoindConfig.User,
		"--bitcoind.rpcpass=" + bitcoindConfig.Password,
		fmt.Sprintf("--bitcoind.rpchost=localhost:%d", +bitcoindConfig.RpcPort),
		"--bitcoind.zmqpubrawtx=" + bitcoindConfig.ZmqPubRawTx,
		"--bitcoind.zmqpubrawblock=" + bitcoindConfig.ZmqPubRawBlock,
		"--debuglevel=trace",
	}

	log.Debugf("executing command: %v", args)

	cmd := exec.Command("lnd", args...)

	// pass LND output to test output, logged with a label
	cmd.Stderr = testutil.LogWriter{Label: "LND", Level: logrus.ErrorLevel}
	cmd.Stdout = testutil.LogWriter{Label: "LND", Level: logrus.DebugLevel}

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

	cleanup := nodeCleaner{}
	cleanup.clean = func() error {
		cleanup.hasBeenCleaned = true
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
			return errors.Wrap(err, "couldn't kill lnd process")
		}
		negativeGetInfo := func() error {
			_, err := lnd.GetInfo(context.Background(), &lnrpc.GetInfoRequest{})
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
	// pointer so we can mutate the object
	RegisterCleaner(&cleanup)

	wg.Done()

	return lnd
}

// StartBitcoindOrFail starts a bitcoind node with the given configuration,
// with the data directory set to the users temporary directory. The function
// register a cleanup action that can be performed during test teardown.
func StartBitcoindOrFail(t *testing.T, conf bitcoind.Config) *bitcoind.Conn {
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
		"-deprecatedrpc=generate",
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
		testutil.FatalMsgf(t, "Could not read bitcoind pid file")
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

	client := bitcoindtestutil.GetBitcoindClientOrFail(t, conf)

	// await bitcoind startup
	if err := asyncutil.Retry(retryAttempts, retrySleepDuration, client.Ping); err != nil {
		testutil.FatalMsgf(t, "Could not communicate with bitcoind")
	}

	cleaner := nodeCleaner{}
	cleaner.clean = func() error {
		cleaner.hasBeenCleaned = true
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
			return fmt.Errorf("could communicate with stopped bitcoind")
		}

		log.Debug("Stopped bitcoind process")
		if err := os.RemoveAll(tempDir); err != nil {
			return errors.Wrapf(err, "could not delete bitcoind tmp directory %s", tempDir)
		}
		log.Debugf("Deleted bitcoind tmp directory %s", tempDir)
		return nil
	}
	// pointer so we can mutate the object
	RegisterCleaner(&cleaner)

	// TODO interval here
	conn, err := bitcoind.NewConn(conf, time.Millisecond*7)
	if err != nil {
		testutil.FatalMsg(t, err)
	}

	return conn
}
