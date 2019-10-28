package db_test

import (
	"errors"
	"math"
	"testing"

	"gitlab.com/arcanecrypto/teslacoil/testutil/transactiontestutil"

	"github.com/brianvoe/gofakeit"

	"gitlab.com/arcanecrypto/teslacoil/testutil"
)

var (
	ErrAmountSatConstraint               = errors.New("transactions_amount_sat_must_be_greater_than_0")
	ErrTxidVoutUniqueConstraint          = errors.New("transactions_txid_and_vout_must_be_unique")
	ErrTxidLengthConstraint              = errors.New("transactions_txid_length")
	ErrEitherOnChainOrOffchainConstraint = errors.New("transactions_must_either_onchain_or_offchain ")
)

func init() {
	databaseConfig = testutil.GetDatabaseConfig("constraint")
	testDB = testutil.InitDatabase(databaseConfig)
}

func insertMockTransaction(amountSat int, txid string, vout int) error {

	_, err := testDB.NamedExec(`
		INSERT INTO transactions (amount_milli_sat, direction, address, txid, vout)
			VALUES (:amount_milli_sat, :direction, :address,:txid, :vout)`,
		map[string]interface{}{
			"amount_milli_sat": amountSat,
			"direction":        transactiontestutil.MockDirection(),
			"address":          "this is an address",
			"txid":             txid,
			"vout":             vout,
		},
	)
	return err
}

// TestTransactionsAmountSatConstraint tests that it is impossible to create a transaction with amount less than 0
func TestTransactionsAmountSatConstraint(t *testing.T) {
	t.Run("inserting transaction with 0 amount succeeds", func(t *testing.T) {
		txid := testutil.MockTxid()

		if err := insertMockTransaction(0, txid, 0); err != nil {
			testutil.FailMsgf(t, "could not insert: %w", err)
		}
	})

	t.Run("inserting transaction with greater than 0 amount succeeds", func(t *testing.T) {
		txid := testutil.MockTxid()
		amountSat := gofakeit.Number(0, math.MaxInt64-1)

		if err := insertMockTransaction(amountSat, txid, 0); err != nil {
			testutil.FailMsgf(t, "could not insert: %w", err)
		}
	})

	t.Run("inserting transaction with negative amount fails", func(t *testing.T) {
		txid := testutil.MockTxid()
		amountSat := gofakeit.Number(math.MinInt64+2, 0)

		if err := insertMockTransaction(amountSat, txid, 0); err == nil {
			testutil.FailMsg(t, "successfully inserted transaction with negative amount")
		}
	})
}

// TestTransactionsAmountSatConstraint tests that it is impossible to create a transaction with amount less than 0
func TestTransactionsTxidAndVoutMustBeUniqueConstraint(t *testing.T) {
	t.Run("can insert two transactions with same txid but different vout", func(t *testing.T) {
		txid := testutil.MockTxid()
		// to test for vouts of big sizes as well, we randomize a number
		// we don't want to generate a random number for both, as it is a chance they are the same
		vout := gofakeit.Number(1, math.MaxInt64-1)
		amountSat := gofakeit.Number(0, math.MaxInt64-1)

		if err := insertMockTransaction(amountSat, txid, 0); err != nil {
			testutil.FailMsgf(t, "could not insert: %w", err)
		}

		if err := insertMockTransaction(amountSat, txid, vout); err != nil {
			testutil.FailMsgf(t, "could not insert: %w", err)
		}
	})

	t.Run("can insert two transactions with different txid but same vout", func(t *testing.T) {
		txid1 := testutil.MockTxid()
		txid2 := testutil.MockTxid()
		vout := gofakeit.Number(0, math.MaxInt64-1)
		amountSat := gofakeit.Number(0, math.MaxInt64-1)

		if err := insertMockTransaction(amountSat, txid1, vout); err != nil {
			testutil.FailMsgf(t, "could not insert: %w", err)
		}

		if err := insertMockTransaction(amountSat, txid2, vout); err != nil {
			testutil.FailMsgf(t, "could not insert: %w", err)
		}
	})

	t.Run("can insert two transactions with different txid and different vout", func(t *testing.T) {

		txid1 := testutil.MockTxid()
		txid2 := testutil.MockTxid()
		vout := gofakeit.Number(1, math.MaxInt64-1)
		amountSat := gofakeit.Number(0, math.MaxInt64-1)

		if err := insertMockTransaction(amountSat, txid1, 0); err != nil {
			testutil.FailMsgf(t, "could not insert: %w", err)
		}

		if err := insertMockTransaction(amountSat, txid2, vout); err != nil {
			testutil.FailMsgf(t, "could not insert: %w", err)
		}
	})

	t.Run("can not insert two transactions with same txid and vout", func(t *testing.T) {
		txid := testutil.MockTxid()
		vout := gofakeit.Number(0, math.MaxInt64-1)
		amountSat := gofakeit.Number(math.MinInt64+2, 0)

		if err := insertMockTransaction(amountSat, txid, vout); err == nil {
			testutil.FailMsgf(t, "inserted regardless of constraint")
		}

		if err := insertMockTransaction(amountSat, txid, vout); err == nil {
			testutil.FailMsgf(t, "inserted regardless of constraint")
		}
	})

}

// TestTransactionsAmountSatConstraint tests that it is impossible to create a transaction with amount less than 0
func TestTransactionsTxidLengthConstraint(t *testing.T) {
	t.Run("can insert txid of length 64", func(t *testing.T) {

		if err := insertMockTransaction(
			gofakeit.Number(0, math.MaxInt64-1),
			testutil.MockTxid(),
			gofakeit.Number(0, math.MaxInt64-1)); err != nil {
			testutil.FailMsgf(t, "could not insert transaction: %w", err)
		}

	})

	t.Run("can not insert txid of length 0", func(t *testing.T) {

		if err := insertMockTransaction(
			gofakeit.Number(0, math.MaxInt64-1),
			testutil.MockStringOfLength(0),
			gofakeit.Number(0, math.MaxInt64-1)); err == nil {
			testutil.FailMsg(t, "inserted bad transaction")
		}

	})

	t.Run("can not insert txid of length less than 64", func(t *testing.T) {

		if err := insertMockTransaction(
			gofakeit.Number(0, math.MaxInt64-1),
			testutil.MockStringOfLength(gofakeit.Number(0, 63)),
			gofakeit.Number(0, math.MaxInt64-1)); err == nil {
			testutil.FailMsg(t, "inserted bad transaction")
		}

	})

	t.Run("can not insert txid of length greater than 64", func(t *testing.T) {

		if err := insertMockTransaction(
			gofakeit.Number(0, math.MaxInt64-1),
			testutil.MockStringOfLength(gofakeit.Number(65, 500)),
			gofakeit.Number(0, math.MaxInt64-1)); err == nil {
			testutil.FailMsg(t, "inserted bad transaction")
		}

	})
}

// TestTransactionsAmountSatConstraint tests that it is impossible to create a transaction with amount less than 0
func TestTransactionsMustEitherBeOnOrOffchainConstraint(t *testing.T) {
	testutil.DescribeTest(t)

	t.Run("can insert on-chain transaction with all fields", func(t *testing.T) {
	})

	t.Run("can insert lightning transaction with all fields", func(t *testing.T) {
	})

	t.Run("can not insert transaction with address and payment request", func(t *testing.T) {
	})

	t.Run("can not insert transaction with txid, vout and payment request", func(t *testing.T) {
	})

	t.Run("can not insert transaction with ", func(t *testing.T) {
	})

	t.Run("can insert transaction with all fields", func(t *testing.T) {
	})
}

// TestTransactionsAmountSatConstraint tests that it is impossible to create a transaction with amount less than 0
func TestTransactionsTxidOrVoutCantExistAloneConstraint(t *testing.T) {
}

// TestTransactionsAmountSatConstraint tests that it is impossible to create a transaction with amount less than 0
func TestTransactionsInvoiceStatusMustExistIfPaymentRequestExists(t *testing.T) {}

// TestTransactionsAmountSatConstraint tests that it is impossible to create a transaction with amount less than 0
func TestTransactionsHashMustExistIfPeimageIsDefined(t *testing.T) {}

// TestTransactionsAmountSatConstraint tests that it is impossible to create a transaction with amount less than 0
func TestPaymentRequestMustExistForOtherFieldsToExist(t *testing.T) {}

// TestTransactionsAmountSatConstraint tests that it is impossible to create a transaction with amount less than 0
func TestTransactionsOnchainMustHaveAddressIfHasTxid(t *testing.T) {}

// TestTransactionsAmountSatConstraint tests that it is impossible to create a transaction with amount less than 0
func TestTransactionsOnchainMustHaveTxidIfConfirmedOrSettled(t *testing.T) {}
