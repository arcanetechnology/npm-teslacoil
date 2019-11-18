package transactions_test

import (
	"sync"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit"
	"github.com/stretchr/testify/require"
	"gitlab.com/arcanecrypto/teslacoil/async"
	"gitlab.com/arcanecrypto/teslacoil/models/transactions"
	"gitlab.com/arcanecrypto/teslacoil/testutil/txtest"
	"gitlab.com/arcanecrypto/teslacoil/testutil/userstestutil"
	"gotest.tools/assert"
)

func getOnchainWithTx(userId int) transactions.Onchain {
	tx := txtest.MockOnchain(userId)
	if tx.ReceivedMoneyAt == nil {
		return getOnchainWithTx(userId)
	}
	return tx
}

func Test_CountForUser(t *testing.T) {
	t.Parallel()
	t.Run("zero for non-existant user", func(t *testing.T) {
		count, err := transactions.CountForUser(testDB, 1238776)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("count TXs", func(t *testing.T) {
		user := userstestutil.CreateUserOrFail(t, testDB)
		count := gofakeit.Number(1, 20)
		var wg sync.WaitGroup
		for i := 0; i < count; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if gofakeit.Bool() {
					tx := txtest.MockOffchain(user.ID)
					_, err := transactions.InsertOffchain(testDB, tx)
					require.NoError(t, err)
				} else {
					tx := getOnchainWithTx(user.ID)
					_, err := transactions.InsertOnchain(testDB, tx)
					require.NoError(t, err)
				}
			}()
		}
		const timeout = time.Second * 3
		require.False(t, async.WaitTimeout(&wg, timeout), "WaitGroup timed out after %s", timeout)

		found, err := transactions.CountForUser(testDB, user.ID)
		require.NoError(t, err)
		assert.Equal(t, count, found)
	})
}
