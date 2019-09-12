package testutil

import (
	"context"

	"github.com/lightningnetwork/lnd/lnrpc"
	"google.golang.org/grpc/metadata"
)

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

func (m *MockSubscribeInvoicesClient) RecvMsg(msg interface{}) error {
	return nil
}

func (m *MockSubscribeInvoicesClient) Recv() (*lnrpc.Invoice, error) {
	return nil, nil
}
