package lntestutil

import (
	"context"
	"sync"

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
