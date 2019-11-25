package transactions_test

import (
	"math"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/arcanecrypto/teslacoil/async"
	"gitlab.com/arcanecrypto/teslacoil/models/transactions"
	"gitlab.com/arcanecrypto/teslacoil/testutil/txtest"
	"gitlab.com/arcanecrypto/teslacoil/testutil/userstestutil"
)

func getOnchainWithTx(userId int) transactions.Onchain {
	tx := txtest.MockOnchain(userId)
	if tx.ReceivedMoneyAt == nil {
		return getOnchainWithTx(userId)
	}
	return tx
}

type counter int32

func (c *counter) increment() int32 {
	return atomic.AddInt32((*int32)(c), 1)
}

func createTxsForUser(t *testing.T, count, userId int) []transactions.Transaction {
	var wg sync.WaitGroup

	// we set a max number of goroutines, to not exhaust the connection count in Postgres
	const MaxGoroutines = 20
	goRoutineCount := int(math.Min(float64(MaxGoroutines), float64(count)))
	result := make(chan transactions.Transaction, count)

	// because each goroutine performs the check for workload completion before
	// they start working, they'll see that they still have work to do, before
	// the other goroutines have completed their current work. so we start the
	// counter at the amount of goroutines, this should make sure we produce
	// the expected number of transactions
	txCounter := counter(0)
	for i := 0; i < goRoutineCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for txCounter.increment() <= int32(count) {
				// sleep for a milli to get some variation in when TXs are created
				time.Sleep(time.Millisecond)
				if gofakeit.Bool() {
					tx := txtest.MockOffchain(userId)
					inserted, err := transactions.InsertOffchain(testDB, tx)
					require.NoError(t, err)
					result <- inserted.ToTransaction()
				} else {
					tx := getOnchainWithTx(userId)
					inserted, err := transactions.InsertOnchain(testDB, tx)
					require.NoError(t, err)
					result <- inserted.ToTransaction()
				}
			}
		}()
	}
	const timeout = time.Second * 3
	require.False(t, async.WaitTimeout(&wg, timeout), "WaitGroup timed out after %s", timeout)
	close(result)
	var txs []transactions.Transaction
	for tx := range result {
		txs = append(txs, tx)
	}
	return txs
}

type txListByAmount []transactions.Transaction

func (t txListByAmount) Len() int {
	return len(t)
}

func (t txListByAmount) Less(i, j int) bool {
	elemI := t[i]
	if elemI.AmountMilliSat == nil {
		return true
	}
	elemJ := t[j]
	if elemJ.AmountMilliSat == nil {
		return false
	}

	return *t[i].AmountMilliSat < *t[j].AmountMilliSat
}

func (t txListByAmount) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

type txListByDate []transactions.Transaction

func (t txListByDate) Len() int {
	return len(t)
}

func (t txListByDate) Less(i, j int) bool {
	elemI := t[i]
	elemJ := t[j]

	var dateI time.Time
	var dateJ time.Time
	if elemI.ReceivedMoneyAt != nil {
		dateI = *elemI.ReceivedMoneyAt
	} else {
		dateI = elemI.CreatedAt
	}

	if elemJ.ReceivedMoneyAt != nil {
		dateJ = *elemJ.ReceivedMoneyAt
	} else {
		dateJ = elemJ.CreatedAt
	}

	return dateI.Before(dateJ)
}

func (t txListByDate) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

var _ sort.Interface = txListByAmount{}
var _ sort.Interface = txListByDate{}

func TestGetAllTransactions(t *testing.T) {
	t.Parallel()
	user := userstestutil.CreateUserOrFail(t, testDB)
	count := gofakeit.Number(50, 500)
	allTxs := createTxsForUser(t, count, user.ID)

	// make sure we have at least one non-expired
	tx := txtest.MockOffchain(user.ID)
	const MinExpiry = 60 // one minute
	for tx.Status != transactions.Offchain_CREATED || tx.Expiry < MinExpiry {
		tx = txtest.MockOffchain(user.ID)
	}
	_, err := transactions.InsertOffchain(testDB, tx)
	count += 1
	require.NoError(t, err)

	// make sure we have at least one expired
	var createdAt time.Time
	for {
		tx = txtest.MockOffchain(user.ID)
		tx.CreatedAt = gofakeit.Date()
		createdAt = tx.CreatedAt
		if tx.IsExpired() {
			break
		}
	}
	inserted, err := transactions.InsertOffchain(testDB, tx)
	count += 1
	require.NoError(t, err)

	_, err = testDB.Exec(`UPDATE transactions SET created_at = $1 WHERE id = $2`, createdAt, inserted.ID)
	require.NoError(t, err)

	sortedByAmount := txListByAmount(allTxs)
	sort.Sort(sortedByAmount)

	sortedByDate := txListByDate(allTxs)
	sort.Sort(sortedByDate)

	t.Run("sort by ascending", func(t *testing.T) {
		t.Parallel()
		all, err := transactions.GetAllTransactions(testDB, user.ID, transactions.GetAllParams{
			Sort: transactions.SortAscending,
		})
		require.NoError(t, err)
		require.Len(t, all, count)
		first := all[0]
		last := all[len(all)-1]
		assert.True(t, last.CreatedAt.After(first.CreatedAt), "Last should be after first")
	})

	t.Run("sort by descending", func(t *testing.T) {
		t.Parallel()
		all, err := transactions.GetAllTransactions(testDB, user.ID, transactions.GetAllParams{
			Sort: transactions.SortDescending,
		})
		require.NoError(t, err)
		require.Len(t, all, count)
		first := all[0]
		last := all[len(all)-1]
		assert.True(t, first.CreatedAt.After(last.CreatedAt), "First should be after last")
	})

	t.Run("limit by minimum amount", func(t *testing.T) {
		t.Parallel()

		first := *sortedByAmount[0].AmountMilliSat
		last := *sortedByAmount[len(sortedByAmount)-1].AmountMilliSat
		min := first - last
		all, err := transactions.GetAllTransactions(testDB, user.ID, transactions.GetAllParams{
			MinMilliSats: &min,
		})
		require.NoError(t, err)
		require.NotEmpty(t, all)
		for _, elem := range all {
			require.NotNil(t, elem.AmountMilliSat)
			assert.Greater(t, *elem.AmountMilliSat, min)
		}
	})

	t.Run("limit by maximum amount", func(t *testing.T) {
		t.Parallel()

		first := *sortedByAmount[0].AmountMilliSat
		last := *sortedByAmount[len(sortedByAmount)-1].AmountMilliSat
		max := first - last
		all, err := transactions.GetAllTransactions(testDB, user.ID, transactions.GetAllParams{
			MaxMilliSats: &max,
		})
		require.NoError(t, err)
		require.NotEmpty(t, all, "max %d", max)
		for _, elem := range all {
			require.NotNil(t, elem.AmountMilliSat)
			assert.Greater(t, max, *elem.AmountMilliSat)
		}
	})

	t.Run("limit by max and min amount", func(t *testing.T) {
		t.Parallel()

		first := *sortedByAmount[0].AmountMilliSat
		last := *sortedByAmount[len(sortedByAmount)-1].AmountMilliSat
		max := first - last
		min := rand.Int63n(max+last) - first
		all, err := transactions.GetAllTransactions(testDB, user.ID, transactions.GetAllParams{
			MaxMilliSats: &max,
			MinMilliSats: &min,
		})
		require.NoError(t, err)
		require.NotEmpty(t, all)
		for _, elem := range all {
			require.NotNil(t, elem.AmountMilliSat)
			assert.Greater(t, max, *elem.AmountMilliSat)
			assert.Less(t, min, *elem.AmountMilliSat)
		}
	})

	t.Run("limit by before", func(t *testing.T) {
		t.Parallel()
		middleElem := sortedByDate[len(sortedByDate)/2]
		before := middleElem.CreatedAt
		if middleElem.ReceivedMoneyAt != nil {
			before = *middleElem.ReceivedMoneyAt
		}
		all, err := transactions.GetAllTransactions(testDB, user.ID, transactions.GetAllParams{
			End: &before,
		})
		require.NoError(t, err)
		require.NotEmpty(t, all, "before %s", before)

		for _, elem := range all {
			if elem.ReceivedMoneyAt != nil {
				assert.True(t, elem.ReceivedMoneyAt.Before(before), "elem.ReceivedMoneyAt (%s) should be before %s", elem.CreatedAt, before)
			} else {
				assert.True(t, elem.CreatedAt.Before(before), "elem.CreatedAt (%s) should be before %s", elem.CreatedAt, before)
			}
		}
	})

	t.Run("limit by after", func(t *testing.T) {
		t.Parallel()

		middleElem := sortedByDate[len(sortedByDate)/2]
		after := middleElem.CreatedAt
		if middleElem.ReceivedMoneyAt != nil {
			after = *middleElem.ReceivedMoneyAt
		}
		all, err := transactions.GetAllTransactions(testDB, user.ID, transactions.GetAllParams{
			Start: &after,
		})
		require.NoError(t, err)
		require.NotEmpty(t, all, "after %s", after)

		for _, elem := range all {
			if elem.ReceivedMoneyAt != nil {
				assert.True(t, elem.ReceivedMoneyAt.After(after), "elem.ReceivedMoneyAt (%s) should be after %s", elem.CreatedAt, after)
			} else {
				assert.True(t, elem.CreatedAt.After(after), "elem.CreatedAt (%s) should be after %s", elem.CreatedAt, after)
			}
		}
	})

	t.Run("limit by before and after", func(t *testing.T) {
		t.Parallel()
		middleElem := sortedByDate[len(sortedByDate)/4]
		var after = middleElem.CreatedAt
		if middleElem.ReceivedMoneyAt != nil {
			after = *middleElem.ReceivedMoneyAt
		}
		before := after.Add(10 * 24 * 365 * time.Hour) // ten years

		all, err := transactions.GetAllTransactions(testDB, user.ID, transactions.GetAllParams{
			End:   &before,
			Start: &after,
		})
		require.NoError(t, err)
		require.NotEmpty(t, all, "before %s after %s", before, after)

		for _, elem := range all {
			if elem.ReceivedMoneyAt != nil {
				assert.True(t, elem.ReceivedMoneyAt.After(after))
				assert.True(t, elem.ReceivedMoneyAt.Before(before))
			} else {
				assert.True(t, elem.CreatedAt.After(after))
				assert.True(t, elem.CreatedAt.Before(before))
			}
		}
	})

	t.Run("only get incoming", func(t *testing.T) {
		t.Parallel()
		inbound := transactions.INBOUND
		all, err := transactions.GetAllTransactions(testDB, user.ID, transactions.GetAllParams{
			Direction: &inbound,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, all)

		for _, elem := range all {
			assert.Equal(t, transactions.INBOUND, elem.Direction)
		}
	})

	t.Run("only get outgoing", func(t *testing.T) {
		t.Parallel()
		outbound := transactions.OUTBOUND
		all, err := transactions.GetAllTransactions(testDB, user.ID, transactions.GetAllParams{
			Direction: &outbound,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, all)

		for _, elem := range all {
			assert.Equal(t, transactions.OUTBOUND, elem.Direction)
		}
	})

	t.Run("only get non-expired", func(t *testing.T) {
		t.Parallel()

		f := false
		all, err := transactions.GetAllTransactions(testDB, user.ID, transactions.GetAllParams{
			Expired: &f,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, all)

		for _, elem := range all {
			assert.False(t, elem.IsExpired())
		}
	})

	t.Run("only get expired", func(t *testing.T) {
		t.Parallel()

		tr := true
		all, err := transactions.GetAllTransactions(testDB, user.ID, transactions.GetAllParams{
			Expired: &tr,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, all)

		for _, elem := range all {
			assert.True(t, elem.IsExpired())
		}
	})
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
		createTxsForUser(t, count, user.ID)

		found, err := transactions.CountForUser(testDB, user.ID)
		require.NoError(t, err)
		assert.Equal(t, count, found)
	})
}
