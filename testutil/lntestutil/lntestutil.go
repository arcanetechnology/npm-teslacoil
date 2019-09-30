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

	"github.com/btcsuite/btcd/rpcclient"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/ln"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"google.golang.org/grpc"
)

var (
	log = build.Log
)

const (
	retryAttempts      = 7
	retrySleepDuration = time.Millisecond * 100
)

// LightningMockClient is a mocked out version of LND for testing purposes.
type LightningMockClient struct {
	InvoiceResponse         lnrpc.Invoice
	SendPaymentSyncResponse lnrpc.SendResponse
	DecodePayReqResponse    lnrpc.PayReq
	SendCoinsResponse       lnrpc.SendCoinsResponse
}

func (client LightningMockClient) WalletBalance(ctx context.Context, in *lnrpc.WalletBalanceRequest, opts ...grpc.CallOption) (*lnrpc.WalletBalanceResponse, error) {
	panic("WalletBalance")
}

func (client LightningMockClient) ChannelBalance(ctx context.Context, in *lnrpc.ChannelBalanceRequest, opts ...grpc.CallOption) (*lnrpc.ChannelBalanceResponse, error) {
	panic("ChannelBalance")
}

func (client LightningMockClient) GetTransactions(ctx context.Context, in *lnrpc.GetTransactionsRequest, opts ...grpc.CallOption) (*lnrpc.TransactionDetails, error) {
	panic("GetTransactions")
}

func (client LightningMockClient) EstimateFee(ctx context.Context, in *lnrpc.EstimateFeeRequest, opts ...grpc.CallOption) (*lnrpc.EstimateFeeResponse, error) {
	panic("EstimateFee")
}

func (client LightningMockClient) ListUnspent(ctx context.Context, in *lnrpc.ListUnspentRequest, opts ...grpc.CallOption) (*lnrpc.ListUnspentResponse, error) {
	panic("ListUnspent")
}

func (client LightningMockClient) SubscribeTransactions(ctx context.Context, in *lnrpc.GetTransactionsRequest, opts ...grpc.CallOption) (lnrpc.Lightning_SubscribeTransactionsClient, error) {
	panic("SubscribeTransactions")
}

func (client LightningMockClient) SendMany(ctx context.Context, in *lnrpc.SendManyRequest, opts ...grpc.CallOption) (*lnrpc.SendManyResponse, error) {
	panic("SendMany")
}

func (client LightningMockClient) SignMessage(ctx context.Context, in *lnrpc.SignMessageRequest, opts ...grpc.CallOption) (*lnrpc.SignMessageResponse, error) {
	panic("SignMessage")
}

func (client LightningMockClient) VerifyMessage(ctx context.Context, in *lnrpc.VerifyMessageRequest, opts ...grpc.CallOption) (*lnrpc.VerifyMessageResponse, error) {
	panic("VerifyMessage")
}

func (client LightningMockClient) ConnectPeer(ctx context.Context, in *lnrpc.ConnectPeerRequest, opts ...grpc.CallOption) (*lnrpc.ConnectPeerResponse, error) {
	panic("ConnectPeer")
}

func (client LightningMockClient) DisconnectPeer(ctx context.Context, in *lnrpc.DisconnectPeerRequest, opts ...grpc.CallOption) (*lnrpc.DisconnectPeerResponse, error) {
	panic("DisconnectPeer")
}

func (client LightningMockClient) ListPeers(ctx context.Context, in *lnrpc.ListPeersRequest, opts ...grpc.CallOption) (*lnrpc.ListPeersResponse, error) {
	panic("ListPeers")
}

func (client LightningMockClient) GetInfo(ctx context.Context, in *lnrpc.GetInfoRequest, opts ...grpc.CallOption) (*lnrpc.GetInfoResponse, error) {
	panic("GetInfo")
}

func (client LightningMockClient) PendingChannels(ctx context.Context, in *lnrpc.PendingChannelsRequest, opts ...grpc.CallOption) (*lnrpc.PendingChannelsResponse, error) {
	panic("PendingChannels")
}

func (client LightningMockClient) ListChannels(ctx context.Context, in *lnrpc.ListChannelsRequest, opts ...grpc.CallOption) (*lnrpc.ListChannelsResponse, error) {
	panic("ListChannels")
}

func (client LightningMockClient) SubscribeChannelEvents(ctx context.Context, in *lnrpc.ChannelEventSubscription, opts ...grpc.CallOption) (lnrpc.Lightning_SubscribeChannelEventsClient, error) {
	panic("SubscribeChannelEvents")
}

func (client LightningMockClient) ClosedChannels(ctx context.Context, in *lnrpc.ClosedChannelsRequest, opts ...grpc.CallOption) (*lnrpc.ClosedChannelsResponse, error) {
	panic("ClosedChannels")
}

func (client LightningMockClient) OpenChannelSync(ctx context.Context, in *lnrpc.OpenChannelRequest, opts ...grpc.CallOption) (*lnrpc.ChannelPoint, error) {
	panic("OpenChannelSync")
}

func (client LightningMockClient) OpenChannel(ctx context.Context, in *lnrpc.OpenChannelRequest, opts ...grpc.CallOption) (lnrpc.Lightning_OpenChannelClient, error) {
	panic("OpenChannel")
}

func (client LightningMockClient) CloseChannel(ctx context.Context, in *lnrpc.CloseChannelRequest, opts ...grpc.CallOption) (lnrpc.Lightning_CloseChannelClient, error) {
	panic("CloseChannel")
}

func (client LightningMockClient) AbandonChannel(ctx context.Context, in *lnrpc.AbandonChannelRequest, opts ...grpc.CallOption) (*lnrpc.AbandonChannelResponse, error) {
	panic("AbandonChannel")
}

func (client LightningMockClient) SendPayment(ctx context.Context, opts ...grpc.CallOption) (lnrpc.Lightning_SendPaymentClient, error) {
	panic("SendPayment")
}

func (client LightningMockClient) SendToRoute(ctx context.Context, opts ...grpc.CallOption) (lnrpc.Lightning_SendToRouteClient, error) {
	panic("SendToRoute")
}

func (client LightningMockClient) SendToRouteSync(ctx context.Context, in *lnrpc.SendToRouteRequest, opts ...grpc.CallOption) (*lnrpc.SendResponse, error) {
	panic("SendToRouteSync")
}

func (client LightningMockClient) ListInvoices(ctx context.Context, in *lnrpc.ListInvoiceRequest, opts ...grpc.CallOption) (*lnrpc.ListInvoiceResponse, error) {
	panic("ListInvoices")
}

func (client LightningMockClient) SubscribeInvoices(ctx context.Context, in *lnrpc.InvoiceSubscription, opts ...grpc.CallOption) (lnrpc.Lightning_SubscribeInvoicesClient, error) {
	return &MockSubscribeInvoicesClient{}, nil
}

func (client LightningMockClient) ListPayments(ctx context.Context, in *lnrpc.ListPaymentsRequest, opts ...grpc.CallOption) (*lnrpc.ListPaymentsResponse, error) {
	panic("ListPayments")
}

func (client LightningMockClient) DeleteAllPayments(ctx context.Context, in *lnrpc.DeleteAllPaymentsRequest, opts ...grpc.CallOption) (*lnrpc.DeleteAllPaymentsResponse, error) {
	panic("DeleteAllPayments")
}

func (client LightningMockClient) DescribeGraph(ctx context.Context, in *lnrpc.ChannelGraphRequest, opts ...grpc.CallOption) (*lnrpc.ChannelGraph, error) {
	panic("DescribeGraph")
}

func (client LightningMockClient) GetChanInfo(ctx context.Context, in *lnrpc.ChanInfoRequest, opts ...grpc.CallOption) (*lnrpc.ChannelEdge, error) {
	panic("GetChanInfo")
}

func (client LightningMockClient) GetNodeInfo(ctx context.Context, in *lnrpc.NodeInfoRequest, opts ...grpc.CallOption) (*lnrpc.NodeInfo, error) {
	panic("GetNodeInfo")
}

func (client LightningMockClient) QueryRoutes(ctx context.Context, in *lnrpc.QueryRoutesRequest, opts ...grpc.CallOption) (*lnrpc.QueryRoutesResponse, error) {
	panic("QueryRoutes")
}

func (client LightningMockClient) GetNetworkInfo(ctx context.Context, in *lnrpc.NetworkInfoRequest, opts ...grpc.CallOption) (*lnrpc.NetworkInfo, error) {
	panic("GetNetworkInfo")
}

func (client LightningMockClient) StopDaemon(ctx context.Context, in *lnrpc.StopRequest, opts ...grpc.CallOption) (*lnrpc.StopResponse, error) {
	panic("StopDaemon")
}

func (client LightningMockClient) SubscribeChannelGraph(ctx context.Context, in *lnrpc.GraphTopologySubscription, opts ...grpc.CallOption) (lnrpc.Lightning_SubscribeChannelGraphClient, error) {
	panic("SubscribeChannelGraph")
}

func (client LightningMockClient) DebugLevel(ctx context.Context, in *lnrpc.DebugLevelRequest, opts ...grpc.CallOption) (*lnrpc.DebugLevelResponse, error) {
	panic("DebugLevel")
}

func (client LightningMockClient) FeeReport(ctx context.Context, in *lnrpc.FeeReportRequest, opts ...grpc.CallOption) (*lnrpc.FeeReportResponse, error) {
	panic("FeeReport")
}

func (client LightningMockClient) UpdateChannelPolicy(ctx context.Context, in *lnrpc.PolicyUpdateRequest, opts ...grpc.CallOption) (*lnrpc.PolicyUpdateResponse, error) {
	panic("UpdateChannelPolicy")
}

func (client LightningMockClient) ForwardingHistory(ctx context.Context, in *lnrpc.ForwardingHistoryRequest, opts ...grpc.CallOption) (*lnrpc.ForwardingHistoryResponse, error) {
	panic("ForwardingHistory")
}

func (client LightningMockClient) ExportChannelBackup(ctx context.Context, in *lnrpc.ExportChannelBackupRequest, opts ...grpc.CallOption) (*lnrpc.ChannelBackup, error) {
	panic("ExportChannelBackup")
}

func (client LightningMockClient) ExportAllChannelBackups(ctx context.Context, in *lnrpc.ChanBackupExportRequest, opts ...grpc.CallOption) (*lnrpc.ChanBackupSnapshot, error) {
	panic("ExportAllChannelBackups")
}

func (client LightningMockClient) VerifyChanBackup(ctx context.Context, in *lnrpc.ChanBackupSnapshot, opts ...grpc.CallOption) (*lnrpc.VerifyChanBackupResponse, error) {
	panic("VerifyChanBackup")
}

func (client LightningMockClient) RestoreChannelBackups(ctx context.Context, in *lnrpc.RestoreChanBackupRequest, opts ...grpc.CallOption) (*lnrpc.RestoreBackupResponse, error) {
	panic("RestoreChannelBackups")
}

func (client LightningMockClient) SubscribeChannelBackups(ctx context.Context, in *lnrpc.ChannelBackupSubscription, opts ...grpc.CallOption) (lnrpc.Lightning_SubscribeChannelBackupsClient, error) {
	panic("SubscribeChannelBackups")
}

func (client LightningMockClient) NewAddress(ctx context.Context, in *lnrpc.NewAddressRequest, opts ...grpc.CallOption) (*lnrpc.NewAddressResponse, error) {
	return &lnrpc.NewAddressResponse{
		Address: "sb1qnl462s336uu4n8xanhyvpega4zwjr9jrhc26x4",
	}, nil
}

func (client LightningMockClient) SendCoins(ctx context.Context, in *lnrpc.SendCoinsRequest, opts ...grpc.CallOption) (*lnrpc.SendCoinsResponse, error) {
	return &lnrpc.SendCoinsResponse{
		Txid: "this_is_a_fake_txid",
	}, nil
}

func (client LightningMockClient) AddInvoice(ctx context.Context,
	in *lnrpc.Invoice, opts ...grpc.CallOption) (
	*lnrpc.AddInvoiceResponse, error) {
	return &lnrpc.AddInvoiceResponse{}, nil
}

func (client LightningMockClient) LookupInvoice(ctx context.Context,
	in *lnrpc.PaymentHash, opts ...grpc.CallOption) (*lnrpc.Invoice, error) {
	return &client.InvoiceResponse, nil
}

func (client LightningMockClient) DecodePayReq(ctx context.Context,
	in *lnrpc.PayReqString, opts ...grpc.CallOption) (*lnrpc.PayReq, error) {
	return &client.DecodePayReqResponse, nil
}

func (client LightningMockClient) SendPaymentSync(ctx context.Context,
	in *lnrpc.SendRequest, opts ...grpc.CallOption) (
	*lnrpc.SendResponse, error) {
	return &client.SendPaymentSyncResponse, nil
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
		Network:   "regtest",
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
func GetBitcoindConfig(t *testing.T) BitcoindConfig {
	return BitcoindConfig{
		P2pPort:        getPortOrFail(t),
		RpcPort:        getPortOrFail(t),
		User:           "rpc_user_for_tests",
		Password:       "rpc_pass_for_tests",
		ZmqPubRawTx:    fmt.Sprintf("tcp://0.0.0.0:%d", getPortOrFail(t)),
		ZmqPubRawBlock: fmt.Sprintf("tcp://0.0.0.0:%d", getPortOrFail(t)),
	}

}

// BitcoindConfig contains everything we need to reliably start a regtest
// bitcoind node
type BitcoindConfig struct {
	RpcPort        int
	P2pPort        int
	User           string
	Password       string
	ZmqPubRawTx    string
	ZmqPubRawBlock string
}

// ToConnConfig converts this BitcoindConfig to the format the rpcclient
// library expects
func (conf *BitcoindConfig) ToConnConfig() *rpcclient.ConnConfig {
	return &rpcclient.ConnConfig{
		Host:         fmt.Sprintf("127.0.0.1:%d", conf.RpcPort),
		User:         conf.User,
		Pass:         conf.Password,
		DisableTLS:   true, // Bitcoin Core doesn't do TLS
		HTTPPostMode: true, // Bitcoin Core only supports HTTP POST mode
	}

}

// GetBitcoindClientOrFail returns a bitcoind RPC client, corresponding to
// the given configuration.
func GetBitcoindClientOrFail(t *testing.T, conf BitcoindConfig) *rpcclient.Client {
	// Bitcoin Core doesn't do notifications
	var notificationHandler *rpcclient.NotificationHandlers = nil

	client, err := rpcclient.New(conf.ToConnConfig(), notificationHandler)
	if err != nil {
		testutil.FatalMsg(t, err)
	}

	return client
}

// retry retries the given function until it doesn't fail. It doubles the
// period between attempts each time.
// Cribbed from https://upgear.io/blog/simple-golang-retry-function/
func retry(attempts int, sleep time.Duration, fn func() error) error {
	if err := fn(); err != nil {
		if attempts > 1 {
			time.Sleep(sleep)
			return retry(attempts-1, 2*sleep, fn)
		}
		return err
	}
	return nil
}

func getTotalRetryDuration(attempts int, sleep time.Duration) time.Duration {
	if attempts <= 0 {
		return sleep
	}
	return sleep + getTotalRetryDuration(attempts-1, sleep*2)
}

// StartBitcoindOrFail starts a bitcoind node with the given configuration,
// with the data directory set to the users temporary directory. The function
// returns the created client, as well as a function that cleans up the operation
// (stopping the node and deleting the data directory).
func StartBitcoindOrFail(t *testing.T, conf BitcoindConfig) (client *rpcclient.Client, cleanup func() error) {
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
	if err := retry(retryAttempts, retrySleepDuration, readPidFile); err != nil {
		testutil.FatalMsgf(t, "Could not read bitcoind pid file after %d attempts and %s total sleep duration",
			retryAttempts, getTotalRetryDuration(retryAttempts, retrySleepDuration))
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
	if err := retry(retryAttempts, retrySleepDuration, client.Ping); err != nil {
		testutil.FatalMsgf(t, "Could not communicate with bitcoind after %d attempts and %s total sleep duration",
			retryAttempts, getTotalRetryDuration(retryAttempts, retrySleepDuration))
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
		if err := retry(retryAttempts, retrySleepDuration, negativePing); err != nil {
			return fmt.Errorf("could communicate with stopped bitcoind after %d attempts and %s total sleep duration",
				retryAttempts, getTotalRetryDuration(retryAttempts, retrySleepDuration))
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
func StartLndOrFail(t *testing.T, bitcoindConfig BitcoindConfig, lndConfig ln.LightningConfig) (client lnrpc.LightningClient, cleanup func() error) {
	if lndConfig.RPCServer == "" {
		testutil.FatalMsg(t, "lndConfig.RPCServer needs to be set, was empty")
	}
	if lndConfig.LndDir == "" {
		testutil.FatalMsg(t, "lndConfig.LndDir needs to be set, was empty")
	}
	if lndConfig.Network != "regtest" {
		testutil.FatalMsg(t, "lndConfig.Network was not regtest! Support for this is not implemented")
	}

	if lndConfig.MacaroonPath == "" {
		lndConfig.MacaroonPath = filepath.Join(lndConfig.LndDir, "data", "chain", "bitcoin", lndConfig.Network, "admin.macaroon")
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

	if err := retry(retryAttempts, retrySleepDuration, isReady); err != nil {
		testutil.FatalMsgf(t, "lnd cert and macaroon file did not greated after waiting %d",
			getTotalRetryDuration(retryAttempts, retrySleepDuration))
	}
	log.Debugf("lnd cert file and macaroon file exists")

	var lnd lnrpc.LightningClient
	var err error
	getLnd := func() error {
		lnd, err = ln.NewLNDClient(lndConfig)
		return err
	}
	if err := retry(retryAttempts, retrySleepDuration, getLnd); err != nil {
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
		if err := retry(retryAttempts, retrySleepDuration, negativeGetInfo); err != nil {
			return fmt.Errorf("could communicate with stopped lnd after %d attempts and %s total sleep duration",
				retryAttempts, getTotalRetryDuration(retryAttempts, retrySleepDuration))
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
