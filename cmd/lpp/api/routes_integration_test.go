//+build integration

package api_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"gitlab.com/arcanecrypto/teslacoil/testutil/mock"

	"gitlab.com/arcanecrypto/teslacoil/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/ln"
	"gitlab.com/arcanecrypto/teslacoil/models/users"

	"github.com/brianvoe/gofakeit"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/cmd/lpp/api"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/httptestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/nodetestutil"
)

var (
	databaseConfig = testutil.GetDatabaseConfig("routes_integration")
	testDB         *db.DB
	conf           = api.Config{LogLevel: logrus.InfoLevel, Network: chaincfg.RegressionNetParams}
)

func init() {
	// we're not closing DB connections here...
	// this probably shouldn't matter, as the conn.
	// closes when the process exits anyway
	testDB = testutil.InitDatabase(databaseConfig)
}

func TestCreateInvoiceRoute(t *testing.T) {
	nodetestutil.RunWithLnd(t, false, func(lnd lnrpc.LightningClient) {
		app, err := api.NewApp(testDB,
			lnd,
			mock.GetMockSendGridClient(),
			bitcoind.TeslacoilBitcoindMockClient{},
			testutil.GetMockHttpPoster(),
			conf)
		if err != nil {
			testutil.FatalMsg(t, err)
		}

		h := httptestutil.NewTestHarness(app.Router, testDB)

		testutil.DescribeTest(t)

		password := gofakeit.Password(true, true, true, true, true, 32)
		accessToken, _ := h.CreateAndAuthenticateUser(t, users.CreateUserArgs{
			Email:    gofakeit.Email(),
			Password: password,
		})

		t.Run("Create an invoice without memo and description", func(t *testing.T) {
			testutil.DescribeTest(t)

			amountSat := gofakeit.Number(0, ln.MaxAmountSatPerInvoice)

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
			testutil.DescribeTest(t)

			amountSat := gofakeit.Number(0, ln.MaxAmountSatPerInvoice)

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
	testutil.DescribeTest(t)

	nodetestutil.RunWithBitcoindAndLndPair(t, func(lnd1 lnrpc.LightningClient, lnd2 lnrpc.LightningClient, bitcoind bitcoind.TeslacoilBitcoind) {
		app, err := api.NewApp(testDB,
			lnd1,
			mock.GetMockSendGridClient(),
			bitcoind,
			testutil.GetMockHttpPoster(),
			conf)
		if err != nil {
			testutil.FatalMsg(t, err)
		}

		h := httptestutil.NewTestHarness(app.Router, testDB)

		password := gofakeit.Password(true, true, true, true, true, 32)
		accessToken, userID := h.CreateAndAuthenticateUser(t, users.CreateUserArgs{
			Email:    gofakeit.Email(),
			Password: password,
		})
		h.GiveUserBalance(t, lnd1, bitcoind, accessToken, 50000000)
		// it takes time to propagate the confirmed balance to the lnd nodes,
		// therefore we sleep for 500 milliseconds
		//time.Sleep(500 * time.Millisecond)

		t.Run("can send payment", func(t *testing.T) {
			testutil.DescribeTest(t)

			amountSat := gofakeit.Number(0, ln.MaxAmountSatPerInvoice)
			paymentRequest, err := lnd2.AddInvoice(context.Background(), &lnrpc.Invoice{
				Value: int64(amountSat),
			})
			if err != nil {
				testutil.FatalMsgf(t, "could not create invoice: %v", err)
			}

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
			amountSat := gofakeit.Number(0, ln.MaxAmountSatPerInvoice)
			paymentRequest, err := lnd2.AddInvoice(context.Background(), &lnrpc.Invoice{
				Value: int64(amountSat),
			})
			if err != nil {
				testutil.FatalMsgf(t, "could not create invoice: %v", err)
			}

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
			testutil.AssertMsg(t, res["description"] != nil, "Description was empty")

		})

		t.Run("sending invoice with bad path does not decrease users balance", func(t *testing.T) {
			testutil.DescribeTest(t)

			user, err := users.GetByID(testDB, userID)
			if err != nil {
				testutil.FatalMsgf(t, "could not getbyid: %v", err)
			}
			balance := user.Balance

			amountSat := gofakeit.Number(0, ln.MaxAmountSatPerInvoice)

			paymentRequest, err := lnd1.AddInvoice(context.Background(), &lnrpc.Invoice{
				Value: int64(amountSat),
			})
			if err != nil {
				testutil.FatalMsgf(t, "could not create invoice: %v", err)
			}

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

			user, err = users.GetByID(testDB, userID)
			if err != nil {
				testutil.FatalMsgf(t, "could not getbyid: %v", err)
			}

			if user.Balance != balance {
				testutil.FatalMsgf(t, "expected users balance to not decrease and be %d, but was %d", balance, user.Balance)
			}
		})
	})
}
