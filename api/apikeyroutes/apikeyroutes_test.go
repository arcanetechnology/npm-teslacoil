package apikeyroutes_test

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/brianvoe/gofakeit"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/arcanecrypto/teslacoil/api"
	"gitlab.com/arcanecrypto/teslacoil/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/models/apikeys"
	"gitlab.com/arcanecrypto/teslacoil/models/users"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/httptestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/lntestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/mock"
	"gitlab.com/arcanecrypto/teslacoil/testutil/userstestutil"
)

var (
	testDB              *db.DB
	h                   httptestutil.TestHarness
	mockLightningClient = lntestutil.GetLightningMockClient()
	mockBitcoindClient  = bitcoind.GetBitcoinMockClient()
	mockHttpPoster      = testutil.GetMockHttpPoster()
	mockSendGridClient  = mock.GetMockSendGridClient()
	log                 = build.Log
	conf                = api.Config{
		LogLevel: logrus.InfoLevel,
		Network:  chaincfg.RegressionNetParams,
	}
)

func init() {
	gofakeit.Seed(0)
	dbConf := testutil.GetDatabaseConfig("routes")
	testDB = testutil.InitDatabase(dbConf)

	app, err := api.NewApp(testDB, mockLightningClient,
		mockSendGridClient, mockBitcoindClient,
		mockHttpPoster, conf)

	if err != nil {
		panic(err)
	}

	h = httptestutil.NewTestHarness(app.Router, testDB)
}

func TestGetByHash(t *testing.T) {
	t.Parallel()

	user := userstestutil.CreateUserOrFail(t, testDB)
	rawKey, key, err := apikeys.New(testDB, user.ID, apikeys.AllPermissions, "")
	require.NoError(t, err)
	log.WithFields(logrus.Fields{
		"hash":   hex.EncodeToString(key.HashedKey),
		"userId": user.ID,
	}).Info("Created API key")

	t.Run("find an existing key for the right user", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: rawKey.String(),
			Path:        fmt.Sprintf("/apikey/%x", key.HashedKey),
			Method:      "GET",
		})

		json := h.AssertResponseOkWithJson(t, req)
		assert.Equal(t, base64.StdEncoding.EncodeToString(key.HashedKey), json["hashedKey"])
	})

	t.Run("not find the same key without authenticating", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   fmt.Sprintf("/apikey/%x", key.HashedKey),
			Method: "GET",
		})

		// bad request because we're attaching to auth header
		_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})
	t.Run("not find the same key as another user", func(t *testing.T) {
		t.Parallel()
		accessToken, _ := h.CreateAndAuthenticateUser(t, users.CreateUserArgs{
			Email:    gofakeit.Email(),
			Password: gofakeit.Password(true, true, true, true, true, 32),
		})
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        fmt.Sprintf("/apikey/%x", rawKey.Bytes()),
			Method:      "GET",
		})

		_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusNotFound)
	})
}

func TestGetAllForUser(t *testing.T) {
	t.Parallel()

	// this call also creates a key
	user := userstestutil.CreateUserOrFail(t, testDB)
	rawKey, _, err := apikeys.New(testDB, user.ID, apikeys.AllPermissions)
	require.NoError(t, err)

	req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
		AccessToken: rawKey.String(),
		Path:        "/apikey/all",
		Method:      "GET",
	})

	list := h.AssertResponseOkWithJsonList(t, req)
	assert.Len(t, list, 2)
}

func TestCreateApiKey(t *testing.T) {
	t.Parallel()

	password := gofakeit.Password(true, true, true, true, true, 32)
	accessToken, _ := h.CreateAndAuthenticateUser(t, users.CreateUserArgs{
		Email:    gofakeit.Email(),
		Password: password,
	})

	t.Run("create an API key with description", func(t *testing.T) {
		t.Parallel()
		desc := gofakeit.Sentence(10)
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/apikey",
			Method:      "POST",
			Body: fmt.Sprintf(`{
				"readWallet": true,
				"description": %q
			}`, desc),
		})
		res := h.AssertResponseOkWithJson(t, req)
		assert.Equal(t, desc, res["description"])
	})

	t.Run("create an API key without description", func(t *testing.T) {
		perm := apikeys.RandomPermissionSet()
		bodyBytes, err := json.Marshal(perm)
		body := string(bodyBytes)
		require.NoError(t, err)
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/apikey",
			Method:      "POST",
			Body:        body,
		})
		res := h.AssertResponseOkWithJson(t, req)

		require.NotNil(t, res["key"])

		assert.Contains(t, res["key"], res["lastLetters"])
		assert.NotNil(t, res["hashedKey"])

		require.NotNil(t, res["createdAt"])

		const layout = "2006-01-02T15:04:05.000000Z"
		createdAt, err := time.Parse(layout, res["createdAt"].(string))
		require.NoError(t, err)
		assert.WithinDuration(t, time.Now(), createdAt, time.Second)

		assert.Equal(t, perm.ReadWallet, res["readWallet"])
		assert.Equal(t, perm.CreateInvoice, res["createInvoice"])
		assert.Equal(t, perm.SendTransaction, res["sendTransaction"])
		assert.Equal(t, perm.EditAccount, res["editAccount"])
		assert.Nil(t, res["description"])

		t.Run("creating a new key should yield a different one", func(t *testing.T) {

			req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
				AccessToken: accessToken,
				Path:        "/apikey",
				Method:      "POST",
				Body:        body,
			})
			newJson := h.AssertResponseOkWithJson(t, req)
			assert.NotEqual(t, res["key"], newJson["key"])
			assert.Equal(t, res["userId"], newJson["userId"])
			assert.NotNil(t, res["userId"])
		})
	})
}
