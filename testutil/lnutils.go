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
		Network:   "simnet",
		RPCServer: "localhost:10009",
	}
}
