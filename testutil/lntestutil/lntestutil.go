package lntestutil

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"math/rand"
	"testing"

	"github.com/brianvoe/gofakeit"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/lnrpc"

	"gitlab.com/arcanecrypto/teslacoil/ln"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
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

	value := int64(gofakeit.Number(1, ln.MaxAmountSatPerInvoice))

	return LightningMockClient{
		InvoiceResponse: lnrpc.Invoice{
			PaymentRequest: fmt.Sprintf("SomePayRequest%d", gofakeit.Number(0, 10000)),
			RHash:          doubleHash(invoicePreimage),
			RPreimage:      invoicePreimage,
			Expiry:         1337,
			State:          lnrpc.Invoice_SETTLED,
			Value:          value,
			AmtPaidMsat:    value * 1000,
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
		SendCoinsResponse: lnrpc.SendCoinsResponse{
			Txid: "0c10119609137327c72fe605452375c40727871bd18dad18db16da649e9bdcc1",
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
			Expiry:         1337,
			State:          lnrpc.Invoice_SETTLED,
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
		SendCoinsResponse: lnrpc.SendCoinsResponse{
			Txid: "0c10119609137327c72fe605452375c40727871bd18dad18db16da649e9bdcc1",
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
		LndDir:  tempDir,
		Network: chaincfg.RegressionNetParams,
		RPCHost: "localhost",
		RPCPort: port,
		P2pPort: testutil.GetPortOrFail(t),
	}
}
