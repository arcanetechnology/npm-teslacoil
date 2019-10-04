package apikeys

import (
	"flag"
	"os"
	"testing"

	"github.com/brianvoe/gofakeit"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"
	"gitlab.com/arcanecrypto/teslacoil/internal/users"
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
