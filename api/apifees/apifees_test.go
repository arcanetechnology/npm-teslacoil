package apifees_test

import (
	"net/http"
	"testing"

	"github.com/brianvoe/gofakeit"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/stretchr/testify/assert"
	"gitlab.com/arcanecrypto/teslacoil/api"
	"gitlab.com/arcanecrypto/teslacoil/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/httptestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/lntestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/mock"
	"gitlab.com/arcanecrypto/teslacoil/testutil/txtest"
)

var (
	conf = api.Config{
		Network: chaincfg.RegressionNetParams,
	}

	h httptestutil.TestHarness

	lnClient = lntestutil.LightningMockClient{
		QueryRoutesResponse: lnrpc.QueryRoutesResponse{
			Routes: []*lnrpc.Route{{
				Hops:          nil,
				TotalFeesMsat: 1234,
			}},
			SuccessProb: 0,
		},
	}
)

func init() {
	gofakeit.Seed(0)

	app, err := api.NewApp(nil, lnClient,
		mock.GetMockSendGridClient(), bitcoind.GetBitcoinMockClient(),
		testutil.GetMockHttpPoster(), conf)
	if err != nil {
		panic(err)
	}

	h = httptestutil.NewTestHarness(app.Router, nil)
}

func TestGetBlockChainFees(t *testing.T) {
	t.Parallel()

	t.Run("no target", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/fees",
			Method: "GET",
		})

		res := h.AssertResponseOkWithJson(t, req)

		assert.NotNil(t, res["satsPerByte"])
		assert.NotNil(t, res["averageTransaction"])
	})

	t.Run("with target", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/fees?target=10",
			Method: "GET",
		})

		res := h.AssertResponseOkWithJson(t, req)

		assert.NotNil(t, res["satsPerByte"])
		assert.NotNil(t, res["averageTransaction"])
	})

	t.Run("bad target", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/fees?target=-10",
			Method: "GET",
		})

		_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})
}

func TestGetLightningFees(t *testing.T) {
	t.Parallel()

	t.Run("good payment request", func(t *testing.T) {
		t.Parallel()
		payReq := txtest.MockPaymentRequest()
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/fees/" + payReq,
			Method: "GET",
		})

		res := h.AssertResponseOkWithJson(t, req)
		assert.Greater(t, res["milliSatoshis"], res["satoshis"])
	})

	t.Run("bad payment request", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/fees/foobar",
			Method: "GET",
		})

		_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})
}
