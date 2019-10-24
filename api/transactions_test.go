package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/api/apierr"
	"gitlab.com/arcanecrypto/teslacoil/async"
	"gitlab.com/arcanecrypto/teslacoil/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/ln"
	"gitlab.com/arcanecrypto/teslacoil/models/apikeys"
	"gitlab.com/arcanecrypto/teslacoil/models/transactions"
	"gitlab.com/arcanecrypto/teslacoil/models/users"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/httptestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/lntestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/nodetestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/userstestutil"
)

var (
	databaseConfig = testutil.GetDatabaseConfig("routes")
	testDB         *db.DB
	conf           = Config{
		LogLevel: logrus.InfoLevel,
		Network:  chaincfg.RegressionNetParams,
	}

	h httptestutil.TestHarness

	mockSendGridClient                             = testutil.GetMockSendGridClient()
	mockLightningClient lnrpc.LightningClient      = lntestutil.GetLightningMockClient()
	mockBitcoindClient  bitcoind.TeslacoilBitcoind = bitcoind.GetBitcoinMockClient()
	mockHttpPoster                                 = testutil.GetMockHttpPoster()
)

func init() {
	testDB = testutil.InitDatabase(databaseConfig)

	app, err := NewApp(testDB, mockLightningClient,
		mockSendGridClient, mockBitcoindClient,
		mockHttpPoster, conf)
	if err != nil {
		panic(err.Error())
	}

	h = httptestutil.NewTestHarness(app.Router)
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

func TestGetTransactionByID(t *testing.T) {
	token, _ := h.CreateAndAuthenticateUser(t, users.CreateUserArgs{
		Email:    gofakeit.Email(),
		Password: gofakeit.Password(true, true, true, true, true, 21),
	})

	ids := createFakeDeposits(t, 3, token)

	t.Run("can get transaction by ID", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        fmt.Sprintf("/transaction/%d", ids[0]),
			Method:      "GET",
		})

		var trans transactions.Transaction
		h.AssertResponseOKWithStruct(t, req, &trans)

		if trans.ID != ids[0] {
			testutil.FailMsgf(t, "id's not equal, expected %d got %d", ids[0], trans.ID)
		}
	})
	t.Run("getting transaction with wrong ID returns error", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			// createFakeTransaction will always return the transaction in ascending order
			// where the highest index is the highest index saved to the user. therefore we +1
			Path:   fmt.Sprintf("/transaction/%d", ids[len(ids)-1]+1),
			Method: "GET",
		})

		_, _ = h.AssertResponseNotOkWithCode(t, req, 404)
	})

}

func TestGetAllTransactions(t *testing.T) {
	token, _ := h.CreateAndAuthenticateUser(t, users.CreateUserArgs{
		Email:    gofakeit.Email(),
		Password: gofakeit.Password(true, true, true, true, true, 21),
	})
	createFakeDeposits(t, 10, token)

	t.Run("get transactions without query params should get all", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions",
			Method:      "GET",
		})

		assertGetsRightAmount(t, req, 10)
	})
	t.Run("get transactions with limit 10 should get 10", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions?limit=10",
			Method:      "GET",
		})

		assertGetsRightAmount(t, req, 10)
	})
	t.Run("get transactions with limit 5 should get 5", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions?limit=5",
			Method:      "GET",
		})

		assertGetsRightAmount(t, req, 5)
	})
	t.Run("get transactions with limit 0 should get all", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions?limit=0",
			Method:      "GET",
		})

		assertGetsRightAmount(t, req, 10)
	})
	t.Run("get /transactions with offset 10 should get 0", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions?offset=10",
			Method:      "GET",
		})

		assertGetsRightAmount(t, req, 0)
	})

	t.Run("get /transactions with offset 0 should get 10", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions?offset=0",
			Method:      "GET",
		})

		assertGetsRightAmount(t, req, 10)
	})

	t.Run("get /transactions with offset 5 should get 5", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions?offset=5",
			Method:      "GET",
		})

		assertGetsRightAmount(t, req, 5)
	})
	t.Run("get /transactions with offset 5 and limit 3 should get 3", func(t *testing.T) {
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

		var trans transactions.Transaction
		h.AssertResponseOKWithStruct(t, req, &trans)
		testutil.AssertNotEqual(t, trans.Description, nil)
		testutil.AssertEqual(t, *trans.Description, description)
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

		var trans transactions.Transaction
		h.AssertResponseOKWithStruct(t, req, &trans)
		testutil.AssertEqual(t, trans.Description, nil)

	})
}

func TestCreateInvoice(t *testing.T) {
	testutil.DescribeTest(t)

	randomMockClient := lntestutil.GetRandomLightningMockClient()
	app, _ := NewApp(testDB, randomMockClient, mockSendGridClient,
		mockBitcoindClient, mockHttpPoster, conf)
	otherH := httptestutil.NewTestHarness(app.Router)

	password := gofakeit.Password(true, true, true, true, true, 32)
	email := gofakeit.Email()
	accessToken, _ := otherH.CreateAndAuthenticateUser(t, users.CreateUserArgs{
		Email:    email,
		Password: password,
	})

	t.Run("Not create an invoice with non-positive amount ", func(t *testing.T) {
		testutil.DescribeTest(t)

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
		testutil.DescribeTest(t)

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
		testutil.AssertEqual(t, res["customerOrderId"], orderId)

		t.Run("create an invoice with the same order ID twice", func(t *testing.T) {
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
		testutil.DescribeTest(t)

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
		t.Parallel()
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
				} else if _, key, err := apikeys.New(testDB, user); err != nil {
					testutil.FatalMsg(t, err)
				} else {
					apiKey = key
				}

				if _, err := transactions.UpdateInvoiceStatus(*mockInvoice,
					testDB, mockHttpPoster); err != nil {
					testutil.FatalMsg(t, err)
				}

				checkPostSent := func() bool {
					return mockHttpPoster.GetSentPostRequests() == 1
				}

				// emails are sent in a go-routine, so can't assume they're sent fast
				// enough for test to pick up
				if err := async.Await(8,
					time.Millisecond*20, checkPostSent); err != nil {
					testutil.FatalMsg(t, err)
				}

				bodyBytes := mockHttpPoster.GetSentPostRequest(0)
				body := transactions.CallbackBody{}

				if err := json.Unmarshal(bodyBytes, &body); err != nil {
					testutil.FatalMsg(t, err)
				}
				hmac := hmac.New(sha256.New, apiKey.HashedKey)
				_, _ = hmac.Write([]byte(fmt.Sprintf("%d", body.Payment.ID)))

				sum := hmac.Sum(nil)
				testutil.AssertEqual(t, sum, body.Hash)
			})
	})
}

func TestRestServer_CreateApiKey(t *testing.T) {
	testutil.DescribeTest(t)

	password := gofakeit.Password(true, true, true, true, true, 32)
	accessToken, _ := h.CreateAndAuthenticateUser(t, users.CreateUserArgs{
		Email:    gofakeit.Email(),
		Password: password,
	})

	t.Run("create an API key", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/apikey",
			Method:      "POST",
		})
		json := h.AssertResponseOkWithJson(t, req)

		testutil.AssertMsg(t, json["key"] != "", "`key` was empty!")

		t.Run("creating a new key should yield a different one", func(t *testing.T) {

			req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
				AccessToken: accessToken,
				Path:        "/apikey",
				Method:      "POST",
			})
			newJson := h.AssertResponseOkWithJson(t, req)
			testutil.AssertNotEqual(t, json["key"], newJson["key"])
			testutil.AssertEqual(t, json["userId"], newJson["userId"])
			testutil.AssertNotEqual(t, json["userId"], nil)
		})
	})
}

func TestRestServer_GetAllPayments(t *testing.T) {
	t.Parallel()
	pass := gofakeit.Password(true, true, true, true, true, 32)
	user := userstestutil.CreateUserOrFailWithPassword(t, testDB, pass)
	accessToken, _ := h.AuthenticaticateUser(t, users.CreateUserArgs{
		Email:    user.Email,
		Password: pass,
	})

	t.Run("fail with bad query parameters", func(t *testing.T) {
		t.Run("string argument", func(t *testing.T) {
			req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
				AccessToken: accessToken,
				Path:        fmt.Sprintf("/payments?limit=foobar&offset=0"),
				Method:      "GET",
			})

			_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
		})
		t.Run("negative argument", func(t *testing.T) {
			req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
				AccessToken: accessToken,
				Path:        fmt.Sprintf("/payments?offset=-1"),
				Method:      "GET",
			})

			_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
		})
	})

	t.Run("succeed with query parameters", func(t *testing.T) {
		opts := transactions.NewPaymentOpts{
			UserID:    user.ID,
			AmountSat: 123,
		}
		_ = transactions.CreateNewPaymentOrFail(t, testDB, mockLightningClient, opts)
		_ = transactions.CreateNewPaymentOrFail(t, testDB, mockLightningClient, opts)
		_ = transactions.CreateNewPaymentOrFail(t, testDB, mockLightningClient, opts)
		_ = transactions.CreateNewPaymentOrFail(t, testDB, mockLightningClient, opts)
		_ = transactions.CreateNewPaymentOrFail(t, testDB, mockLightningClient, opts)
		_ = transactions.CreateNewPaymentOrFail(t, testDB, mockLightningClient, opts)
		const numPayments = 6

		const limit = 3
		const offset = 2
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        fmt.Sprintf("/payments?limit=%d&offset=%d", limit, offset),
			Method:      "GET",
		})

		res := h.AssertResponseOkWithJsonList(t, req)
		testutil.AssertMsgf(t, len(res) == numPayments-limit, "Unexpected number of payments: %d", len(res))

	})
}

func TestRestServer_WithdrawOnChain(t *testing.T) {
	t.Parallel()
	const balanceSats = 10000

	t.Run("regular withdrawal", func(t *testing.T) {
		t.Parallel()
		pass := gofakeit.Password(true, true, true, true, true, 32)
		user := userstestutil.CreateUserOrFailWithPassword(t, testDB, pass)
		userstestutil.IncreaseBalanceOrFail(t, testDB, user, balanceSats)
		const withdrawAmount = 1234

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
		}`, withdrawAmount, "bcrt1qvn9hnzlpgrvcmrusj6cfh6cvgppp2z8fqeuxmy"),
		})

		h.AssertResponseOk(t, req)

		balanceReq := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/user",
			Method:      "GET",
		})

		balanceRes := h.AssertResponseOkWithJson(t, balanceReq)
		testutil.AssertEqual(t, balanceRes["balance"], balanceSats-withdrawAmount)
	})

	t.Run("fail to withdraw too much", func(t *testing.T) {
		t.Parallel()
		pass := gofakeit.Password(true, true, true, true, true, 32)
		user := userstestutil.CreateUserOrFailWithPassword(t, testDB, pass)
		userstestutil.IncreaseBalanceOrFail(t, testDB, user, balanceSats)
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
		}`, balanceSats+1, "bcrt1qvn9hnzlpgrvcmrusj6cfh6cvgppp2z8fqeuxmy"),
		})

		_, err := h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
		testutil.AssertEqual(t, apierr.ErrBalanceTooLowForWithdrawal, err)

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

// below this point are util functions, not actual tests
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
	if len(trans) != expected {
		testutil.FailMsgf(t, "expected %d transactions, got %d", expected, len(trans))
	}
}
