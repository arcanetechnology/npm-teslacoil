package balance_test

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
	"gitlab.com/arcanecrypto/teslacoil/models/transactions"
	"gitlab.com/arcanecrypto/teslacoil/models/users/balance"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/transactiontestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/userstestutil"
)

var (
	databaseConfig = testutil.GetDatabaseConfig("balance")
	testDB         *db.DB
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

func TestIncomingForUser(t *testing.T) {
	t.Parallel()

	calcNewBalance := func(t *testing.T, tx transactions.Transaction) balance.Balance {
		if off, err := tx.ToOffchain(); err == nil {
			if off.SettledAt != nil && off.Direction == transactions.INBOUND && off.Status == transactions.SUCCEEDED {
				return balance.Balance(off.AmountMSat)
			}
			return balance.Balance(0)
		}
		if on, err := tx.ToOnchain(); err == nil {
			if on.ConfirmedAt != nil || on.SettledAt != nil && on.Direction == transactions.INBOUND {
				require.NotNil(t, on.AmountSat)
				return balance.NewBalanceFromSats(*on.AmountSat)
			}
			return balance.Balance(0)
		}
		t.Fatal("Neither offchain nor onchain ", tx)
		return balance.Balance(0)
	}

	for i := 0; i < 20; i++ {
		t.Run("should get balance for a single onchain TX no."+strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			user := userstestutil.CreateUserOrFail(t, testDB)
			var expected balance.Balance
			tx := transactiontestutil.InsertFakeIncomingOnchainorFail(t, testDB, user.ID)
			expected += calcNewBalance(t, tx.ToTransaction())

			bal, err := balance.IncomingForUser(testDB, user.ID)
			require.NoError(t, err)
			assert.Equal(t, expected, bal)
		})

		t.Run("should get balance for a single offchain TX no."+strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			user := userstestutil.CreateUserOrFail(t, testDB)
			var expected balance.Balance
			tx := transactiontestutil.InsertFakeOffChainOrFail(t, testDB, user.ID)
			expected += calcNewBalance(t, tx.ToTransaction())

			bal, err := balance.IncomingForUser(testDB, user.ID)
			require.NoError(t, err)
			assert.Equal(t, expected, bal, tx)
		})

		t.Run("balance.IncomingForUser no."+strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			user := userstestutil.CreateUserOrFail(t, testDB)

			var expectedBalance balance.Balance
			preTxGeneration, err := balance.IncomingForUser(testDB, user.ID)
			require.NoError(t, err)
			assert.Equal(t, expectedBalance, preTxGeneration)

			tx := transactiontestutil.InsertFakeIncomingOnchainorFail(t, testDB, user.ID)
			expectedBalance += calcNewBalance(t, tx.ToTransaction())

			tx = transactiontestutil.InsertFakeIncomingOnchainorFail(t, testDB, user.ID)
			expectedBalance += calcNewBalance(t, tx.ToTransaction())

			tx = transactiontestutil.InsertFakeIncomingOnchainorFail(t, testDB, user.ID)
			expectedBalance += calcNewBalance(t, tx.ToTransaction())

			postOnchain, err := balance.IncomingForUser(testDB, user.ID)
			require.NoError(t, err)
			assert.Equal(t, postOnchain, expectedBalance)

			offchain := transactiontestutil.InsertFakeIncomingOffchainOrFail(t, testDB, user.ID)
			expectedBalance += calcNewBalance(t, offchain.ToTransaction())

			offchain = transactiontestutil.InsertFakeIncomingOffchainOrFail(t, testDB, user.ID)
			expectedBalance += calcNewBalance(t, offchain.ToTransaction())

			offchain = transactiontestutil.InsertFakeIncomingOffchainOrFail(t, testDB, user.ID)
			expectedBalance += calcNewBalance(t, offchain.ToTransaction())

			postOffchain, err := balance.IncomingForUser(testDB, user.ID)
			require.NoError(t, err)
			assert.Equal(t, postOffchain, expectedBalance)
		})
	}
}

func TestOutgoingForUser(t *testing.T) {
	t.Parallel()

	calcNewBalance := func(t *testing.T, tx transactions.Transaction) balance.Balance {
		if off, err := tx.ToOffchain(); err == nil {
			// we debit users for all offchain TXs, _unless_ the payments are explictly failed
			if off.Direction == transactions.OUTBOUND && off.Status != transactions.FAILED {
				return balance.Balance(off.AmountMSat)
			}
			return balance.Balance(0)
		}
		if on, err := tx.ToOnchain(); err == nil {
			// we debit users for all onchain TXs, full stop. This only applies to transactions with values attached, though
			if on.Direction == transactions.OUTBOUND && on.Txid != nil {
				require.NotNil(t, on.AmountSat)
				return balance.NewBalanceFromSats(*on.AmountSat)
			}
			return balance.Balance(0)
		}
		t.Fatal("Neither offchain nor onchain ", tx)
		return balance.Balance(0)
	}

	for i := 0; i < 20; i++ {
		t.Run("should get balance for a single onchain TX no."+strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			user := userstestutil.CreateUserOrFail(t, testDB)
			var expected balance.Balance
			tx := transactiontestutil.InsertFakeOutgoingOnchainorFail(t, testDB, user.ID)
			expected += calcNewBalance(t, tx.ToTransaction())

			bal, err := balance.OutgoingForUser(testDB, user.ID)
			require.NoError(t, err)
			assert.Equal(t, expected, bal, tx)
		})

		t.Run("should get balance for a single offchain TX no."+strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			user := userstestutil.CreateUserOrFail(t, testDB)
			var expected balance.Balance
			tx := transactiontestutil.InsertFakeOffChainOrFail(t, testDB, user.ID)
			expected += calcNewBalance(t, tx.ToTransaction())

			bal, err := balance.OutgoingForUser(testDB, user.ID)
			require.NoError(t, err)
			assert.Equal(t, expected, bal, tx)
		})

		t.Run("balance.OutgoingForUser no."+strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			user := userstestutil.CreateUserOrFail(t, testDB)

			var expectedBalance balance.Balance
			preTxGeneration, err := balance.OutgoingForUser(testDB, user.ID)
			require.NoError(t, err)
			assert.Equal(t, expectedBalance, preTxGeneration)

			tx := transactiontestutil.InsertFakeOutgoingOnchainorFail(t, testDB, user.ID)
			expectedBalance += calcNewBalance(t, tx.ToTransaction())

			tx = transactiontestutil.InsertFakeOutgoingOnchainorFail(t, testDB, user.ID)
			expectedBalance += calcNewBalance(t, tx.ToTransaction())

			tx = transactiontestutil.InsertFakeOutgoingOnchainorFail(t, testDB, user.ID)
			expectedBalance += calcNewBalance(t, tx.ToTransaction())

			postOnchain, err := balance.OutgoingForUser(testDB, user.ID)
			require.NoError(t, err)
			assert.Equal(t, expectedBalance, postOnchain)

			offchain := transactiontestutil.InsertFakeOutgoingOffchainOrFail(t, testDB, user.ID)
			expectedBalance += calcNewBalance(t, offchain.ToTransaction())

			offchain = transactiontestutil.InsertFakeOutgoingOffchainOrFail(t, testDB, user.ID)
			expectedBalance += calcNewBalance(t, offchain.ToTransaction())

			offchain = transactiontestutil.InsertFakeOutgoingOffchainOrFail(t, testDB, user.ID)
			expectedBalance += calcNewBalance(t, offchain.ToTransaction())

			postOffchain, err := balance.OutgoingForUser(testDB, user.ID)
			require.NoError(t, err)
			assert.Equal(t, expectedBalance, postOffchain)
		})
	}
}

func TestBalanceConversions(t *testing.T) {

	t.Run("NewBalanceFromSats() converts to right amount", func(t *testing.T) {
		expected := balance.Balance(1000)
		actual := balance.NewBalanceFromSats(1)
		assert.Equal(t, expected, actual)
	})

	t.Run("MilliSats() converts to right amount", func(t *testing.T) {
		actual := balance.Balance(1000).MilliSats()
		assert.Equal(t, actual, int64(1000))
	})

	t.Run("Sats() converts to right amount", func(t *testing.T) {
		actual := balance.Balance(1000).Sats()
		assert.Equal(t, actual, int64(1))
	})

	t.Run("Bitcoins() converts to right amount", func(t *testing.T) {
		expected := float64(1)
		actual := balance.Balance(100000000000).Bitcoins() // 1 whole bitcoin, 100 000 million milliSats
		assert.Equal(t, actual, expected)
	})
}
