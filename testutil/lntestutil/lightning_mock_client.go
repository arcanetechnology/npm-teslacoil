package lntestutil

import (
	"context"
	"sync"

	"github.com/lightningnetwork/lnd/lnrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var (
	// check that mock clients satisfies interfaces
	_ lnrpc.LightningClient                   = LightningMockClient{}
	_ lnrpc.Lightning_SubscribeInvoicesClient = &MockSubscribeInvoicesClient{}
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
	chain := []*lnrpc.Chain{{
		Chain:   "bitcoin",
		Network: "regtest",
	}}

	return &lnrpc.GetInfoResponse{
		Chains: chain,
	}, nil
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

func (client LightningMockClient) AddInvoice(ctx context.Context,
	in *lnrpc.Invoice, opts ...grpc.CallOption) (
	*lnrpc.AddInvoiceResponse, error) {
	return &lnrpc.AddInvoiceResponse{}, nil
}

func (client LightningMockClient) SendCoins(ctx context.Context, in *lnrpc.SendCoinsRequest, opts ...grpc.CallOption) (*lnrpc.SendCoinsResponse, error) {
	return &client.SendCoinsResponse, nil
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

type MockSubscribeInvoicesClient struct{}

func (m *MockSubscribeInvoicesClient) Header() (metadata.MD, error) {
	panic("implement me")
}

func (m *MockSubscribeInvoicesClient) Trailer() metadata.MD {
	panic("implement me")
}

func (m *MockSubscribeInvoicesClient) CloseSend() error {
	return nil
}

func (m *MockSubscribeInvoicesClient) Context() context.Context {
	panic("implement me")
}

func (m *MockSubscribeInvoicesClient) SendMsg(msg interface{}) error {
	return nil
}

func blockForever() {
	wg := sync.WaitGroup{}
	wg.Add(1)
	wg.Wait()
}

// From LND docs:
// RecvMsg blocks until it receives a message into m or the stream is done.
//
// This function is called in a go routine that listens for invoices on our
// API. Seeing as this is a mocked LND with no invoices, it doesn't make sense
// to send anything to the API either. We therefore block forever.
func (m *MockSubscribeInvoicesClient) RecvMsg(msg interface{}) error {
	blockForever()
	return nil
}

func (m *MockSubscribeInvoicesClient) Recv() (*lnrpc.Invoice, error) {
	return nil, nil
}
