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

func TestCalculateBalance(t *testing.T) {
	user := userstestutil.CreateUserOrFail(t, testDB)

	transactiontestutil.GenOnchain(user.ID)
	transactiontestutil.GenOnchain(user.ID)
	transactiontestutil.GenOnchain(user.ID)

	balance, err := CalculateBalance(testDB, user.ID)
	if err != nil {
		log.WithError(err).Error("could not calculate balance")
		panic("could not calculate balance")
	}

	testutil.Succeedf(t, "balance is %d", balance)
}
