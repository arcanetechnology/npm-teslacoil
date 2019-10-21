package apikeys

import (
	"flag"
	"os"
	"testing"

	"github.com/brianvoe/gofakeit"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/db"
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
		rawKey, key, err := New(testDB, user)
		if err != nil {
			testutil.FatalMsg(t, err)
		}

		found, err := Get(testDB, rawKey)
		if err != nil {
			testutil.FatalMsg(t, err)
		}

		testutil.AssertEqual(t, key, found)
	})

	t.Run("creating an api key with no related user should not work", func(t *testing.T) {
		user := users.User{
			ID: 123798123,
		}

		_, _, err := New(testDB, user)
		if err == nil {
			testutil.FatalMsg(t, "Created an API key with no corresponding user")
		}
	})

	t.Run("getting an non-existing key should not work", func(t *testing.T) {
		_, err := Get(testDB, uuid.Must(uuid.FromString(gofakeit.UUID())))
		if err == nil {
			testutil.FatalMsg(t, "Was able to find non existant key!")
		}
	})
}

func TestGetByUserId(t *testing.T) {
	user := userstestutil.CreateUserOrFail(t, testDB)

	zero, err := GetByUserId(testDB, user.ID)
	if err != nil {
		testutil.FatalMsg(t, err)
	}
	testutil.AssertMsgf(t, len(zero) == 0, "Got unexpected key list length %d", len(zero))

	if _, _, err := New(testDB, user); err != nil {
		testutil.FatalMsg(t, err)
	}

	one, err := GetByUserId(testDB, user.ID)
	if err != nil {
		testutil.FatalMsg(t, err)
	}
	testutil.AssertMsgf(t, len(one) == 1, "Got unexpected key list length %d", len(one))

	two, err := GetByUserId(testDB, user.ID)
	if err != nil {
		testutil.FatalMsg(t, err)
	}
	testutil.AssertMsgf(t, len(two) == 1, "Got unexpected key list length %d", len(two))

	t.Run("Get empty list of keys for new user", func(t *testing.T) {
		otherUser := userstestutil.CreateUserOrFail(t, testDB)
		keys, err := GetByUserId(testDB, otherUser.ID)
		if err != nil {
			testutil.FatalMsg(t, err)
		}
		testutil.AssertMsgf(t, len(keys) == 0, "Got keys for user: %v", keys)
	})
}
