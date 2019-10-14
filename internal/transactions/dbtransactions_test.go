package transactions

import (
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"gitlab.com/arcanecrypto/teslacoil/internal/platform/bitcoind"

	"github.com/brianvoe/gofakeit"
	"github.com/btcsuite/btcd/chaincfg/chainhash"

	"gitlab.com/arcanecrypto/teslacoil/internal/payments"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/ln"
	"gitlab.com/arcanecrypto/teslacoil/testutil/lntestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/userstestutil"

	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"

	"github.com/lightningnetwork/lnd/lnrpc"
	"gitlab.com/arcanecrypto/teslacoil/internal/users"
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

	flag.Parse()
	result := m.Run()

	os.Exit(result)
}

func TestGetTransactionByID(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	const email1 = "email1@example.com"
	const password1 = "password1"
	const email2 = "email2@example.com"
	const password2 = "password2"
	amount1 := rand.Int63n(ln.MaxAmountSatPerInvoice)
	amount2 := rand.Int63n(ln.MaxAmountSatPerInvoice)

	user := userstestutil.CreateUserOrFail(t, testDB)

	testCases := []struct {
		email          string
		password       string
		expectedResult Transaction
	}{
		{

			email1,
			password1,
			Transaction{
				UserID:      user.ID,
				AmountSat:   amount1,
				Address:     "bar",
				Description: "foo",
				Direction:   payments.Direction("INBOUND"),
				Confirmed:   false,
			},
		},
		{

			email2,
			password2,
			Transaction{
				UserID:      user.ID,
				AmountSat:   amount2,
				Address:     "bar",
				Description: "foo",
				Direction:   payments.Direction("INBOUND"),
				Confirmed:   false,
			},
		},
	}

	for _, test := range testCases {
		t.Run(fmt.Sprintf("GetTransactionByID() for transaction with amount %d", test.expectedResult.AmountSat),
			func(t *testing.T) {

				tx := testDB.MustBegin()

				transaction, err := insertTransaction(tx, test.expectedResult)

				if err != nil {
					testutil.FatalMsgf(t, "should be able to insertTransaction. Error:  %+v",
						err)
				}
				_ = tx.Commit()

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
				user, err := users.GetByID(testDB, user.ID)
				if err != nil {
					testutil.FatalMsgf(t, "could not get user %+v", err)
				}
				if user.Balance != 0 {
					testutil.FatalMsgf(t, "users balance was not 0 %+v", err)
				}
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

		log.Infof("users balance is %d", user.Balance)

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

func TestTransaction_SaveTxToDeposit(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	t.Run("should be able to save txid and vout", func(t *testing.T) {
		user := userstestutil.CreateUserOrFail(t, testDB)
		transaction := CreateTransactionOrFail(t, user.ID)

		hash, err := chainhash.NewHash([]byte(testutil.MockStringOfLength(32)))
		if err != nil {
			testutil.FatalMsgf(t, "should be able to create hash: %+v", err)
		}

		err = transaction.SaveTxToDeposit(testDB, *hash, 0, 0)
		if err != nil {
			testutil.FatalMsgf(t, "SaveTxToDeposit(): %+v", err)
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

		err = transaction.SaveTxToDeposit(testDB, *hash, 0, 0)
		if err != nil && !errors.Is(err, ErrTxHasTxid) {
			testutil.FatalMsgf(t, "error should contain be of type `ErrTxHasTxid`")
		}
	})
}

func TestTransaction_MarkAsConfirmed(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	t.Run("should mark transaction as confirmed and set confirmedAt", func(t *testing.T) {
		user := userstestutil.CreateUserOrFail(t, testDB)
		transaction := CreateTransactionOrFail(t, user.ID)

		err := transaction.MarkAsConfirmed(testDB)
		if err != nil {
			testutil.FatalMsgf(t, "could not mark as confirmed: %+v", err)
		}

		transaction, _ = GetTransactionByID(testDB, transaction.ID, user.ID)

		if !transaction.Confirmed {
			testutil.FatalMsgf(t, "should be confirmed")
		}

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
			ID:      3,
			UserID:  3,
			Address: "footy",
		}

		t2 := Transaction{
			ID:      3,
			UserID:  3,
			Address: "footy",
		}

		equal, reason := t1.ExactlyEqual(t2)
		if !equal {
			testutil.FatalMsgf(t, "should be equal, but were not: %s", reason)
		}
	})
	t.Run("different dates should not be equal", func(t *testing.T) {
		t1 := Transaction{
			ID:        3,
			UserID:    3,
			Address:   "footy",
			UpdatedAt: time.Unix(5000, 0),
			CreatedAt: time.Unix(5000, 0),
		}

		t2 := Transaction{
			ID:        3,
			UserID:    3,
			Address:   "footy",
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
			ID:      1,
			UserID:  3,
			Address: "footy",
		}

		t2 := Transaction{
			ID:      2,
			UserID:  3,
			Address: "footy",
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
			ID:      3,
			UserID:  3,
			Address: "footy",
		}

		t2 := Transaction{
			ID:      3,
			UserID:  3,
			Address: "footy",
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
			ID:        3,
			UserID:    3,
			Address:   "footy",
			CreatedAt: time.Unix(5000, 0),
			UpdatedAt: time.Unix(5000, 0),
			DeletedAt: &deletedAt1,
		}

		t2 := Transaction{
			ID:        3,
			UserID:    3,
			Address:   "footy",
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
			ID:        1,
			UserID:    3,
			Address:   "footy",
			CreatedAt: time.Unix(5000, 0),
			UpdatedAt: time.Unix(5000, 0),
			DeletedAt: &deletedAt1,
		}

		t2 := Transaction{
			ID:        2,
			UserID:    3,
			Address:   "footy",
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
			Address: "footy",
		}

		t2 := Transaction{
			Address: "footyBar",
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
	tx := testDB.MustBegin()

	txs := Transaction{
		UserID:      userID,
		AmountSat:   int64(gofakeit.Number(0, ln.MaxAmountMsatPerInvoice)),
		Address:     "foo",
		Description: "bar",
		Direction:   payments.Direction("INBOUND"),
		Confirmed:   false,
	}

	transaction, err := insertTransaction(tx, txs)

	if err != nil {
		testutil.FatalMsgf(t, "should be able to insertTransaction. Error:  %+v",
			err)
	}
	_ = tx.Commit()

	return transaction
}
