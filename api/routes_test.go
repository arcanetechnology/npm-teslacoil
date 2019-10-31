package api

import (
	"testing"

	"github.com/brianvoe/gofakeit"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/models/users"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/httptestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/lntestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/mock"
)

var (
	testDB              *db.DB
	h                   httptestutil.TestHarness
	mockLightningClient = lntestutil.GetLightningMockClient()
	mockBitcoindClient  = bitcoind.GetBitcoinMockClient()
	mockHttpPoster      = testutil.GetMockHttpPoster()
	mockSendGridClient  = mock.GetMockSendGridClient()
	conf                = Config{
		LogLevel: logrus.InfoLevel,
		Network:  chaincfg.RegressionNetParams,
	}
)

func init() {
	dbConf := testutil.GetDatabaseConfig("routes")
	testDB = testutil.InitDatabase(dbConf)

	app, err := NewApp(testDB, mockLightningClient,
		mockSendGridClient, mockBitcoindClient,
		mockHttpPoster, conf)

	if err != nil {
		panic(err)
	}

	h = httptestutil.NewTestHarness(app.Router, testDB)
}

func TestRestServer_CreateApiKey(t *testing.T) {

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
