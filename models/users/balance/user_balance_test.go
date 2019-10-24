package balance

import (
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	t.Parallel()
	for i := 0; i < 20; i++ {
		t.Run("balance.ForUser no."+strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			user := userstestutil.CreateUserOrFail(t, testDB)

			var expectedBalance Balance
			preTxGeneration, err := ForUser(testDB, user.ID)
			require.NoError(t, err)
			assert.Equal(t, expectedBalance, preTxGeneration)

			tx := transactiontestutil.InsertFakeOnChainOrFail(t, testDB, user.ID)
			if tx.ConfirmedAt != nil || tx.SettledAt != nil {
				expectedBalance += NewBalanceFromSats(tx.AmountSat)
			}
			tx = transactiontestutil.InsertFakeOnChainOrFail(t, testDB, user.ID)
			if tx.ConfirmedAt != nil || tx.SettledAt != nil {
				expectedBalance += NewBalanceFromSats(tx.AmountSat)
			}
			tx = transactiontestutil.InsertFakeOnChainOrFail(t, testDB, user.ID)
			if tx.ConfirmedAt != nil || tx.SettledAt != nil {
				expectedBalance += NewBalanceFromSats(tx.AmountSat)
			}

			postOnchain, err := ForUser(testDB, user.ID)
			require.NoError(t, err)
			assert.Equal(t, postOnchain, expectedBalance)

			offchain := transactiontestutil.InsertFakeOffChainOrFail(t, testDB, user.ID)
			if offchain.SettledAt != nil {
				expectedBalance += Balance(offchain.AmountMSat)
			}
			offchain = transactiontestutil.InsertFakeOffChainOrFail(t, testDB, user.ID)
			if offchain.SettledAt != nil {
				expectedBalance += Balance(offchain.AmountMSat)
			}
			offchain = transactiontestutil.InsertFakeOffChainOrFail(t, testDB, user.ID)
			if offchain.SettledAt != nil {
				expectedBalance += Balance(offchain.AmountMSat)
			}

			postOffchain, err := ForUser(testDB, user.ID)
			require.NoError(t, err)
			assert.Equal(t, postOffchain, expectedBalance)
		})
	}
}

func TestBalanceConversions(t *testing.T) {

	t.Run("NewBalanceFromSats() converts to right amount", func(t *testing.T) {
		expected := Balance(1000)
		actual := NewBalanceFromSats(1)
		assert.Equal(t, expected, actual)
	})

	t.Run("MilliSats() converts to right amount", func(t *testing.T) {
		actual := Balance(1000).MilliSats()
		assert.Equal(t, actual, 1000)
	})

	t.Run("Sats() converts to right amount", func(t *testing.T) {
		actual := Balance(1000).Sats()
		assert.Equal(t, actual, 1)
	})

	t.Run("Bitcoins() converts to right amount", func(t *testing.T) {
		expected := float64(1)
		actual := Balance(100000000000).Bitcoins() // 1 whole bitcoin, 100 000 million milliSats
		assert.Equal(t, actual, expected)
	})
}
