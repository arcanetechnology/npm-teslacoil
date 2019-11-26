// Package nodetestutil provides functionality for running tests with
// actual Bitcoin and Lightning nodes

package nodetestutil

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"gitlab.com/arcanecrypto/teslacoil/build"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/arcanecrypto/teslacoil/async"
	"gitlab.com/arcanecrypto/teslacoil/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/ln"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/bitcoindtestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/lntestutil"
)

const (
	retryAttempts      = 7
	retrySleepDuration = time.Millisecond * 100
)

var (
	log         = build.AddSubLogger("NODETESTUTIL")
	_   cleaner = nodeCleaner{}
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
type cleaner interface {
	// HasBeenCleaned returns whether or not the cleanup action has been performed
	HasBeenCleaned() bool
	// Clean performs the cleanup action
	Clean() error
}

var nodeCleaners []cleaner

// registerCleaner appends the given cleanup action to our local list of
// actions that should be performed.
func registerCleaner(cleaner cleaner) {
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

func fundLndOrFail(t *testing.T, lnd lnrpc.LightningClient, bitcoin bitcoind.TeslacoilBitcoind) {
	if t.Failed() {
		return
	}

	address, err := lnd.NewAddress(context.Background(), &lnrpc.NewAddressRequest{
		Type: lnrpc.AddressType_WITNESS_PUBKEY_HASH,
	})
	require.NoError(t, err)

	btcAddr, err := btcutil.DecodeAddress(address.Address, &chaincfg.RegressionNetParams)
	require.NoError(t, err)

	// we want to fund LND with a few outputs, so that sending one TX doesn't lock up
	// all the other money until it's confirmed
	var wg sync.WaitGroup
	const utxos = 20
	const sats = 100 * btcutil.SatoshiPerBitcoin // 100 BTC
	wg.Add(utxos)
	for i := 0; i < utxos; i++ {
		go func() {
			_, err = bitcoin.Btcctl().SendToAddress(btcAddr, btcutil.Amount(sats/utxos))
			require.NoError(t, err)
			wg.Done()
		}()
	}
	const timeout = time.Second * 3
	if async.WaitTimeout(&wg, timeout) {
		t.Fatalf("Funding LND timed out after %s", timeout)
	}

	// confirm the TX
	_, err = bitcoind.GenerateToAddress(bitcoin, 6, btcAddr)
	require.NoError(t, err)

	// wait for lnd to catch up
	err = async.RetryNoBackoff(10, time.Millisecond*500, func() error {
		balance, err := lnd.WalletBalance(context.Background(), &lnrpc.WalletBalanceRequest{})
		if err != nil {
			return err
		}
		if balance.ConfirmedBalance < sats {
			return fmt.Errorf("confirmed balance (%d) was below target (%f)", balance.ConfirmedBalance, sats)
		}
		return nil
	})
	require.NoError(t, err)

}

// GetLndPairAndBitcoind starts two LND nodes and a bitcoind nodes, funds them, and registers cleanup actions that
// can be performed at test teardown.
func GetLndPairAndBitcoind(t *testing.T) (lnrpc.LightningClient, lnrpc.LightningClient, bitcoind.TeslacoilBitcoind) {
	prevLen := len(nodeCleaners)

	bitcoindConf := bitcoindtestutil.GetBitcoindConfig(t)
	bitcoin := StartBitcoindOrFail(t, bitcoindConf)

	// we do this in a go routine, as generatetoaddress is completely sync and takes a while
	// make sure bitcoind is funded, so we can give some money to LND later
	go func() {
		addr, err := bitcoin.Btcctl().GetNewAddress("")
		require.NoError(t, err)
		_, err = bitcoind.GenerateToAddress(bitcoin, 110, addr)
		require.NoError(t, err, "could not generate to address")
	}()

	var wg sync.WaitGroup
	// wait for two lnd nodes
	wg.Add(4) // two nodes + two funds
	// start two lnd nodes simultaneously to save time
	var lnd1, lnd2 lnrpc.LightningClient
	lndConf := lntestutil.GetLightingConfig(t)
	go func() {
		defer wg.Done()
		lnd1 = startLndOrFailAsync(t, bitcoindConf, lndConf, &wg)
		if t.Failed() {
			return
		}
		fundLndOrFail(t, lnd1, bitcoin)
		log.WithField("lndDir", lndConf.LndDir).Info("Funded LND")
	}()

	lndConf2 := lntestutil.GetLightingConfig(t)
	go func() {
		defer wg.Done()
		lnd2 = startLndOrFailAsync(t, bitcoindConf, lndConf2, &wg)
		if t.Failed() {
			return
		}
		fundLndOrFail(t, lnd2, bitcoin)
		log.WithField("lndDir", lndConf2.LndDir).Info("Funded LND")
	}()

	timeout := time.Second * 20
	if async.WaitTimeout(&wg, timeout) {
		assert.FailNow(t, "LND nodes did not start", "timeout: %s", timeout)
	}

	afterLen := len(nodeCleaners)
	require.Greater(t, afterLen-prevLen, 2, "Node cleaners weren't registered correctly!: %d", afterLen-prevLen)

	// Create new address for node 1 and fund it with a lot of money
	addr, err := lnd1.NewAddress(context.Background(), &lnrpc.NewAddressRequest{
		Type: lnrpc.AddressType_WITNESS_PUBKEY_HASH,
	})
	require.NoError(t, err, "could not get new address from lnd")

	lnd1Address := bitcoindtestutil.ConvertToAddressOrFail(addr.Address, bitcoindConf.Network)

	// get info to open channels with the node
	lnd2Info, err := lnd2.GetInfo(context.Background(), &lnrpc.GetInfoRequest{})
	require.NoError(t, err, "could not get node info from lnd2: %v", err)

	connect := func() error {
		lnAddress := lnrpc.LightningAddress{
			Pubkey: lnd2Info.IdentityPubkey,
			Host:   fmt.Sprintf("127.0.0.1:%d", lndConf2.P2pPort),
		}
		_, err = lnd1.ConnectPeer(context.Background(), &lnrpc.ConnectPeerRequest{
			Addr: &lnAddress,
		})
		return err

	}

	err = async.RetryNoBackoff(10, 300*time.Millisecond, connect)
	require.NoError(t, err, "could not connect nodes %v", err)

	log.WithFields(logrus.Fields{
		"lnd1": lndConf.LndDir,
		"lnd2": lndConf2.LndDir,
	}).Info("Connected LND nodes")

	isSyncedToChain := func() error {
		info, err := lnd1.GetInfo(context.Background(), &lnrpc.GetInfoRequest{})
		if err != nil {
			return err
		}
		if !info.SyncedToChain {
			return errors.New("not synced to chain")
		}
		return nil

	}
	err = async.RetryNoBackoff(10, time.Millisecond*200, isSyncedToChain)
	require.NoError(t, err)

	log.WithField("lndDir", lndConf.LndDir).Info("Synced to chain")

	openchannel := func() error {
		_, err = lnd1.OpenChannelSync(context.Background(), &lnrpc.OpenChannelRequest{
			NodePubkeyString:   lnd2Info.IdentityPubkey,
			LocalFundingAmount: ln.MaxAmountSatPerChannel,
			PushSat:            ln.MaxAmountSatPerChannel / 2,
			SpendUnconfirmed:   true,
		})
		return err
	}

	err = async.RetryNoBackoff(30, 100*time.Millisecond, openchannel)
	require.NoError(t, err)

	// we generate to address to be able to confirm the channel we created
	_, err = bitcoind.GenerateToAddress(bitcoin, 6, lnd1Address)
	require.NoError(t, err, "could not confirm channel")

	return lnd1, lnd2, bitcoin
}

// GetLndAndBitcoind starts a LND and bitcoind node and registers cleanup actions that can be performed at test teardown
func GetLndAndBitcoind(t *testing.T) (lnrpc.LightningClient, bitcoind.TeslacoilBitcoind) {
	prevLen := len(nodeCleaners)

	bitcoindConf := bitcoindtestutil.GetBitcoindConfig(t)
	bitcoin := StartBitcoindOrFail(t, bitcoindConf)

	lndConf := lntestutil.GetLightingConfig(t)
	lnd := StartLndOrFail(t, bitcoindConf, lndConf)

	afterLen := len(nodeCleaners)
	require.GreaterOrEqual(t, afterLen-prevLen, 2, "after: %d, prev: %d", afterLen, prevLen)

	addr, err := lnd.NewAddress(context.Background(), &lnrpc.NewAddressRequest{
		Type: 0,
	})
	require.NoError(t, err, "could not get new address from lnd")

	address := bitcoindtestutil.ConvertToAddressOrFail(addr.Address, bitcoindConf.Network)

	_, err = bitcoindtestutil.GenerateToSelf(10, bitcoin)
	require.NoError(t, err)

	_, err = bitcoind.GenerateToAddress(bitcoin, 101, address)
	require.NoError(t, err, "could not generate to address")

	return lnd, bitcoin
}

// GetLnd lets you test functionality that requires actual LND/bitcoind
// nodes by creating the nodes, running your tests, and then performs the
// necessary cleanup.
func GetLnd(t *testing.T) lnrpc.LightningClient {
	lnd, _ := GetLndAndBitcoind(t)
	return lnd
}

// GetBitcoind lets you test functionality that requires actual bitcoind
// node by creating starting up bitcoind, running the test and then running
// the necessary cleanup.
func GetBitcoind(t *testing.T) bitcoind.TeslacoilBitcoind {
	bitcoindConf := bitcoindtestutil.GetBitcoindConfig(t)
	bitcoin := StartBitcoindOrFail(t, bitcoindConf)

	// fund the node
	_, err := bitcoindtestutil.GenerateToSelf(101, bitcoin)
	require.NoError(t, err)

	return bitcoin
}

// LndWithPid is a regular LND client that's also got a PID
type LndWithPid interface {
	lnrpc.LightningClient
	Pid() int
}

type lndWithPid struct {
	lnrpc.LightningClient
	pid int
}

// Pid returns the process identifier (PID) for the running LND process
func (l lndWithPid) Pid() int {
	return l.pid
}

var _ LndWithPid = lndWithPid{}

// StartLndOrFail returns the created client, and register a cleanup action
// that can be performed during test teardown.
func StartLndOrFail(t *testing.T, bitcoindConfig bitcoind.Config,
	lndConfig ln.LightningConfig) LndWithPid {

	wg := sync.WaitGroup{}
	wg.Add(1)

	return startLndOrFailAsync(t, bitcoindConfig, lndConfig, &wg)
}

// startLndOrFailAsync returns the created client, and register a cleanup action
// that can be performed during test teardown.
func startLndOrFailAsync(t *testing.T, bitcoindConfig bitcoind.Config,
	lndConfig ln.LightningConfig, wg *sync.WaitGroup) LndWithPid {
	version, err := exec.Command("lnd", "--version").Output()
	require.NoError(t, err)
	require.Contains(t, string(version[:len(version)-1]), "lnd version 0.8", "You need to have the latest version of LND installed!")

	if !assert.NotEmpty(t, lndConfig.RPCHost) {
		return nil
	}
	if !assert.NotEmpty(t, lndConfig.LndDir) {
		return nil
	}
	if !assert.Equal(t, lndConfig.Network.Name, chaincfg.RegressionNetParams.Name) {
		return nil
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
		fmt.Sprintf("--rpclisten=%s:%d", lndConfig.RPCHost, lndConfig.RPCPort),
		fmt.Sprintf("--listen=%d", lndConfig.P2pPort),
		fmt.Sprintf("--restlisten=%d", testutil.GetPortOrFail(t)),
		"--bitcoind.rpcuser=" + bitcoindConfig.User,
		"--bitcoind.rpcpass=" + bitcoindConfig.Password,
		fmt.Sprintf("--bitcoind.rpchost=localhost:%d", +bitcoindConfig.RpcPort),
		"--bitcoind.zmqpubrawtx=" + bitcoindConfig.ZmqPubRawTx,
		"--bitcoind.zmqpubrawblock=" + bitcoindConfig.ZmqPubRawBlock,
		"--debuglevel=debug",
	}

	cmd := exec.Command("lnd", args...)

	// pass LND output to test output, logged with a label
	_, file := path.Split(lndConfig.LndDir)
	parts := strings.SplitN(file, "-", 2)
	label := parts[1]
	cmd.Stderr = testutil.LogWriter{Label: label, Level: logrus.ErrorLevel}
	cmd.Stdout = testutil.LogWriter{Label: label, Level: logrus.DebugLevel}

	log.Debugf("Executing command: %s", strings.Join(cmd.Args, " "))
	if !assert.NoError(t, cmd.Start(), "could not start lnd") {
		return nil
	}
	pid := cmd.Process.Pid
	log.Debugf("Started lnd with pid %d", pid)

	// await LND startup
	certFile := filepath.Join(lndConfig.LndDir, "tls.cert")
	// by looking at logs it appears we connect immediately after this file is created
	backupFile := filepath.Join(lndConfig.LndDir, "data", "chain", "bitcoin", lndConfig.Network.Name, "channel.backup")
	isReady := func() error {
		if _, err = os.Stat(certFile); err != nil {
			return err
		}
		if _, err = os.Stat(lndConfig.MacaroonPath); err != nil {
			return err
		}
		if _, err = os.Stat(backupFile); err != nil {
			return err
		}

		return nil
	}

	attempts := 30
	timeout := time.Millisecond * 500
	if os.Getenv("CI") != "" {
		timeout = time.Millisecond * 750
		attempts = 50
	}
	err = async.RetryNoBackoff(attempts, timeout, isReady)
	if !assert.NoError(t, err) {
		return nil
	}

	var lnd lnrpc.LightningClient
	getLnd := func() error {
		lnd, err = ln.NewLNDClient(lndConfig)
		return err
	}
	err = async.RetryNoBackoff(retryAttempts, retrySleepDuration, getLnd)
	if !assert.NoError(t, err, lndConfig) {
		return nil
	}

	cleanup := nodeCleaner{}
	cleanup.clean = func() error {
		cleanup.hasBeenCleaned = true
		if err = syscall.Kill(pid, syscall.SIGTERM); err != nil {
			return errors.Wrap(err, "couldn't kill lnd process")
		}
		negativeGetInfo := func() error {
			_, err = lnd.GetInfo(context.Background(), &lnrpc.GetInfoRequest{})
			if err == nil {
				return errors.New("was able to getinfo from client")
			}
			return nil
		}

		// await lnd shutdown
		if err = async.RetryBackoff(retryAttempts, retrySleepDuration, negativeGetInfo); err != nil {
			return err
		}
		log.Debug("Stopped lnd process")

		if err = os.RemoveAll(lndConfig.LndDir); err != nil {
			return errors.Wrapf(err, "could not delete lnd tmp directory %s", lndConfig.LndDir)
		}
		log.Debugf("Deleted lnd tmp directory %s", lndConfig.LndDir)

		return nil
	}
	// pointer so we can mutate the object
	registerCleaner(&cleanup)

	wg.Done()

	return lndWithPid{
		LightningClient: lnd,
		pid:             pid,
	}
}

// StartBitcoindOrFail starts a bitcoind node with the given configuration,
// with the data directory set to the users temporary directory. The function
// register a cleanup action that can be performed during test teardown.
func StartBitcoindOrFail(t *testing.T, conf bitcoind.Config) *bitcoind.Conn {
	tempDir, err := ioutil.TempDir("", "teslacoil-bitcoind-")
	require.NoError(t, err)

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
	if !assert.NoError(t, cmd.Run()) {
		return nil
	}

	pidFile := filepath.Join(tempDir, "regtest", "bitcoind.pid")

	readPidFile := func() error {
		_, err = os.Stat(pidFile)
		return err
	}
	err = async.RetryNoBackoff(retryAttempts, retrySleepDuration, readPidFile)
	if !assert.NoError(t, err) {
		return nil
	}

	pidBytes, err := ioutil.ReadFile(pidFile)
	if !assert.NoError(t, err) {
		return nil
	}

	pidLines := strings.Split(string(pidBytes), "\n")
	pid, err := strconv.Atoi(pidLines[0])
	if !assert.NoError(t, err) {
		return nil
	}

	log.Debugf("Started bitcoind client with pid %d", pid)

	client := bitcoindtestutil.GetBitcoindClientOrFail(t, conf)

	// await bitcoind startup
	err = async.RetryBackoff(retryAttempts, retrySleepDuration, client.Ping)
	if !assert.NoError(t, err, "Could not communicate with bitcoind") {
		return nil
	}

	cleaner := nodeCleaner{}
	cleaner.clean = func() error {
		cleaner.hasBeenCleaned = true
		if err = syscall.Kill(pid, syscall.SIGTERM); err != nil {
			return errors.Wrap(err, "couldn't kill bitcoind process")
		}

		negativePing := func() error {
			err = client.Ping()
			if err == nil {
				return errors.New("was able to ping client")
			}
			return nil
		}

		// await bitcoind shutdown
		if err = async.RetryBackoff(retryAttempts, retrySleepDuration, negativePing); err != nil {
			return fmt.Errorf("could communicate with stopped bitcoind")
		}

		log.Debug("Stopped bitcoind process")
		deleteBitcoin := func() error {
			if err = os.RemoveAll(tempDir); err != nil {
				return errors.Wrapf(err, "could not delete bitcoind tmp directory %s", tempDir)
			}
			log.Debugf("Deleted bitcoind tmp directory %s", tempDir)
			return nil
		}
		return async.RetryNoBackoff(10, time.Second, deleteBitcoin)
	}
	// pointer so we can mutate the object
	registerCleaner(&cleaner)

	// TODO interval here
	conn, err := bitcoind.NewConn(conf, time.Millisecond*7)
	assert.NoError(t, err)
	return conn
}
