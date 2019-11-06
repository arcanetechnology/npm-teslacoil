package auth

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/models/apikeys"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/userstestutil"
)

var (
	dbConfig = testutil.GetDatabaseConfig("auth")
	testDB   *db.DB

	wrongJwtPrivKey   *rsa.PrivateKey
	correctJwtPrivKey *rsa.PrivateKey
)

func TestMain(m *testing.M) {
	gofakeit.Seed(0)
	reader := rand.Reader

	var err error
	wrongJwtPrivKey, err = rsa.GenerateKey(reader, 5093)
	if err != nil {
		panic(err)
	}

	correctJwtPrivKey, err = rsa.GenerateKey(reader, 5093)
	if err != nil {
		panic(err)
	}

	SetJwtPrivateKey(correctJwtPrivKey)

	testDB = testutil.InitDatabase(dbConfig)
	gofakeit.Seed(0)
	os.Exit(m.Run())
}

func TestCreateJWT(t *testing.T) {
	t.Parallel()
	email := gofakeit.Email()
	id := gofakeit.Number(
		0,
		1000000,
	)
	token, err := CreateJwt(email, id)
	if err != nil {
		testutil.FatalMsg(t, err)
	}

	parsed, claims, err := parseBearerJwt(token)
	if err != nil {
		testutil.FatalMsg(t, err)
	}
	assert.True(t, parsed.Valid, "Token was invalid")
	assert.Equal(t, claims.UserID, id)
	assert.Equal(t, claims.Email, email)
}

func TestParseBearerJwt(t *testing.T) {
	t.Parallel()
	email := gofakeit.Email()
	id := gofakeit.Number(
		0,
		1000000,
	)

	t.Run("creating a JWT with a bad key should not parse", func(t *testing.T) {
		args := createJwtArgs{
			email:      email,
			id:         id,
			privateKey: wrongJwtPrivKey,
		}
		token, err := createJwt(args)
		require.NoError(t, err)
		_, _, err = parseBearerJwt(token)
		require.NotNil(t, err)
		assert.Equal(t, err.Error(), rsa.ErrVerification.Error())
	})

	t.Run("Parsing a JWT with a bad key should not work", func(t *testing.T) {
		token, err := CreateJwt(email, id)
		if err != nil {
			testutil.FatalMsg(t, err)
		}

		_, _, err = parseBearerJwtWithKey(token, &wrongJwtPrivKey.PublicKey)
		testutil.AssertEqual(t, err, rsa.ErrVerification)
	})
}

func setupRouter(middleware gin.HandlerFunc) *gin.Engine {
	r := gin.Default()
	r.Use(middleware)
	r.GET("/ping", func(c *gin.Context) {
		c.Status(200)
	})
	r.GET("/scope-test", func(c *gin.Context) {
		_, ok := RequireScope(c, ReadWallet)
		if !ok {
			return
		}
		c.Status(200)
	})
	return r
}

func TestGetMiddleware(t *testing.T) {
	t.Parallel()
	middleware := GetMiddleware(testDB)
	router := setupRouter(middleware)
	emptyBody := bytes.NewBuffer([]byte(""))

	user := userstestutil.CreateUserOrFail(t, testDB)
	t.Run("authenticate with JWT", func(t *testing.T) {
		token, err := CreateJwt(user.Email, user.ID)
		if err != nil {
			testutil.FatalMsg(t, err)
		}
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ping", emptyBody)
		req.Header.Add(Header, token)
		router.ServeHTTP(w, req)
		testutil.AssertEqual(t, w.Code, http.StatusOK)
	})

	t.Run("not authenticate with JWT from bad key", func(t *testing.T) {
		args := createJwtArgs{
			email:      user.Email,
			id:         user.ID,
			privateKey: wrongJwtPrivKey,
		}
		token, err := createJwt(args)
		if err != nil {
			testutil.FatalMsg(t, err)
		}
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ping", emptyBody)
		req.Header.Add(Header, token)
		router.ServeHTTP(w, req)
		testutil.AssertEqual(t, w.Code, http.StatusUnauthorized)
	})

	t.Run("not authenticate with malformed JWT", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ping", emptyBody)
		req.Header.Add(Header, "Bearer foobar")
		router.ServeHTTP(w, req)
		testutil.AssertEqual(t, w.Code, http.StatusBadRequest)
	})

	t.Run("not authenticate with expired JWT", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ping", emptyBody)
		token, err := createJwt(createJwtArgs{
			email:      user.Email,
			id:         user.ID,
			privateKey: correctJwtPrivKey,
			now: func() time.Time {
				return time.Now().Add(-24 * time.Hour)
			},
		})
		if err != nil {
			testutil.FatalMsg(t, err)
		}
		req.Header.Add(Header, token)
		router.ServeHTTP(w, req)
		assert.Equal(t, w.Code, http.StatusUnauthorized)

	})

	t.Run("not authenticate with JWT issued in the future", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ping", emptyBody)
		token, err := createJwt(createJwtArgs{
			email:      user.Email,
			id:         user.ID,
			privateKey: correctJwtPrivKey,
			now: func() time.Time {
				return time.Now().Add(24 * time.Hour)
			},
		})
		if err != nil {
			testutil.FatalMsg(t, err)
		}
		req.Header.Add(Header, token)
		router.ServeHTTP(w, req)
		assert.Equal(t, w.Code, http.StatusUnauthorized)

	})

	t.Run("authentiate with API key", func(t *testing.T) {
		apiKey, _, err := apikeys.New(testDB, user.ID, apikeys.AllPermissions, "")
		if err != nil {
			testutil.FatalMsg(t, err)
		}
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ping", emptyBody)
		req.Header.Add(Header, apiKey.String())
		router.ServeHTTP(w, req)
		assert.Equal(t, w.Code, http.StatusOK)
	})

	t.Run("not authentiate with non-existant key", func(t *testing.T) {
		key := gofakeit.UUID()
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ping", emptyBody)
		req.Header.Add(Header, key)
		router.ServeHTTP(w, req)
		assert.Equal(t, w.Code, http.StatusUnauthorized)
	})

	t.Run("not authenticate with malformed key", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ping", emptyBody)
		req.Header.Add(Header, "api-key-in-here")
		router.ServeHTTP(w, req)
		assert.Equal(t, w.Code, http.StatusBadRequest)
	})
	t.Run("authenticate with scopes", func(t *testing.T) {
		key, _, err := apikeys.New(testDB, user.ID, apikeys.Permissions{
			CreateInvoice:   true,
			SendTransaction: true,
			EditAccount:     true,
		}, "")
		require.NoError(t, err)
		badW := httptest.NewRecorder()
		badReq, _ := http.NewRequest("GET", "/scope-test", emptyBody)
		badReq.Header.Add(Header, key.String())
		router.ServeHTTP(badW, badReq)
		assert.Equal(t, http.StatusUnauthorized, badW.Code)

		otherKey, _, err := apikeys.New(testDB, user.ID, apikeys.Permissions{
			ReadWallet: true,
		}, "")
		require.NoError(t, err)
		goodW := httptest.NewRecorder()
		goodReq, _ := http.NewRequest("GET", "/scope-test", emptyBody)
		goodReq.Header.Add(Header, otherKey.String())
		router.ServeHTTP(goodW, goodReq)
		assert.Equal(t, http.StatusOK, goodW.Code)

	})
}
