package transactions

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"testing"

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
)

var (
	testnetAddress = "tb1q40gzxjcamcny49st7m8lyz9rtmssjgfefc33at"
	simnetAddress  = "sb1qnl462s336uu4n8xanhyvpega4zwjr9jrhc26x4"
)

func TestMain(m *testing.M) {
	build.SetLogLevel(logrus.ErrorLevel)

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

func TestWithdrawOnChainBadOpts(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		scenario  string
		balance   int64
		amountSat int64
	}{
		{
			scenario:  "withdraw more than balance",
			balance:   1000,
			amountSat: 2000,
		},
		{
			scenario:  "withdraw negative amount",
			balance:   1000,
			amountSat: -5000,
		},
		{
			scenario:  "withdraw 0 amount",
			balance:   2000,
			amountSat: 0,
		},
	}
	mockLNcli := lntestutil.LightningMockClient{
		SendCoinsResponse: lnrpc.SendCoinsResponse{
			Txid: "owrgkpoaerkgpok",
		},
	}

	for _, test := range testCases {
		user := CreateUserWithBalanceOrFail(t, test.balance)

		t.Run(test.scenario, func(t *testing.T) {
			txid, err := WithdrawOnChain(testDB, mockLNcli, WithdrawOnChainArgs{
				UserID:    user.ID,
				AmountSat: test.amountSat,
				Address:   simnetAddress,
			})
			if err == nil || txid != nil {
				testutil.FatalMsgf(t, "should not send transaction, bad opts")
			}
		})
	}

}

func TestWithdrawOnChainSendAll(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		balance int64
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

		user := CreateUserWithBalanceOrFail(t, test.balance)

		t.Run("can withdraw on-chain", func(t *testing.T) {

			mockLNcli := lntestutil.LightningMockClient{
				SendCoinsResponse: lnrpc.SendCoinsResponse{
					Txid: "owrgkpoaerkgpok",
				},
			}

			_, err := WithdrawOnChain(testDB, mockLNcli, WithdrawOnChainArgs{
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
		})

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
}

func assertTransactionsAreEqual(t *testing.T, actual, expected Transaction) {
	t.Helper()
	ok, diff := actual.Equal(expected)
	if !ok {
		t.Fatalf("transactions not equal: %s", diff)
	}
}

func CreateUserWithBalanceOrFail(t *testing.T, balance int64) users.User {
	u := userstestutil.CreateUserOrFail(t, testDB)

	tx := testDB.MustBegin()
	user, err := users.IncreaseBalance(tx, users.ChangeBalance{
		UserID:    u.ID,
		AmountSat: balance,
	})
	if err != nil {
		testutil.FatalMsgf(t,
			"[%s] could not increase balance by %d for user %d: %+v", t.Name(),
			balance, u.ID, err)
	}
	err = tx.Commit()
	if err != nil {
		testutil.FatalMsg(t, "could not commit tx")
	}

	if user.Balance != balance {
		testutil.FatalMsgf(t, "wrong balance, expected [%d], got [%d]", balance,
			user.Balance)
	}

	return user
}
