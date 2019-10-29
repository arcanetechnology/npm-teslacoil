package db_test

import (
	"errors"
	"math"
	"testing"
	"time"

	"gitlab.com/arcanecrypto/teslacoil/testutil/userstestutil"

	"gitlab.com/arcanecrypto/teslacoil/testutil/transactiontestutil"

	"github.com/brianvoe/gofakeit"

	"gitlab.com/arcanecrypto/teslacoil/testutil"
)

var (
	ErrConstraintPositiveVout                  = errors.New("transactions_positive_vout")
	ErrConstraintPositiveExpiry                = errors.New("transactions_positive_expiry")
	ErrConstraintTxidVoutUnique                = errors.New("transactions_txid_and_vout_must_be_unique")
	ErrConstraintTxidLength                    = errors.New("transactions_txid_length")
	ErrMustHaveTxidIfConfirmedAtBlock          = errors.New("transactions_must_have_txid_if_confirmed_at_block")
	ErrMustHaveTxidIfConfirmedAt               = errors.New("transactions_must_have_txid_if_confirmed_at")
	ErrMustHaveTxidOrPaymentRequestIfSettledAt = errors.New("transactions_must_have_txid_or_payment_request_if_settled_at")
	ErrConstraintTxidOrVoutCantExistAlone      = errors.New("transactions_txid_or_vout_cant_exist_alone")
	ErrConstraintHashMustExistIfPreimage       = errors.New("transactions_hash_must_exist_if_preimage_is_defined")
	ErrConstraintEitherOnChainOrOffchain       = errors.New("transactions_must_either_be_onchain_or_offchain")
	ErrConstraintPositiveAmountMilliSat        = errors.New("transactions_positive_amount_milli_sat")
)

var address = "tb1qr9t688zvk4xhtz24f0q8hzand3dpvu89kzkwgs"

func init() {
	databaseConfig = testutil.GetDatabaseConfig("constraint")
	testDB = testutil.InitDatabase(databaseConfig)
}

func TestTransactionsPositiveVout(t *testing.T) {
	insertMockTransaction := func(vout int) error {
		txid := transactiontestutil.MockTxid()
		_, err := testDB.NamedExec(`
		INSERT INTO transactions (direction, address, txid, vout)
			VALUES (:direction, :address,:txid, :vout)`,
			map[string]interface{}{
				"direction": transactiontestutil.MockDirection(),
				"address":   address,
				"txid":      txid,
				"vout":      vout,
			},
		)
		return err
	}

	t.Run("can insert positive vout", func(t *testing.T) {
		vout := gofakeit.Number(1, math.MaxInt32-1)

		err := insertMockTransaction(vout)
		testutil.AssertEqual(t, nil, err)
	})

	t.Run("can insert 0 vout", func(t *testing.T) {
		vout := 0

		err := insertMockTransaction(vout)
		testutil.AssertEqual(t, nil, err)
	})

	t.Run("can not insert negative vout", func(t *testing.T) {
		vout := gofakeit.Number(math.MinInt32+2, -1)

		err := insertMockTransaction(vout)
		testutil.AssertEqual(t, ErrConstraintPositiveVout, err)
	})
}

func TestTransactionsPositiveExpiry(t *testing.T) {
	insertMockTransaction := func(expiry int) error {
		txid := transactiontestutil.MockTxid()
		_, err := testDB.NamedExec(`
		INSERT INTO transactions (direction, address, txid, vout, expiry)
			VALUES (:direction, :address,:txid, :vout, :expiry)`,
			map[string]interface{}{
				"direction": transactiontestutil.MockDirection(),
				"address":   address,
				"txid":      txid,
				"vout":      0,
				"expiry":    expiry,
			},
		)
		return err
	}

	t.Run("can insert positive expiry", func(t *testing.T) {
		expiry := gofakeit.Number(1, math.MaxInt64-1)

		err := insertMockTransaction(expiry)
		testutil.AssertEqual(t, nil, err)
	})

	t.Run("can insert 0 expiry", func(t *testing.T) {
		err := insertMockTransaction(0)
		testutil.AssertEqual(t, nil, err)
	})

	t.Run("can not insert negative expiry", func(t *testing.T) {
		expiry := gofakeit.Number(math.MinInt64+2, -1)

		err := insertMockTransaction(expiry)
		testutil.AssertEqual(t, ErrConstraintPositiveExpiry, err)
	})
}

// TestTransactionsAmountSat tests that it is impossible to create a transaction with amount less than 0
func TestTransactionsTxidAndVoutMustBeUnique(t *testing.T) {
	insertMockTransaction := func(txid string, vout int) error {
		_, err := testDB.NamedExec(`
		INSERT INTO transactions (direction, address, txid, vout)
			VALUES (:direction, :address,:txid, :vout)`,
			map[string]interface{}{
				"direction": transactiontestutil.MockDirection(),
				"address":   address,
				"txid":      txid,
				"vout":      vout,
			},
		)
		return err
	}

	t.Run("can insert two transactions with same txid but different vout", func(t *testing.T) {
		txid := transactiontestutil.MockTxid()
		// to test for vouts of big sizes as well, we randomize the vout
		// we don't want to generate a random number for both, as it is a chance they are the same
		vout := gofakeit.Number(1, math.MaxInt32-1)

		err := insertMockTransaction(txid, 0)
		testutil.AssertEqual(t, nil, err)

		err = insertMockTransaction(txid, vout)
		testutil.AssertEqual(t, nil, err)
	})

	t.Run("can insert two transactions with different txid but same vout", func(t *testing.T) {
		txid1 := transactiontestutil.MockTxid()
		txid2 := transactiontestutil.MockTxid()
		vout := gofakeit.Number(0, math.MaxInt32-1)

		err := insertMockTransaction(txid1, vout)
		testutil.AssertEqual(t, nil, err)

		err = insertMockTransaction(txid2, vout)
		testutil.AssertEqual(t, nil, err)
	})

	t.Run("can insert two transactions with different txid and different vout", func(t *testing.T) {
		txid1 := transactiontestutil.MockTxid()
		txid2 := transactiontestutil.MockTxid()
		vout := gofakeit.Number(1, math.MaxInt32-1)

		err := insertMockTransaction(txid1, 0)
		testutil.AssertEqual(t, nil, err)

		err = insertMockTransaction(txid2, vout)
		testutil.AssertEqual(t, nil, err)
	})

	t.Run("can not insert two transactions with same txid and vout", func(t *testing.T) {
		txid := transactiontestutil.MockTxid()
		vout := gofakeit.Number(0, math.MaxInt32-1)

		err := insertMockTransaction(txid, vout)
		testutil.AssertEqual(t, nil, err)

		err = insertMockTransaction(txid, vout)
		testutil.AssertEqual(t, ErrConstraintTxidVoutUnique, err)
	})

}

// TestTransactionsAmountSat tests that it is impossible to create a transaction with amount less than 0
func TestTransactionsTxidLength(t *testing.T) {
	insertMockTransaction := func(txid string) error {
		_, err := testDB.NamedExec(`
		INSERT INTO transactions (direction, address, txid, vout)
			VALUES (:direction, :address,:txid, :vout)`,
			map[string]interface{}{
				"direction": transactiontestutil.MockDirection(),
				"address":   address,
				"txid":      txid,
				"vout":      0,
			},
		)
		return err
	}

	t.Run("can insert txid of length 64", func(t *testing.T) {
		err := insertMockTransaction(transactiontestutil.MockTxid())
		testutil.AssertEqual(t, nil, err)
	})

	t.Run("can not insert txid of length 0", func(t *testing.T) {
		err := insertMockTransaction("")
		testutil.AssertEqual(t, ErrConstraintTxidLength, err)
	})

	t.Run("can not insert txid of length less than 64", func(t *testing.T) {
		err := insertMockTransaction(testutil.MockStringOfLength(gofakeit.Number(1, 63)))
		testutil.AssertEqual(t, ErrConstraintTxidLength, err)
	})

	t.Run("can not insert txid of length greater than 64", func(t *testing.T) {
		err := insertMockTransaction(testutil.MockStringOfLength(gofakeit.Number(65, 256)))
		testutil.AssertEqual(t, ErrConstraintTxidLength, err)
	})
}

// TestTransactionsAmountSat tests that it is impossible to create a transaction with amount less than 0
func TestTransactionsOnchainMustHaveTxidIfConfirmedOrSettled(t *testing.T) {
	insertMockTransaction := func(txid *string, confirmedAt *time.Time, confirmedAtBlock *int, settledAt *time.Time) error {
		var vout *int
		if txid != nil {
			// to not kick of the txid_or_vout_cant_exist_alone constraint
			num := gofakeit.Number(0, math.MaxInt32-1)
			vout = &num
		}

		_, err := testDB.NamedExec(`
		INSERT INTO transactions (direction, address, txid, vout, confirmed_at, confirmed_at_block, settled_at)
			VALUES (:direction, :address,:txid, :vout, :confirmed_at, :confirmed_at_block, :settled_at)`,
			map[string]interface{}{
				"direction":          transactiontestutil.MockDirection(),
				"address":            address,
				"txid":               txid,
				"vout":               vout,
				"confirmed_at":       confirmedAt,
				"confirmed_at_block": confirmedAtBlock,
				"settled_at":         settledAt,
			},
		)

		return err
	}

	now := time.Now()

	t.Run("can insert transaction with confirmed_at and txid", func(t *testing.T) {
		txid := transactiontestutil.MockTxid()
		err := insertMockTransaction(&txid, &now, nil, nil)

		testutil.AssertEqual(t, nil, err)
	})

	t.Run("can insert transaction with confirmed_at_block and txid", func(t *testing.T) {
		txid := transactiontestutil.MockTxid()
		confirmedAtBlock := gofakeit.Number(100, 2000000)
		err := insertMockTransaction(&txid, nil, &confirmedAtBlock, nil)

		testutil.AssertEqual(t, nil, err)
	})

	t.Run("can insert transaction with settled_at and txid", func(t *testing.T) {
		txid := transactiontestutil.MockTxid()
		err := insertMockTransaction(&txid, nil, nil, &now)

		testutil.AssertEqual(t, nil, err)
	})

	t.Run("can not be confirmed_at if txid is not present", func(t *testing.T) {
		err := insertMockTransaction(nil, &now, nil, nil)

		testutil.AssertEqual(t, ErrMustHaveTxidIfConfirmedAt, err)
	})

	t.Run("can not be confirmed_at_block if txid is not present", func(t *testing.T) {
		confirmedAtBlock := gofakeit.Number(100, 2000000)
		err := insertMockTransaction(nil, nil, &confirmedAtBlock, nil)

		testutil.AssertEqual(t, ErrMustHaveTxidIfConfirmedAtBlock, err)
	})

	t.Run("can not be settled_at if txid is not present", func(t *testing.T) {
		err := insertMockTransaction(nil, nil, nil, &now)

		testutil.AssertEqual(t, ErrMustHaveTxidOrPaymentRequestIfSettledAt, err)
	})
}

// TestTransactionsAmountSat tests that it is impossible to create a transaction with amount less than 0
func TestTransactionsTxidOrVoutCantExistAlone(t *testing.T) {
	insertMockTransaction := func(txid *string, vout *int) error {
		_, err := testDB.NamedExec(`
		INSERT INTO transactions (direction, address, txid, vout)
			VALUES (:direction, :address,:txid, :vout)`,
			map[string]interface{}{
				"direction": transactiontestutil.MockDirection(),
				"address":   address,
				"txid":      txid,
				"vout":      vout,
			},
		)
		return err
	}

	t.Run("can insert transaction with txid and vout", func(t *testing.T) {
		txid := transactiontestutil.MockTxid()
		vout := gofakeit.Number(0, math.MaxInt32)

		err := insertMockTransaction(&txid, &vout)
		testutil.AssertEqual(t, nil, err)
	})

	t.Run("can not insert transaction with just txid", func(t *testing.T) {
		txid := transactiontestutil.MockTxid()

		err := insertMockTransaction(&txid, nil)
		testutil.AssertEqual(t, ErrConstraintTxidOrVoutCantExistAlone, err)

	})

	t.Run("can not insert transaction with just vout", func(t *testing.T) {
		vout := gofakeit.Number(0, math.MaxInt32)

		err := insertMockTransaction(nil, &vout)
		testutil.AssertEqual(t, ErrConstraintTxidOrVoutCantExistAlone, err)

	})
}

// TestTransactionsAmountSat tests that it is impossible to create a transaction with amount less than 0
func TestTransactionsHashMustExistIfPreimageIsDefined(t *testing.T) {
	insertMockTransaction := func(preimage *[]byte, hashedPreimage *[]byte) error {
		paymentRequest := "pay_req"
		_, err := testDB.NamedExec(`
		INSERT INTO transactions (direction, payment_request, preimage, hashed_preimage)
			VALUES (:direction, :payment_request, :preimage, :hashed_preimage)`,
			map[string]interface{}{
				"direction":       transactiontestutil.MockDirection(),
				"payment_request": paymentRequest,
				"preimage":        preimage,
				"hashed_preimage": hashedPreimage,
			},
		)
		return err
	}

	t.Run("can insert transaction with hash and preimage", func(t *testing.T) {
		preimage := transactiontestutil.MockPreimage()
		hash := transactiontestutil.MockHash([]byte("a really bad preimage"))

		err := insertMockTransaction(&preimage, &hash)
		testutil.AssertEqual(t, nil, err)
	})

	t.Run("can insert transaction with just payment hash", func(t *testing.T) {
		hash := transactiontestutil.MockHash([]byte("a really bad preimage"))

		err := insertMockTransaction(nil, &hash)
		testutil.AssertEqual(t, nil, err)
	})

	t.Run("can not insert transaction with just preimage", func(t *testing.T) {
		preimage := transactiontestutil.MockPreimage()

		err := insertMockTransaction(&preimage, nil)
		testutil.AssertEqual(t, ErrConstraintHashMustExistIfPreimage, err)
	})
}

// TestTransactionsAmountSat tests that it is impossible to create a transaction with amount less than 0
func TestTransactionsPaymentRequestMustExistForOtherFieldsToExist(t *testing.T) {
	t.Run("can insert transaction with payment request and memo", func(t *testing.T) {

	})
	t.Run("can insert transaction with payment and hashed_preimage", func(t *testing.T) {

	})
	t.Run("can not insert transaction with just memo", func(t *testing.T) {

	})
	t.Run("can not insert transaction with just hashed_preimage", func(t *testing.T) {

	})

	t.Run("can not insert transaction with just hashed_preimage and memo", func(t *testing.T) {

	})
}

// TestTransactionsAmountSat tests that it is impossible to create a transaction with amount less than 0
func TestTransactionsMustEitherBeOnchainOrOffchain(t *testing.T) {
	user := userstestutil.CreateUserOrFail(t, testDB)

	t.Run("can insert onchain transaction", func(t *testing.T) {
		_, err := transactiontestutil.InsertFakeOnchain(t, testDB, user.ID)
		testutil.AssertEqual(t, nil, err)
	})
	t.Run("can insert offchain transaction", func(t *testing.T) {
		_, err := transactiontestutil.InsertFakeOffchain(t, testDB, user.ID)
		testutil.AssertEqual(t, nil, err)
	})
	t.Run("can not insert transaction with address and payment_request", func(t *testing.T) {
		insertMockTransaction := func() error {
			paymentRequest := "fake payment request"
			_, err := testDB.NamedExec(`
		INSERT INTO transactions (direction, address, payment_request)
			VALUES (:direction, :address, :payment_request)`,
				map[string]interface{}{
					"direction":       transactiontestutil.MockDirection(),
					"address":         address,
					"payment_request": paymentRequest,
				},
			)
			return err
		}

		err := insertMockTransaction()
		testutil.AssertEqual(t, ErrConstraintEitherOnChainOrOffchain, err)
	})

}

// TestTransactionsAmountSat tests that it is impossible to create a transaction with amount less than 0
func TestTransactionsAmountMilliSat(t *testing.T) {
	insertMockTransaction := func(amountMilliSat int) error {
		_, err := testDB.NamedExec(`
		INSERT INTO transactions (direction, address, amount_milli_sat)
			VALUES (:direction, :address, :amount_milli_sat)`,
			map[string]interface{}{
				"direction":        transactiontestutil.MockDirection(),
				"address":          address,
				"amount_milli_sat": amountMilliSat,
			},
		)
		return err
	}

	t.Run("inserting transaction with 0 amount succeeds", func(t *testing.T) {
		err := insertMockTransaction(0)
		testutil.AssertEqual(t, nil, err)
	})

	t.Run("inserting transaction with greater than 0 amount succeeds", func(t *testing.T) {
		amount := gofakeit.Number(1, math.MaxInt64-1)

		err := insertMockTransaction(amount)
		testutil.AssertEqual(t, nil, err)
	})

	t.Run("inserting transaction with negative amount fails", func(t *testing.T) {
		amount := gofakeit.Number(math.MinInt64+2, -1)

		err := insertMockTransaction(amount)
		testutil.AssertEqual(t, ErrConstraintPositiveAmountMilliSat, err)
	})
}
