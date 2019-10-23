package transactions

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/btcsuite/btcutil"
	"github.com/google/go-cmp/cmp"
	"gitlab.com/arcanecrypto/teslacoil/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/models/users"

	"github.com/brianvoe/gofakeit"
	"github.com/btcsuite/btcd/chaincfg/chainhash"

	"gitlab.com/arcanecrypto/teslacoil/ln"
	"gitlab.com/arcanecrypto/teslacoil/testutil/lntestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/userstestutil"

	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/db"

	"github.com/lightningnetwork/lnd/lnrpc"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
)

var (
	databaseConfig = testutil.GetDatabaseConfig("transactions")
	testDB         *db.DB
	testnetAddress = "tb1q40gzxjcamcny49st7m8lyz9rtmssjgfefc33at"
)

func TestMain(m *testing.M) {
	build.SetLogLevel(logrus.DebugLevel)

	testDB = testutil.InitDatabase(databaseConfig)

	result := m.Run()

	os.Exit(result)
}

func genPreimage() []byte {
	p := make([]byte, 32)
	_, _ = rand.Read(p)
	return p
}

func genTxid() string {
	p := make([]byte, 32)
	_, _ = rand.Read(p)
	return hex.EncodeToString(p)

}

func genStatus() Status {
	s := []Status{FAILED, OPEN, SUCCEEDED}
	return s[rand.Intn(3)]
}

func genMaybeString(fn func() string) *string {
	var res *string
	if gofakeit.Bool() {
		r := fn()
		res = &r
	}
	return res
}

func genDirection() Direction {
	direction := OUTBOUND
	if gofakeit.Bool() {
		direction = INBOUND
	}
	return direction
}

func int64Between(min, max int64) int64 {
	return rand.Int63n(max-min+1) + min
}

func genOffchain(user users.User) Offchain {
	amountMSat := rand.Int63n(ln.MaxAmountMsatPerInvoice)

	var preimage []byte
	var settledAt *time.Time
	var hashedPreimage []byte
	if gofakeit.Bool() {
		preimage = genPreimage()
		now := time.Now()
		start := now.Add(-(time.Hour * 24 * 60)) // 60 days ago
		s := gofakeit.DateRange(start, now)
		settledAt = &s
		h := sha256.Sum256(hashedPreimage)
		hashedPreimage = h[:]
	} else {
		hashedPreimage = genPreimage()
	}

	return Offchain{
		UserID:          user.ID,
		CallbackURL:     genMaybeString(gofakeit.URL),
		CustomerOrderId: genMaybeString(gofakeit.Word),
		Expiry:          gofakeit.Int64(),
		Direction:       genDirection(),
		AmountSat:       amountMSat / 1000,
		Description: genMaybeString(func() string {
			return gofakeit.Sentence(gofakeit.Number(0, 10))
		}),
		PaymentRequest: "DO ME LATER",
		Preimage:       preimage,
		HashedPreimage: hashedPreimage,
		AmountMSat:     amountMSat,
		SettledAt:      settledAt,
		Memo: genMaybeString(func() string {
			return gofakeit.Sentence(gofakeit.Number(0, 10))
		}),

		Status: genStatus(),
	}
}

func genOnchain(user users.User) Onchain {
	now := time.Now()
	start := now.Add(-(time.Hour * 24 * 60)) // 60 days ago

	var settledAt *time.Time
	if gofakeit.Bool() {
		s := gofakeit.DateRange(start, now)
		settledAt = &s
	}

	var confirmedAtBlock *int
	var confirmedAt *time.Time
	if gofakeit.Bool() {
		cA := gofakeit.DateRange(start, now)
		confirmedAt = &cA
		c := gofakeit.Number(1, 1000000)
		confirmedAtBlock = &c
	}

	var txid *string
	var vout *int
	if gofakeit.Bool() {
		t := genTxid() // do me later
		txid = &t
		v := gofakeit.Number(0, 12)
		vout = &v
	}

	return Onchain{
		UserID:          user.ID,
		CallbackURL:     genMaybeString(gofakeit.URL),
		CustomerOrderId: genMaybeString(gofakeit.Word),
		Expiry:          gofakeit.Int64(),
		Direction:       genDirection(),
		AmountSat:       int64Between(0, btcutil.MaxSatoshi),
		Description: genMaybeString(func() string {
			return gofakeit.Sentence(gofakeit.Number(0, 10))
		}),
		ConfirmedAtBlock: confirmedAtBlock,
		Address:          "DO ME LATER",
		Txid:             txid,
		Vout:             vout,
		ConfirmedAt:      confirmedAt,
		SettledAt:        settledAt,
	}
}

func TestInsertOnchainTransaction(t *testing.T) {
	t.Parallel()
	user := userstestutil.CreateUserOrFail(t, testDB)
	for i := 0; i < 20; i++ {
		t.Run("inserting arbitrary onchain "+strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			onchain := genOnchain(user)

			inserted, err := insertOnchain(testDB, onchain)
			if err != nil {
				testutil.FatalMsg(t, err)
			}

			onchain.CreatedAt = inserted.CreatedAt
			onchain.UpdatedAt = inserted.UpdatedAt
			if onchain.SettledAt != nil {
				if onchain.SettledAt.Sub(*inserted.SettledAt) > time.Millisecond*500 {
					testutil.AssertEqual(t, *onchain.SettledAt, *inserted.SettledAt)
				}
				onchain.SettledAt = inserted.SettledAt
			}

			if onchain.ConfirmedAt != nil {
				if onchain.ConfirmedAt.Sub(*inserted.ConfirmedAt) > time.Millisecond*500 {
					testutil.AssertEqual(t, *onchain.ConfirmedAt, *inserted.ConfirmedAt)
				}
				onchain.ConfirmedAt = inserted.ConfirmedAt
			}

			// ID should be created by DB for us
			testutil.AssertNotEqual(t, onchain.ID, inserted.ID)
			onchain.ID = inserted.ID
			diff := cmp.Diff(onchain, inserted)
			if diff != "" {
				testutil.FatalMsg(t, diff)
			}

			foundTx, err := GetTransactionByID(testDB, inserted.ID, user.ID)
			if err != nil {
				testutil.FatalMsg(t, err)
			}

			foundOnChain, err := foundTx.ToOnchain()
			if err != nil {
				testutil.FatalMsg(t, err)
			}

			diff = cmp.Diff(foundOnChain, inserted)
			if diff != "" {
				testutil.FatalMsg(t, diff)
			}

			allTXs, err := GetAllTransactions(testDB, user.ID)
			if err != nil {
				testutil.FatalMsg(t, err)
			}
			found := false
			for _, tx := range allTXs {
				on, err := tx.ToOnchain()
				if err != nil {
					break
				}
				if cmp.Diff(on, inserted) == "" {
					found = true
					break
				}
			}
			if !found {
				testutil.FatalMsg(t, "Did not find TX when doing GetAll")
			}
		})
	}
}

func TestInsertOffchainTransaction(t *testing.T) {
	t.Parallel()
	user := userstestutil.CreateUserOrFail(t, testDB)
	for i := 0; i < 20; i++ {
		t.Run("inserting arbitrary offchain "+strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			offchain := genOffchain(user)

			inserted, err := insertOffChain(testDB, offchain)
			if err != nil {
				testutil.FatalMsg(t, err)
			}

			offchain.CreatedAt = inserted.CreatedAt
			offchain.UpdatedAt = inserted.UpdatedAt

			if offchain.SettledAt != nil {
				if offchain.SettledAt.Sub(*inserted.SettledAt) > time.Millisecond*500 {
					testutil.AssertEqual(t, *offchain.SettledAt, *inserted.SettledAt)
				}
				offchain.SettledAt = inserted.SettledAt
			}

			// ID should be created by DB for us
			testutil.AssertNotEqual(t, offchain.ID, inserted.ID)
			offchain.ID = inserted.ID
			diff := cmp.Diff(offchain, inserted)
			if diff != "" {
				testutil.FatalMsg(t, diff)
			}

			foundTx, err := GetTransactionByID(testDB, inserted.ID, user.ID)
			if err != nil {
				testutil.FatalMsg(t, err)
			}

			foundOffChain, err := foundTx.ToOffChain()
			if err != nil {
				testutil.FatalMsg(t, err)
			}

			diff = cmp.Diff(foundOffChain, inserted)
			if diff != "" {
				testutil.FatalMsg(t, diff)
			}

			allTXs, err := GetAllTransactions(testDB, user.ID)
			if err != nil {
				testutil.FatalMsg(t, err)
			}
			found := false
			for _, tx := range allTXs {
				off, err := tx.ToOffChain()
				if err != nil {
					break
				}
				if cmp.Diff(off, inserted) == "" {
					found = true
					break
				}
			}
			if !found {
				testutil.FatalMsg(t, "Did not find TX when doing GetAll")
			}
		})

	}
}

func TestGetTransactionByID(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	const email1 = "email1@example.com"
	const password1 = "password1"
	const email2 = "email2@example.com"
	const password2 = "password2"
	// amount1 := rand.Int63n(ln.MaxAmountSatPerInvoice)
	// amount2 := rand.Int63n(ln.MaxAmountSatPerInvoice)

	user := userstestutil.CreateUserOrFail(t, testDB)

	foo := "foo"
	testCases := []struct {
		email          string
		password       string
		expectedResult Transaction
	}{
		{

			email1,
			password1,
			Transaction{
				UserID: user.ID,
				// AmountSat: amount1,
				// Address:     "bar",
				Description: &foo,
				Direction:   INBOUND,
			},
		},
		{

			email2,
			password2,
			Transaction{
				UserID: user.ID,
				// AmountSat: amount2,
				// Address:     "bar",
				Description: &foo,
				Direction:   INBOUND,
			},
		},
	}

	for _, test := range testCases {
		t.Run(fmt.Sprintf("GetTransactionByID() for transaction with amount %d", test.expectedResult.AmountMSat),
			func(t *testing.T) {

				transaction, err := insertTransaction(testDB, test.expectedResult)
				if err != nil {
					testutil.FatalMsgf(t, "should be able to insertTransaction. Error:  %+v",
						err)
				}

				// Act
				transaction, err = GetTransactionByID(testDB, transaction.ID, test.expectedResult.UserID)
				if err != nil {
					testutil.FatalMsgf(t, "should be able to GetByID. Error: %+v", err)
				}

				assertTransactionsAreEqual(t, transaction, test.expectedResult)
			})
	}
}

func TestGetAllOffset(t *testing.T) {
	testutil.DescribeTest(t)
}

func TestGetAllLimit(t *testing.T) {
	testutil.DescribeTest(t)
}

func TestWithdrawOnChain(t *testing.T) {
	t.Parallel()

	mockBitcoin := bitcoind.TeslacoilBitcoindMockClient{}

	t.Run("ignores amount and withdraws all the users balance", func(t *testing.T) {

		testCases := []struct {
			balance int
			// We specify amountSat to make sure it is ignored when sendAll is true
			amountSat int64
		}{
			{
				balance:   10000,
				amountSat: 500000,
			},
			{
				balance:   20000,
				amountSat: -500000,
			},
			{
				balance:   500, // 20 000
				amountSat: 0,
			},
		}

		for _, test := range testCases {

			mockLNcli := lntestutil.LightningMockClient{
				SendCoinsResponse: lnrpc.SendCoinsResponse{
					Txid: testutil.MockTxid(),
				},
			}

			user := userstestutil.CreateUserWithBalanceOrFail(t, testDB, test.balance)

			_, err := WithdrawOnChain(testDB, mockLNcli, mockBitcoin, WithdrawOnChainArgs{
				UserID:    user.ID,
				AmountSat: test.amountSat,
				Address:   testnetAddress,
				SendAll:   true,
			})
			if err != nil {
				testutil.FatalMsgf(t, "could not WithdrawOnChain %+v", err)
			}

			// TODO: Test this creates transactions for the right amount
			// t.Run("withdrew the right amount", func(t *testing.T) {
			// Look up the txid on-chain, and check the amount
			// fmt.Println("txid: ", txid)
			// })

			// Assert
			t.Run("users balance is 0", func(t *testing.T) {
				_, err := users.GetByID(testDB, user.ID)
				if err != nil {
					testutil.FatalMsgf(t, "could not get user %+v", err)
				}
				// if user.Balance != 0 {
				// 	testutil.FatalMsgf(t, "users balance was not 0 %+v", err)
				// }
			})
		}
	})

	t.Run("withdraw more than balance fails", func(t *testing.T) {
		user := userstestutil.CreateUserWithBalanceOrFail(t, testDB,
			500)
		mockLNcli := lntestutil.LightningMockClient{
			SendCoinsResponse: lnrpc.SendCoinsResponse{
				Txid: testutil.MockTxid(),
			},
		}

		transaction, err := WithdrawOnChain(testDB, mockLNcli, mockBitcoin, WithdrawOnChainArgs{
			UserID:    user.ID,
			AmountSat: 5000,
			Address:   testnetAddress,
		})

		if err == nil {
			testutil.FatalMsgf(t, "should return error and not send transaction: %+v", transaction)
		}
	})
	t.Run("withdraw negative amount fails", func(t *testing.T) {
		user := userstestutil.CreateUserWithBalanceOrFail(t, testDB, 500)
		mockLNcli := lntestutil.LightningMockClient{
			SendCoinsResponse: lnrpc.SendCoinsResponse{
				Txid: testutil.MockTxid(),
			},
		}

		transaction, err := WithdrawOnChain(testDB, mockLNcli, mockBitcoin, WithdrawOnChainArgs{
			UserID:    user.ID,
			AmountSat: -5000,
			Address:   testnetAddress,
		})

		if err == nil {
			testutil.FatalMsgf(t, "should return error and not send transaction: %+v", transaction)
		}
	})
	t.Run("withdraw 0 amount fails", func(t *testing.T) {
		user := userstestutil.CreateUserWithBalanceOrFail(t, testDB, 500)
		mockLNcli := lntestutil.LightningMockClient{
			SendCoinsResponse: lnrpc.SendCoinsResponse{
				Txid: testutil.MockTxid(),
			},
		}

		transaction, err := WithdrawOnChain(testDB, mockLNcli, mockBitcoin, WithdrawOnChainArgs{
			UserID:    user.ID,
			AmountSat: 0,
			Address:   testnetAddress,
		})

		if err == nil {
			testutil.FatalMsgf(t, "should return error and not send transaction: %+v", transaction)
		}
	})

	t.Run("inserting bad txid fails", func(t *testing.T) {
		mockLNcli := lntestutil.LightningMockClient{
			SendCoinsResponse: lnrpc.SendCoinsResponse{
				Txid: "I am a bad txid",
			},
		}
		user := userstestutil.CreateUserWithBalanceOrFail(t, testDB, 10000)

		transaction, err := WithdrawOnChain(testDB, mockLNcli, mockBitcoin, WithdrawOnChainArgs{
			UserID:    user.ID,
			AmountSat: 5000,
			Address:   testnetAddress,
			SendAll:   true,
		})

		if err == nil {
			testutil.FatalMsgf(t, "should return error and not send transaction: %+v", transaction)
		}
	})
}

func TestTransaction_saveTxToTransaction(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	t.Run("should be able to save txid and vout", func(t *testing.T) {
		user := userstestutil.CreateUserOrFail(t, testDB)
		transaction := CreateTransactionOrFail(t, user.ID)

		hash, err := chainhash.NewHash([]byte(testutil.MockStringOfLength(32)))
		if err != nil {
			testutil.FatalMsgf(t, "should be able to create hash: %+v", err)
		}

		err = transaction.saveTxToTransaction(testDB, *hash, 0, 0)
		if err != nil {
			testutil.FatalMsgf(t, "SaveTxToTransaction(): %+v", err)
		}

		transaction, err = GetTransactionByID(testDB, transaction.ID, transaction.UserID)
		if err != nil {
			testutil.FatalMsgf(t, "should be able to GetTransactionByID: %+v", err)
		}

		if *transaction.Vout != 0 {
			testutil.FatalMsgf(t, "expected vout to be 0, but is %d", *transaction.Vout)
		}

		if *transaction.Txid != hash.String() {
			testutil.FatalMsgf(t, "expected txid to be %s, but is %s", hash.String(), *transaction.Txid)
		}
	})

	t.Run("transaction with txid should fail with 'transaction already has TXID'", func(t *testing.T) {
		txid := "FOO"
		vout := 0
		transaction := Transaction{
			Txid: &txid,
			Vout: &vout,
		}

		hash, err := chainhash.NewHash([]byte(testutil.MockStringOfLength(32)))
		if err != nil {
			testutil.FatalMsgf(t, "should be able to create hash: %+v", hash)
		}

		err = transaction.saveTxToTransaction(testDB, *hash, 0, 0)
		if err != nil && !errors.Is(err, ErrTxHasTxid) {
			testutil.FatalMsgf(t, "error should contain be of type `ErrTxHasTxid`")
		}
	})
}

func TestTransaction_markAsConfirmed(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	t.Run("should mark transaction as confirmed and set confirmedAt", func(t *testing.T) {
		user := userstestutil.CreateUserOrFail(t, testDB)
		transaction := CreateTransactionOrFail(t, user.ID)

		err := transaction.markAsConfirmed(testDB)
		if err != nil {
			testutil.FatalMsgf(t, "could not mark as confirmed: %+v", err)
		}

		transaction, _ = GetTransactionByID(testDB, transaction.ID, user.ID)

		// if !transaction.Confirmed {
		// 	testutil.FatalMsgf(t, "should be confirmed")
		// }

		if transaction.ConfirmedAt == nil {
			testutil.FatalMsgf(t, "ConfirmedAt should have a value")
		}
	})
}

func TestTransaction_ExactlyEqual(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	t.Run("should be equal", func(t *testing.T) {
		t1 := Transaction{
			ID:     3,
			UserID: 3,
			// Address: "footy",
		}

		t2 := Transaction{
			ID:     3,
			UserID: 3,
			// Address: "footy",
		}

		equal, reason := t1.ExactlyEqual(t2)
		if !equal {
			testutil.FatalMsgf(t, "should be equal, but were not: %s", reason)
		}
	})
	t.Run("different dates should not be equal", func(t *testing.T) {
		t1 := Transaction{
			ID:     3,
			UserID: 3,
			// Address:   "footy",
			UpdatedAt: time.Unix(5000, 0),
			CreatedAt: time.Unix(5000, 0),
		}

		t2 := Transaction{
			ID:     3,
			UserID: 3,
			// Address:   "footy",
			UpdatedAt: time.Unix(1000, 0),
			CreatedAt: time.Unix(1000, 0),
		}

		equal, _ := t1.ExactlyEqual(t2)
		if equal {
			testutil.FatalMsgf(t, "should not be equal, but are")
		}
	})
	t.Run("different ID's should not be equal", func(t *testing.T) {
		t1 := Transaction{
			ID:     1,
			UserID: 3,
			// Address: "footy",
		}

		t2 := Transaction{
			ID:     2,
			UserID: 3,
			// Address: "footy",
		}

		equal, _ := t1.ExactlyEqual(t2)
		if equal {
			testutil.FatalMsgf(t, "should not be equal, but are")
		}
	})
}

func TestTransaction_Equal(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	t.Run("should be equal", func(t *testing.T) {
		t1 := Transaction{
			ID:     3,
			UserID: 3,
			// Address: "footy",
		}

		t2 := Transaction{
			ID:     3,
			UserID: 3,
			// Address: "footy",
		}

		equal, reason := t1.Equal(t2)
		if !equal {
			testutil.FatalMsgf(t, "should be equal, but were not: %s", reason)
		}
	})
	t.Run("different dates should be equal", func(t *testing.T) {
		deletedAt1 := time.Unix(5000, 0)
		deletedAt2 := time.Unix(1000, 0)
		t1 := Transaction{
			ID:     3,
			UserID: 3,
			// Address:   "footy",
			CreatedAt: time.Unix(5000, 0),
			UpdatedAt: time.Unix(5000, 0),
			DeletedAt: &deletedAt1,
		}

		t2 := Transaction{
			ID:     3,
			UserID: 3,
			// Address:   "footy",
			CreatedAt: time.Unix(1000, 0),
			UpdatedAt: time.Unix(1000, 0),
			DeletedAt: &deletedAt2,
		}

		equal, reason := t1.Equal(t2)
		if !equal {
			testutil.FatalMsgf(t, "should be equal, but were not: %s", reason)
		}
	})
	t.Run("different dates and ID's should be equal", func(t *testing.T) {
		deletedAt1 := time.Unix(5000, 0)
		deletedAt2 := time.Unix(1000, 0)
		t1 := Transaction{
			ID:     1,
			UserID: 3,
			// Address:   "footy",
			CreatedAt: time.Unix(5000, 0),
			UpdatedAt: time.Unix(5000, 0),
			DeletedAt: &deletedAt1,
		}

		t2 := Transaction{
			ID:     2,
			UserID: 3,
			// Address:   "footy",
			CreatedAt: time.Unix(1000, 0),
			UpdatedAt: time.Unix(1000, 0),
			DeletedAt: &deletedAt2,
		}

		equal, reason := t1.Equal(t2)
		if !equal {
			testutil.FatalMsgf(t, "should be equal, but were not: %s", reason)
		}
	})
	t.Run("should not be equal", func(t *testing.T) {
		t1 := Transaction{
			// Address: "footy",
		}

		t2 := Transaction{
			// Address: "footyBar",
		}

		equal, _ := t1.Equal(t2)
		if equal {
			testutil.FatalMsgf(t, "should not be equal, but are")
		}
	})
}

func assertTransactionsAreEqual(t *testing.T, actual, expected Transaction) {
	t.Helper()
	ok, diff := actual.Equal(expected)
	if !ok {
		t.Fatalf("transactions not equal: %s", diff)
	}
}

func CreateTransactionOrFail(t *testing.T, userID int) Transaction {

	bar := "bar"
	txs := Transaction{
		UserID:     userID,
		AmountMSat: int64(gofakeit.Number(0, ln.MaxAmountMsatPerInvoice)),
		// Address:     "foo",
		Description: &bar,
		Direction:   INBOUND,
	}

	transaction, err := insertTransaction(testDB, txs)

	if err != nil {
		testutil.FatalMsgf(t, "should be able to insertTransaction. Error:  %+v",
			err)
	}

	return transaction
}
