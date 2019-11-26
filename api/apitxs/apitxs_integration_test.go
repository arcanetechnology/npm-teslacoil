//+build integration

package apitxs_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"testing"

	"github.com/btcsuite/btcutil"
	"gitlab.com/arcanecrypto/teslacoil/api/auth"
	"gitlab.com/arcanecrypto/teslacoil/testutil/userstestutil"

	"gitlab.com/arcanecrypto/teslacoil/api/apierr"

	"gitlab.com/arcanecrypto/teslacoil/models/transactions"

	"github.com/btcsuite/btcd/chaincfg"
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
	conf = api.Config{Network: chaincfg.RegressionNetParams}
}

func TestCreateInvoiceRoute(t *testing.T) {
	lnd := nodetestutil.GetLnd(t)
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
		assert.Nil(t, res["memo"])
		assert.Nil(t, res["description"])

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
		assert.Equal(t, res["memo"], memo)
		assert.Equal(t, res["description"], description)

	})
}

func TestPayInvoice(t *testing.T) {

	assertPreimageIsOfHash := func(t *testing.T, preimage string, hash string) {
		// we decode the base64 encoded string into a []byte
		decodedPreimage, err := hex.DecodeString(preimage)
		require.NoError(t, err)

		// then we hash the preimage using shasum256
		shasum := sha256.Sum256(decodedPreimage)

		assert.Equal(t, hex.EncodeToString(shasum[:]), hash)
	}

	lnd1, lnd2, bitcoind := nodetestutil.GetLndPairAndBitcoind(t)
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

		res := h.AssertResponseOkWithJson(t, req)
		preimage, ok := res["preimage"].(string)
		require.True(t, ok, res)
		hash, ok := res["hash"].(string)
		require.True(t, ok, res)

		assertPreimageIsOfHash(t, preimage, hash)
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

		postPaymentBalance, err := balance.ForUser(testDB, recipientUserID)
		assert.NoError(t, err)

		assert.Equal(t, prePaymentBalance.Sats()+amount, postPaymentBalance.Sats())
		assert.Equal(t, res["status"], "completed")
		assert.NotNil(t, res["preimage"])
		assert.Equal(t, hex.EncodeToString(offchain.HashedPreimage), res["hash"])
		assert.Equal(t, float64(offchain.AmountSat), res["amountSat"])
		assertPreimageIsOfHash(t, res["preimage"].(string), res["hash"].(string))

	})

	t.Run("cannot pay invoice for more than balance", func(t *testing.T) {
		const initialBalance = ln.MaxAmountSatPerInvoice / 4
		user := userstestutil.CreateUserWithBalanceOrFail(t, testDB, initialBalance)
		jwt, err := auth.CreateJwt(user.Email, user.ID)
		require.NoError(t, err)

		bal, err := balance.ForUser(testDB, user.ID)
		require.NoError(t, err)

		offchain, err := transactions.CreateTeslacoilInvoice(testDB, lnd2, transactions.NewOffchainOpts{
			UserID:    user.ID,
			AmountSat: bal.Sats() + 1,
		})
		require.NoError(t, err)

		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: jwt,
			Path:        "/invoices/pay",
			Method:      "POST",
			Body: fmt.Sprintf(`{
					"paymentRequest": %q
				}`, offchain.PaymentRequest),
		})

		_, err = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
		testutil.AssertEqualErr(t, apierr.ErrBalanceTooLow, err)

		// check that balance hasn't changed
		newBal, err := balance.ForUser(testDB, user.ID)
		require.NoError(t, err)

		assert.Equal(t, bal.MilliSats(), newBal.MilliSats())

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

	t.Run("cannot pay invoice with 0 amount", func(t *testing.T) {
		t.Parallel()
		invoice, err := lnd2.AddInvoice(context.Background(), &lnrpc.Invoice{})
		require.NoError(t, err)

		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/invoices/pay",
			Method:      "POST",
			Body: fmt.Sprintf(`{
					"paymentRequest": %q
				}`, invoice.PaymentRequest)})

		_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})
}

func fakeInvoiceAmount() int64 {
	return int64(gofakeit.Number(0, ln.MaxAmountSatPerInvoice))
}
