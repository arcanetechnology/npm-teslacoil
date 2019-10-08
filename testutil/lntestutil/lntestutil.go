package lntestutil

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit"
	"github.com/btcsuite/btcd/chaincfg"
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
	port := testutil.GetPortOrFail(t)
	tempDir, err := ioutil.TempDir("", "teslacoil-lnd-")
	if err != nil {
		testutil.FatalMsgf(t, "Could not create temp lnd dir: %v", err)
	}
	return ln.LightningConfig{
		LndDir:    tempDir,
		Network:   chaincfg.RegressionNetParams,
		RPCServer: fmt.Sprintf("localhost:%d", port),
		P2pPort:   testutil.GetPortOrFail(t),
	}
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
	bitcoindConf := bitcoind.GetBitcoindConfig(t)
	lndConf := GetLightingConfig(t)
	_, cleanupBitcoind := bitcoind.StartBitcoindOrFail(t, bitcoindConf)

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

// RunWithBitcoindAndLnd lets you test functionality that requires actual LND/bitcoind
// nodes by creating the nodes, running your tests, and then performs the
// necessary cleanup.
func RunWithBitcoindAndLnd(t *testing.T, test func(lnd lnrpc.LightningClient, bitcoin bitcoind.TeslacoilBitcoind)) {
	bitcoindConf := bitcoind.GetBitcoindConfig(t)
	lndConf := GetLightingConfig(t)
	bitcoin, cleanupBitcoind := bitcoind.StartBitcoindOrFail(t, bitcoindConf)

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

	test(lnd, bitcoin)

	if err := cleanup(); err != nil {
		t.Fatalf("Couldn't clean up after %q: %v", t.Name(), err)
	}
}
