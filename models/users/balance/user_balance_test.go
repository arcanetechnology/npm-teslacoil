package balance

import (
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/transactiontestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/userstestutil"
)

var (
	databaseConfig = testutil.GetDatabaseConfig("users")
	testDB         *db.DB
	log            = build.Log
)

func TestMain(m *testing.M) {
	build.SetLogLevel(logrus.ErrorLevel)

	rand.Seed(time.Now().UnixNano())

	testDB = testutil.InitDatabase(databaseConfig)

	result := m.Run()

	if err := testDB.Close(); err != nil {
		panic(err.Error())
	}

	os.Exit(result)
}

func TestForUser(t *testing.T) {
	user := userstestutil.CreateUserOrFail(t, testDB)
	var expectedBalance int64

	tx := transactiontestutil.InsertFakeOnChainOrFail(t, testDB, user.ID)
	expectedBalance += tx.AmountSat
	tx = transactiontestutil.InsertFakeOnChainOrFail(t, testDB, user.ID)
	expectedBalance += tx.AmountSat
	tx = transactiontestutil.InsertFakeOnChainOrFail(t, testDB, user.ID)
	expectedBalance += tx.AmountSat

	balance, err := ForUser(testDB, user.ID)
	if err != nil {
		log.WithError(err).Error("could not calculate balance")
	}

	if balance != NewBalanceFromSats(int(expectedBalance)) {
		testutil.FailMsgf(t, "expected balance to be %q, was %q", expectedBalance, balance)
	}
}

func TestBalanceConvertions(t *testing.T) {

	t.Run("NewBalanceFromSats() converts to right amount", func(t *testing.T) {
		expected := Balance(1000)
		actual := NewBalanceFromSats(1)

		if expected != actual {
			testutil.FailMsgf(t, "expected %q got %q", expected, actual)
		}
	})

	t.Run("MilliSats() converts to right amount", func(t *testing.T) {
		expected := 1000
		actual := Balance(1000).MilliSats()

		if expected != actual {
			testutil.FailMsgf(t, "expected %q got %q", expected, actual)
		}
	})

	t.Run("Sats() converts to right amount", func(t *testing.T) {
		expected := 1
		actual := Balance(1000).Sats()

		if expected != actual {
			testutil.FailMsgf(t, "expected %q got %q", expected, actual)
		}
	})

	t.Run("Bitcoins() converts to right amount", func(t *testing.T) {
		expected := float64(1)
		actual := Balance(100000000000).Bitcoins() // 1 whole bitcoin, 100 000 million milliSats

		if expected != actual {
			testutil.FailMsgf(t, "expected %f got %f", expected, actual)
		}
	})
}
