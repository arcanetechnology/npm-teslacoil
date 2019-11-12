package apitxs_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/arcanecrypto/teslacoil/api"
	"gitlab.com/arcanecrypto/teslacoil/api/apierr"
	"gitlab.com/arcanecrypto/teslacoil/async"
	"gitlab.com/arcanecrypto/teslacoil/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/ln"
	"gitlab.com/arcanecrypto/teslacoil/models/apikeys"
	"gitlab.com/arcanecrypto/teslacoil/models/transactions"
	"gitlab.com/arcanecrypto/teslacoil/models/users"
	"gitlab.com/arcanecrypto/teslacoil/models/users/balance"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/httptestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/lntestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/mock"
	"gitlab.com/arcanecrypto/teslacoil/testutil/nodetestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/txtest"
	"gitlab.com/arcanecrypto/teslacoil/testutil/userstestutil"
)

var (
	databaseConfig = testutil.GetDatabaseConfig("tx_routes")
	testDB         *db.DB
	conf           = api.Config{
		LogLevel: logrus.InfoLevel,
		Network:  chaincfg.RegressionNetParams,
	}

	h httptestutil.TestHarness

	mockSendGridClient                             = mock.GetMockSendGridClient()
	mockLightningClient lnrpc.LightningClient      = lntestutil.GetLightningMockClient()
	mockBitcoindClient  bitcoind.TeslacoilBitcoind = bitcoind.GetBitcoinMockClient()
	mockHttpPoster                                 = testutil.GetMockHttpPoster()
)

func init() {
	testDB = testutil.InitDatabase(databaseConfig)

	app, err := api.NewApp(testDB, mockLightningClient,
		mockSendGridClient, mockBitcoindClient,
		mockHttpPoster, conf)
	if err != nil {
		panic(err.Error())
	}

	h = httptestutil.NewTestHarness(app.Router, testDB)
}

func TestMain(m *testing.M) {
	build.SetLogLevel(logrus.DebugLevel)

	// new values for gofakeit every time
	gofakeit.Seed(0)

	result := m.Run()

	if err := nodetestutil.CleanupNodes(); err != nil {
		panic(err)
	}

	if err := testDB.Close(); err != nil {
		panic(err)
	}
	os.Exit(result)
}

func TestGetTransaction(t *testing.T) {
	t.Parallel()
	token, userId := h.CreateAndAuthenticateUser(t, users.CreateUserArgs{
		Email:    gofakeit.Email(),
		Password: gofakeit.Password(true, true, true, true, true, 21),
	})

	ids := createFakeDeposits(t, 3, token)

	t.Run("reject request with bad ID parameter", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions/foobar",
			Method:      "GET",
		})
		_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})

	t.Run("not find non-existant TX", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions/99999",
			Method:      "GET",
		})
		_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusNotFound)
	})

	t.Run("not find TX for other user", func(t *testing.T) {
		t.Parallel()
		otherUser := userstestutil.CreateUserOrFail(t, testDB)
		txForOtherUser := txtest.InsertFakeIncomingOrFail(t, testDB, otherUser.ID)

		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        fmt.Sprintf("/transactions/%d", txForOtherUser.ID),
			Method:      "GET",
		})
		_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusNotFound)
	})

	t.Run("can get transaction by ID", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        fmt.Sprintf("/transactions/%d", ids[0]),
			Method:      "GET",
		})

		var trans transactions.Transaction
		h.AssertResponseOKWithStruct(t, req, &trans)

		assert.Equal(t, trans.ID, ids[0])
	})

	t.Run("can get an offchain TX by ID", func(t *testing.T) {
		t.Parallel()
		tx := txtest.InsertFakeOffChainOrFail(t, testDB, userId)

		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        fmt.Sprintf("/transactions/%d", tx.ID),
			Method:      "GET",
		})
		res := h.AssertResponseOkWithJson(t, req)
		assert.Equal(t, float64(tx.ID), res["id"])
		assert.Equal(t, float64(tx.UserID), res["userId"])
		assert.Equal(t, "lightning", res["type"])
	})

	t.Run("can get an onchain TX by ID", func(t *testing.T) {
		t.Parallel()
		tx := txtest.InsertFakeOnChainOrFail(t, testDB, userId)

		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        fmt.Sprintf("/transactions/%d", tx.ID),
			Method:      "GET",
		})
		res := h.AssertResponseOkWithJson(t, req)
		assert.Equal(t, float64(tx.ID), res["id"])
		assert.Equal(t, float64(tx.UserID), res["userId"])
		assert.Equal(t, "blockchain", res["type"])
	})

	t.Run("can get a onchain TX with funds", func(t *testing.T) {
		t.Parallel()

		tx := getOnchainWithMoney(userId)
		inserted, err := transactions.InsertOnchain(testDB, tx)
		require.NoError(t, err)

		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        fmt.Sprintf("/transactions/%d", inserted.ID),
			Method:      "GET",
		})
		res := h.AssertResponseOkWithJson(t, req)
		assert.Equal(t, float64(*inserted.AmountSat), res["amountSat"])
		assert.Equal(t, float64(*inserted.Vout), res["vout"])
		assert.Equal(t, *inserted.Txid, res["txid"])
		const layout = "2006-01-02T15:04:05.000000Z"
		foundTime, err := time.Parse(layout, res["createdAt"].(string))
		require.NoError(t, err)

		assert.WithinDuration(t, *inserted.ReceivedMoneyAt, foundTime, time.Millisecond*100)
	})
}

func TestGetTransactionsBothKinds(t *testing.T) {
	t.Parallel()

	token, userId := h.CreateAndAuthenticateUser(t, users.CreateUserArgs{
		Email:    gofakeit.Email(),
		Password: gofakeit.Password(true, true, true, true, true, 21),
	})

	var wg sync.WaitGroup
	incomingOnchainTxs := gofakeit.Number(1, 30)
	wg.Add(1)
	go func() {
		for i := 0; i < incomingOnchainTxs; i++ {
			txtest.InsertFakeIncomingOnchainorFail(t, testDB, userId)
		}
		wg.Done()
	}()

	outgoingOnchainTxs := gofakeit.Number(1, 30)
	wg.Add(1)
	go func() {
		for i := 0; i < outgoingOnchainTxs; i++ {
			txtest.InsertFakeOutgoingOnchainorFail(t, testDB, userId)
		}
		wg.Done()
	}()

	outgoingOffchainTxs := gofakeit.Number(1, 30)
	wg.Add(1)
	go func() {
		for i := 0; i < outgoingOffchainTxs; i++ {
			txtest.InsertFakeOutgoingOffchainOrFail(t, testDB, userId)
		}
		wg.Done()
	}()

	incomingOffchainTxs := gofakeit.Number(1, 30)
	wg.Add(1)
	go func() {
		for i := 0; i < incomingOffchainTxs; i++ {
			txtest.InsertFakeIncomingOffchainOrFail(t, testDB, userId)
		}
		wg.Done()
	}()

	req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
		AccessToken: token,
		Path:        "/transactions",
		Method:      "GET",
	})

	// wait for TXs to be inserted
	if async.WaitTimeout(&wg, time.Second*3) {
		t.Fatal("TX creation timed out")
	}

	response := h.AssertResponseOkWithJsonList(t, req)
	assert.Len(t, response, incomingOffchainTxs+incomingOnchainTxs+outgoingOffchainTxs+outgoingOnchainTxs)
}

func TestGetTransactionsEmpty(t *testing.T) {
	t.Parallel()
	token, _ := h.CreateAndAuthenticateUser(t, users.CreateUserArgs{
		Email:    gofakeit.Email(),
		Password: gofakeit.Password(true, true, true, true, true, 32),
	})

	req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
		AccessToken: token,
		Path:        "/transactions",
		Method:      "GET",
	})

	txs := h.AssertResponseOkWithJsonList(t, req)
	assert.Len(t, txs, 0)
}

func TestGetTransactionsFiltering(t *testing.T) {
	t.Parallel()
	token, _ := h.CreateAndAuthenticateUser(t, users.CreateUserArgs{
		Email:    gofakeit.Email(),
		Password: gofakeit.Password(true, true, true, true, true, 21),
	})
	const incomingOnchainTxs = 10
	createFakeDeposits(t, incomingOnchainTxs, token)

	t.Run("should reject non-numeric limit", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions?limit=baz",
			Method:      "GET",
		})

		_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})

	t.Run("should reject non-numeric offset", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions?offset=qux",
			Method:      "GET",
		})

		_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})

	t.Run("get transactions without query params should get all", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions",
			Method:      "GET",
		})

		assertGetsRightAmount(t, req, 10)
	})
	t.Run("get transactions with limit 10 should get 10", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions?limit=10",
			Method:      "GET",
		})

		assertGetsRightAmount(t, req, 10)
	})
	t.Run("get transactions with limit 5 should get 5", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions?limit=5",
			Method:      "GET",
		})

		assertGetsRightAmount(t, req, 5)
	})
	t.Run("get transactions with limit 0 should get all", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions?limit=0",
			Method:      "GET",
		})

		assertGetsRightAmount(t, req, 10)
	})
	t.Run("get /transactions with offset 10 should get 0", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions?offset=10",
			Method:      "GET",
		})

		assertGetsRightAmount(t, req, 0)
	})

	t.Run("get /transactions with offset 0 should get 10", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions?offset=0",
			Method:      "GET",
		})

		assertGetsRightAmount(t, req, 10)
	})

	t.Run("get /transactions with offset 5 should get 5", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions?offset=5",
			Method:      "GET",
		})

		assertGetsRightAmount(t, req, 5)
	})
	t.Run("get /transactions with offset 5 and limit 3 should get 3", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions?limit=3&offset=5",
			Method:      "GET",
		})

		assertGetsRightAmount(t, req, 3)
	})
}

func TestNewDeposit(t *testing.T) {
	t.Parallel()
	token, _ := h.CreateAndAuthenticateUser(t, users.CreateUserArgs{
		Email:    gofakeit.Email(),
		Password: gofakeit.Password(true, true, true, true, true, 21),
	})

	t.Run("can create new deposit with description", func(t *testing.T) {
		description := "fooDescription"
		t.Parallel()
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/deposit",
			Method:      "POST",
			Body: fmt.Sprintf(
				`{ "forceNewAddress": %t, "description": %q }`,
				true,
				description),
		})

		tx := h.AssertResponseOkWithJson(t, req)
		assert.Equal(t, tx["description"], description)
	})

	t.Run("can create new deposit without description", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/deposit",
			Method:      "POST",
			Body: fmt.Sprintf(
				`{"forceNewAddress":%t}`,
				true),
		})

		tx := h.AssertResponseOkWithJson(t, req)
		assert.Nil(t, tx["description"])

	})
}

func TestCreateInvoice(t *testing.T) {

	randomMockClient := lntestutil.GetRandomLightningMockClient()
	app, _ := api.NewApp(testDB, randomMockClient, mockSendGridClient,
		mockBitcoindClient, mockHttpPoster, conf)
	otherH := httptestutil.NewTestHarness(app.Router, testDB)

	password := gofakeit.Password(true, true, true, true, true, 32)
	email := gofakeit.Email()
	accessToken, _ := otherH.CreateAndAuthenticateUser(t, users.CreateUserArgs{
		Email:    email,
		Password: password,
	})

	t.Run("Not create an invoice with non-positive amount ", func(t *testing.T) {

		// gofakeit panics with too low value here...
		// https://github.com/brianvoe/gofakeit/issues/56
		amountSat := gofakeit.Number(math.MinInt64+2, -1)

		req := httptestutil.GetAuthRequest(t,
			httptestutil.AuthRequestArgs{
				AccessToken: accessToken,
				Path:        "/invoices/create",
				Method:      "POST",
				Body: fmt.Sprintf(`{
					"amountSat": %d
				}`, amountSat),
			})

		_, _ = otherH.AssertResponseNotOk(t, req)
	})

	t.Run("Not create an invoice with too large amount", func(t *testing.T) {

		amountSat := gofakeit.Number(ln.MaxAmountSatPerInvoice, math.MaxInt64)

		req := httptestutil.GetAuthRequest(t,
			httptestutil.AuthRequestArgs{
				AccessToken: accessToken,
				Path:        "/invoices/create",
				Method:      "POST",
				Body: fmt.Sprintf(`{
					"amountSat": %d
				}`, amountSat),
			})

		_, _ = otherH.AssertResponseNotOk(t, req)
	})

	t.Run("Not create an invoice with a very long customer order id", func(t *testing.T) {
		t.Parallel()
		longId := gofakeit.Sentence(1000)

		req := httptestutil.GetAuthRequest(t,
			httptestutil.AuthRequestArgs{
				AccessToken: accessToken,
				Path:        "/invoices/create",
				Method:      "POST",
				Body: fmt.Sprintf(`{
					"amountSat": %d,
					"orderId": %q
				}`, 1337, longId),
			})

		_, _ = otherH.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)

	})

	t.Run("Create an invoice with a customer order id", func(t *testing.T) {
		t.Parallel()
		const orderId = "this-is-my-order-id"

		req := httptestutil.GetAuthRequest(t,
			httptestutil.AuthRequestArgs{
				AccessToken: accessToken,
				Path:        "/invoices/create",
				Method:      "POST",
				Body: fmt.Sprintf(`{
					"amountSat": %d,
					"orderId": %q
				}`, 1337, orderId),
			})

		res := otherH.AssertResponseOkWithJson(t, req)
		assert.Equal(t, orderId, res["customerOrderId"])

		t.Run("create an invoice with the same order ID twice", func(t *testing.T) {
			t.Parallel()
			req := httptestutil.GetAuthRequest(t,
				httptestutil.AuthRequestArgs{
					AccessToken: accessToken,
					Path:        "/invoices/create",
					Method:      "POST",
					Body: fmt.Sprintf(`{
					"amountSat": %d,
					"orderId": %q
				}`, 1337, orderId),
				})

			otherH.AssertResponseOk(t, req)

		})
	})

	t.Run("Not create an invoice with zero amount ", func(t *testing.T) {
		t.Parallel()

		req := httptestutil.GetAuthRequest(t,
			httptestutil.AuthRequestArgs{
				AccessToken: accessToken,
				Path:        "/invoices/create",
				Method:      "POST",
				Body: `{
					"amountSat": 0
				}`,
			})

		_, _ = otherH.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})

	t.Run("Not create an invoice with an invalid callback URL", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/invoices/create",
			Method:      "POST",
			Body: `{
				"amountSat": 1000,
				"callbackUrl": "bad-url"
			}`,
		})
		_, _ = otherH.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})

	t.Run("create an invoice with a valid callback URL", func(t *testing.T) {
		mockInvoice, _ := ln.AddInvoice(randomMockClient, lnrpc.Invoice{})
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/invoices/create",
			Method:      "POST",
			Body: fmt.Sprintf(`{
				"amountSat": %d,
				"callbackUrl": "https://example.com"
			}`, mockInvoice.Value),
		})
		invoicesJson := otherH.AssertResponseOkWithJson(t, req)
		testutil.AssertMsg(t, invoicesJson["callbackUrl"] != nil, "callback URL was nil!")

		t.Run("receive a POST to the given URL when paying the invoice",
			func(t *testing.T) {
				user, err := users.GetByEmail(testDB, email)
				if err != nil {
					testutil.FatalMsg(t, err)
				}

				var apiKey apikeys.Key
				// check if there are any API keys
				if keys, err := apikeys.GetByUserId(testDB, user.ID); err == nil && len(keys) > 0 {
					apiKey = keys[0]
					// if not, try to create one, fail it if doesn't work
				} else if _, key, err := apikeys.New(testDB, user.ID, apikeys.AllPermissions, ""); err != nil {
					testutil.FatalMsg(t, err)
				} else {
					apiKey = key
				}

				_, err = transactions.HandleSettledInvoice(*mockInvoice, testDB, mockHttpPoster)
				require.NoError(t, err)

				checkPostSent := func() bool {
					return mockHttpPoster.GetSentPostRequests() == 1
				}

				// emails are sent in a go-routine, so can't assume they're sent fast
				// enough for test to pick up
				require.NoError(t, async.Await(8, time.Millisecond*20, checkPostSent))

				bodyBytes := mockHttpPoster.GetSentPostRequest(0)
				body := transactions.CallbackBody{}

				require.NoError(t, json.Unmarshal(bodyBytes, &body))

				hmac := hmac.New(sha256.New, apiKey.HashedKey)
				_, _ = hmac.Write([]byte(fmt.Sprintf("%d", body.Offchain.ID)))

				sum := hmac.Sum(nil)
				testutil.AssertEqual(t, sum, body.Hash)
			})
	})
}

func getTx(minAmountSat int64, userId int) transactions.Offchain {
	tx := txtest.MockOffchain(userId)
	if tx.Direction != transactions.INBOUND ||
		tx.Status != transactions.Offchain_COMPLETED ||
		tx.AmountMilliSat/1000 < minAmountSat {
		return getTx(minAmountSat, userId)
	}
	return tx
}

func TestWithdrawOnChain(t *testing.T) {
	t.Parallel()

	t.Run("regular withdrawal", func(t *testing.T) {
		t.Parallel()
		const withdrawAmount = 1234

		pass := gofakeit.Password(true, true, true, true, true, 32)
		user := userstestutil.CreateUserOrFailWithPassword(t, testDB, pass)
		tx := getTx(withdrawAmount, user.ID)
		_, err := transactions.InsertOffchain(testDB, tx)
		require.NoError(t, err)

		bal, err := balance.ForUser(testDB, user.ID)
		require.NoError(t, err)
		require.True(t, bal.Sats() > withdrawAmount, tx)

		accessToken, _ := h.AuthenticaticateUser(t, users.CreateUserArgs{
			Email:    user.Email,
			Password: pass,
		})

		const address = "bcrt1qvn9hnzlpgrvcmrusj6cfh6cvgppp2z8fqeuxmy"
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/withdraw",
			Method:      "POST",
			Body: fmt.Sprintf(`{
			"amountSat": %d,
			"address": %q
		}`, withdrawAmount, address),
		})

		res := h.AssertResponseOkWithJson(t, req)
		assert.Equal(t, address, res["address"])
		assert.Equal(t, float64(withdrawAmount), res["amountSat"])
		assert.Equal(t, false, res["confirmed"])
		assert.Equal(t, "blockchain", res["type"])

		balanceReq := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/users",
			Method:      "GET",
		})

		balanceRes := h.AssertResponseOkWithJson(t, balanceReq)
		expectedBalance := (tx.AmountMilliSat / 1000) - withdrawAmount

		testutil.AssertEqual(t, expectedBalance, balanceRes["balanceSats"])
	})

	t.Run("fail to withdraw too much", func(t *testing.T) {
		t.Parallel()

		pass := gofakeit.Password(true, true, true, true, true, 32)
		user := userstestutil.CreateUserOrFailWithPassword(t, testDB, pass)

		accessToken, _ := h.AuthenticaticateUser(t, users.CreateUserArgs{
			Email:    user.Email,
			Password: pass,
		})

		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/withdraw",
			Method:      "POST",
			Body: fmt.Sprintf(`{
			"amountSat": %d,
			"address": %q
		}`, 1337, "bcrt1qvn9hnzlpgrvcmrusj6cfh6cvgppp2z8fqeuxmy"),
		})

		_, err := h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
		testutil.AssertEqual(t, apierr.ErrBalanceTooLow, err)

	})

	t.Run("fail to withdraw with bad API key permission", func(t *testing.T) {
		t.Parallel()

		pass := gofakeit.Password(true, true, true, true, true, 32)
		user := userstestutil.CreateUserOrFailWithPassword(t, testDB, pass)

		key, _, err := apikeys.New(testDB, user.ID, apikeys.Permissions{
			ReadWallet:    true,
			CreateInvoice: true,
			EditAccount:   true,
		}, "")
		require.NoError(t, err)

		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: key.String(),
			Path:        "/withdraw",
			Method:      "POST",
			Body: fmt.Sprintf(`{
			"amountSat": %d,
			"address": %q
		}`, 1337, "bcrt1qvn9hnzlpgrvcmrusj6cfh6cvgppp2z8fqeuxmy"),
		})

		_, err = h.AssertResponseNotOkWithCode(t, req, http.StatusUnauthorized)
		assert.True(t, apierr.ErrBadApiKey.Is(err), err)

	})
}

func TestGetOffchainByPaymentRequest(t *testing.T) {
	t.Parallel()
	token, userID := h.CreateAndAuthenticateUser(t, users.CreateUserArgs{
		Email:    gofakeit.Email(),
		Password: gofakeit.Password(true, true, true, true, true, 21),
	})

	t.Run("can get transaction by payment request", func(t *testing.T) {
		t.Parallel()

		offchain := txtest.InsertFakeOffChainOrFail(t, testDB, userID)

		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        fmt.Sprintf("/invoices/%s", offchain.PaymentRequest),
			Method:      "GET",
		})
		res := h.AssertResponseOkWithJson(t, req)
		assert.Equal(t, offchain.PaymentRequest, res["paymentRequest"])
	})

	t.Run("non-existant payment request returns error", func(t *testing.T) {
		t.Parallel()

		payReq := txtest.MockPaymentRequest()

		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        fmt.Sprintf("/invoices/%s", payReq),
			Method:      "GET",
		})
		_, err := h.AssertResponseNotOkWithCode(t, req, http.StatusNotFound)
		assert.True(t, apierr.ErrTransactionNotFound.Is(err))
	})
	t.Run("bad payment request returns validation error", func(t *testing.T) {
		t.Parallel()

		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/invoices/not-a-payment-request",
			Method:      "GET",
		})
		_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})
}

func createFakeDeposit(t *testing.T, accessToken string, forceNewAddress bool, description string) int {
	req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
		AccessToken: accessToken,
		Path:        "/deposit",
		Method:      "POST",
		Body: fmt.Sprintf(
			`{ "forceNewAddress": %t, "description": %q }`,
			forceNewAddress,
			description),
	})

	var trans transactions.Transaction
	h.AssertResponseOKWithStruct(t, req, &trans)

	return trans.ID
}

func createFakeDeposits(t *testing.T, amount int, accessToken string) []int {
	t.Helper()

	ids := make([]int, amount)
	for i := 0; i < amount; i++ {
		ids[i] = createFakeDeposit(t, accessToken, true, "")
	}
	return ids
}

func assertGetsRightAmount(t *testing.T, req *http.Request, expected int) {
	var trans []transactions.Transaction
	h.AssertResponseOKWithStruct(t, req, &trans)
	assert.Equal(t, expected, len(trans))
}

func getOnchainWithMoney(userId int) transactions.Onchain {
	tx := txtest.MockOnchain(userId)
	if tx.Txid == nil {
		return getOnchainWithMoney(userId)
	}
	return tx
}
