package testutil

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path"

	"github.com/lightningnetwork/lnd/lnrpc"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/ln"
	"gitlab.com/arcanecrypto/teslacoil/util"
	"google.golang.org/grpc"
)

var (
	SamplePreimage = func() []byte {
		encoded, _ := hex.DecodeString(SamplePreimageHex)
		return encoded
	}()
	SamplePreimageHex = "0123456789abcdef0123456789abcdef"
	SampleHash        = func() [32]byte {
		first := sha256.Sum256(SamplePreimage)
		return sha256.Sum256(first[:])
	}()
	SampleHashHex = hex.EncodeToString(SampleHash[:])
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

func (client LightningMockClient) NewAddress(ctx context.Context, in *lnrpc.NewAddressRequest, opts ...grpc.CallOption) (*lnrpc.NewAddressResponse, error) {
	panic("NewAddress")
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

// GetLightingConfig returns a LN config you can use where the content of the
// config does not matter at all
func GetLightingConfig() ln.LightningConfig {
	return ln.LightningConfig{
		LndDir: path.Join(
			util.GetEnvOrFail("GOPATH"), "src", "gitlab.com",
			"arcanecrypto", "teslacoil", "docker", ".alice"),
		Network:   "regtest",
		RPCServer: "localhost:10009",
	}
}
