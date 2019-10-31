package apikeys_test

import (
	"flag"
	"os"
	"testing"

	"github.com/brianvoe/gofakeit"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/models/apikeys"
	"gitlab.com/arcanecrypto/teslacoil/models/users"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/userstestutil"
)

var (
	databaseConfig = testutil.GetDatabaseConfig("api_keys")
	testDB         *db.DB
)

func TestMain(m *testing.M) {
	build.SetLogLevel(logrus.ErrorLevel)

	testDB = testutil.InitDatabase(databaseConfig)

	flag.Parse()
	result := m.Run()

	os.Exit(result)
}

func TestNew(t *testing.T) {
	t.Run("creating an api key should work", func(t *testing.T) {
		user := userstestutil.CreateUserOrFail(t, testDB)
		rawKey, key, err := apikeys.New(testDB, user.ID)
		if err != nil {
			testutil.FatalMsg(t, err)
		}

		found, err := apikeys.Get(testDB, rawKey)
		if err != nil {
			testutil.FatalMsg(t, err)
		}

		testutil.AssertEqual(t, key, found)
	})

	t.Run("creating an api key with no related user should not work", func(t *testing.T) {
		user := users.User{
			ID: 123798123,
		}

		_, _, err := apikeys.New(testDB, user.ID)
		if err == nil {
			testutil.FatalMsg(t, "Created an API key with no corresponding user")
		}
	})

	t.Run("getting an non-existing key should not work", func(t *testing.T) {
		_, err := apikeys.Get(testDB, uuid.Must(uuid.FromString(gofakeit.UUID())))
		if err == nil {
			testutil.FatalMsg(t, "Was able to find non existant key!")
		}
	})
}

func TestGetByUserId(t *testing.T) {
	user, err := users.Create(testDB, users.CreateUserArgs{
		Email:    gofakeit.Email(),
		Password: gofakeit.Password(true, true, true, true, true, 32),
	})
	require.NoError(t, err)

	zero, err := apikeys.GetByUserId(testDB, user.ID)
	require.NoError(t, err)
	assert.Len(t, zero, 0)

	_, _, err = apikeys.New(testDB, user.ID)
	require.NoError(t, err)

	one, err := apikeys.GetByUserId(testDB, user.ID)
	require.NoError(t, err)
	assert.Len(t, one, 1)

	two, err := apikeys.GetByUserId(testDB, user.ID)
	require.NoError(t, err)
	assert.Len(t, two, 1)

	t.Run("Get empty list of keys for new user", func(t *testing.T) {
		otherUser, err := users.Create(testDB, users.CreateUserArgs{
			Email:    gofakeit.Email(),
			Password: gofakeit.Password(true, true, true, true, true, 32),
		})
		require.NoError(t, err)

		keys, err := apikeys.GetByUserId(testDB, otherUser.ID)
		require.NoError(t, err)
		assert.Len(t, keys, 0)
	})
}
