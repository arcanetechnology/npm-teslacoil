//+build integration

package apitxs_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/btcsuite/btcutil"
	"gitlab.com/arcanecrypto/teslacoil/api/apierr"

	"gitlab.com/arcanecrypto/teslacoil/models/transactions"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/sirupsen/logrus"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/arcanecrypto/teslacoil/api"
	"gitlab.com/arcanecrypto/teslacoil/models/users/balance"
	"gitlab.com/arcanecrypto/teslacoil/testutil/mock"

	"gitlab.com/arcanecrypto/teslacoil/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/ln"
	"gitlab.com/arcanecrypto/teslacoil/models/users"

	"github.com/brianvoe/gofakeit"
	"github.com/lightningnetwork/lnd/lnrpc"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/httptestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/nodetestutil"
)

func init() {
	// we're not closing DB connections here...
	// this probably shouldn't matter, as the conn.
	// closes when the process exits anyway
	testDB = testutil.InitDatabase(databaseConfig)
	databaseConfig = testutil.GetDatabaseConfig("api_txs_integration")
	conf = api.Config{LogLevel: logrus.InfoLevel, Network: chaincfg.RegressionNetParams}
}

func TestCreateInvoiceRoute(t *testing.T) {
	nodetestutil.RunWithLnd(t, false, func(lnd lnrpc.LightningClient) {
		app, err := api.NewApp(testDB,
			lnd,
			mock.GetMockSendGridClient(),
			bitcoind.TeslacoilBitcoindMockClient{},
			testutil.GetMockHttpPoster(),
			conf)
		require.NoError(t, err)
		h := httptestutil.NewTestHarness(app.Router, testDB)

		password := gofakeit.Password(true, true, true, true, true, 32)
		accessToken, _ := h.CreateAndAuthenticateUser(t, users.CreateUserArgs{
			Email:    gofakeit.Email(),
			Password: password,
		})

		t.Run("Create an invoice without memo and description", func(t *testing.T) {

			amountSat := fakeInvoiceAmount()

			req := httptestutil.GetAuthRequest(t,
				httptestutil.AuthRequestArgs{
					AccessToken: accessToken,
					Path:        "/invoices/create",
					Method:      "POST",
					Body: fmt.Sprintf(`{
					"amountSat": %d
				}`, amountSat),
				})

			res := h.AssertResponseOkWithJson(t, req)
			testutil.AssertMsg(t, res["memo"] == nil, "Memo was not empty")
			testutil.AssertMsg(t, res["description"] == nil, "Description was not empty")

		})

		t.Run("Create an invoice with memo and description", func(t *testing.T) {

			amountSat := fakeInvoiceAmount()

			memo := gofakeit.Sentence(gofakeit.Number(1, 20))
			description := gofakeit.Sentence(gofakeit.Number(1, 20))

			req := httptestutil.GetAuthRequest(t,
				httptestutil.AuthRequestArgs{
					AccessToken: accessToken,
					Path:        "/invoices/create",
					Method:      "POST",
					Body: fmt.Sprintf(`{
					"amountSat": %d,
					"memo": %q,
					"description": %q
				}`, amountSat, memo, description),
				})

			res := h.AssertResponseOkWithJson(t, req)
			testutil.AssertEqual(t, res["memo"], memo)
			testutil.AssertEqual(t, res["description"], description)

		})

	})
}

func TestPayInvoice(t *testing.T) {

	nodetestutil.RunWithBitcoindAndLndPair(t, func(lnd1 lnrpc.LightningClient, lnd2 lnrpc.LightningClient, bitcoind bitcoind.TeslacoilBitcoind) {
		app, err := api.NewApp(testDB,
			lnd1,
			mock.GetMockSendGridClient(),
			bitcoind,
			testutil.GetMockHttpPoster(),
			conf)
		require.NoError(t, err)
		h = httptestutil.NewTestHarness(app.Router, testDB)

		password := gofakeit.Password(true, true, true, true, true, 32)
		accessToken, userID := h.CreateAndAuthenticateUser(t, users.CreateUserArgs{
			Email:    gofakeit.Email(),
			Password: password,
		})
		const initialBalance = 5 * btcutil.SatoshiPerBitcoin
		h.GiveUserBalance(t, lnd1, bitcoind, accessToken, initialBalance)

		t.Run("can send payment", func(t *testing.T) {
			amountSat := fakeInvoiceAmount()
			paymentRequest, err := lnd2.AddInvoice(context.Background(), &lnrpc.Invoice{
				Value: amountSat,
			})
			require.NoError(t, err)

			req := httptestutil.GetAuthRequest(t,
				httptestutil.AuthRequestArgs{
					AccessToken: accessToken,
					Path:        "/invoices/pay",
					Method:      "POST",
					Body: fmt.Sprintf(`{
					"paymentRequest": %q,
					"description": ""
				}`, paymentRequest.PaymentRequest),
				})

			_ = h.AssertResponseOkWithJson(t, req)
		})

		t.Run("invalid payment request is not OK", func(t *testing.T) {
			req := httptestutil.GetAuthRequest(t,
				httptestutil.AuthRequestArgs{
					AccessToken: accessToken,
					Path:        "/invoices/pay",
					Method:      "POST",
					Body: fmt.Sprintf(`{
					"paymentRequest": %q
				}`, "a bad payment request"),
				})

			_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
		})

		t.Run("can set description", func(t *testing.T) {
			amountSat := fakeInvoiceAmount()
			paymentRequest, err := lnd2.AddInvoice(context.Background(), &lnrpc.Invoice{
				Value: amountSat,
			})
			require.NoError(t, err)
			description := gofakeit.HipsterSentence(5)
			req := httptestutil.GetAuthRequest(t,
				httptestutil.AuthRequestArgs{
					AccessToken: accessToken,
					Path:        "/invoices/pay",
					Method:      "POST",
					Body: fmt.Sprintf(`{
					"paymentRequest": %q,
					"description": %q
				}`, paymentRequest.PaymentRequest, description),
				})

			res := h.AssertResponseOkWithJson(t, req)
			require.NotNil(t, res["description"])
		})

		t.Run("sending invoice with bad path does not decrease users balance", func(t *testing.T) {
			prepaymentBalance, err := balance.ForUser(testDB, userID)
			require.NoError(t, err)

			amountSat := fakeInvoiceAmount()

			paymentRequest, err := lnd1.AddInvoice(context.Background(), &lnrpc.Invoice{
				Value: amountSat,
			})
			require.NoError(t, err)

			description := gofakeit.HipsterSentence(5)
			req := httptestutil.GetAuthRequest(t,
				httptestutil.AuthRequestArgs{
					AccessToken: accessToken,
					Path:        "/invoices/pay",
					Method:      "POST",
					Body: fmt.Sprintf(`{
					"paymentRequest": %q,
					"description": %q
				}`, paymentRequest.PaymentRequest, description),
				})

			_, _ = h.AssertResponseNotOk(t, req)

			postpaymentBalance, err := balance.ForUser(testDB, userID)
			assert.Equal(t, prepaymentBalance, postpaymentBalance)
		})

		t.Run("paying invoice of other teslacoil users results in internal transfer", func(t *testing.T) {
			t.Parallel()
			_, recipientUserID := h.CreateAndAuthenticateUser(t, users.CreateUserArgs{
				Email:    gofakeit.Email(),
				Password: password,
			})
			amount := fakeInvoiceAmount()

			offchain, err := transactions.CreateTeslacoilInvoice(testDB, lnd1, transactions.NewOffchainOpts{
				UserID:    recipientUserID,
				AmountSat: amount,
			})
			assert.NoError(t, err)

			prePaymentBalance, err := balance.ForUser(testDB, recipientUserID)
			assert.NoError(t, err)

			req := httptestutil.GetAuthRequest(t,
				httptestutil.AuthRequestArgs{
					AccessToken: accessToken,
					Path:        "/invoices/pay",
					Method:      "POST",
					Body: fmt.Sprintf(`{
					"paymentRequest": %q
				}`, offchain.PaymentRequest),
				})

			res := h.AssertResponseOkWithJson(t, req)

			fmt.Println(res)
			postPaymentBalance, err := balance.ForUser(testDB, recipientUserID)
			assert.NoError(t, err)

			assert.Equal(t, prePaymentBalance.Sats()+amount, postPaymentBalance.Sats())
			assert.True(t, res["internalTransfer"].(bool))
		})

		t.Run("paying own invoice returns specific error", func(t *testing.T) {
			t.Parallel()
			offchain, err := transactions.CreateTeslacoilInvoice(testDB, lnd1, transactions.NewOffchainOpts{
				UserID:    userID,
				AmountSat: 5000,
			})
			assert.NoError(t, err)

			req := httptestutil.GetAuthRequest(t,
				httptestutil.AuthRequestArgs{
					AccessToken: accessToken,
					Path:        "/invoices/pay",
					Method:      "POST",
					Body: fmt.Sprintf(`{
					"paymentRequest": %q
				}`, offchain.PaymentRequest),
				})

			_, err_ := h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
			assert.True(t, apierr.ErrCannotPayOwnInvoice.Is(err_))
		})
	})
}

func fakeInvoiceAmount() int64 {
	return int64(gofakeit.Number(0, ln.MaxAmountSatPerInvoice))
}
