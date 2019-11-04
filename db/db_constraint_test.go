package db_test

import (
	"errors"
	"math"
	"testing"
	"time"

	"gitlab.com/arcanecrypto/teslacoil/testutil/txtest"

	"github.com/stretchr/testify/assert"

	"gitlab.com/arcanecrypto/teslacoil/testutil/userstestutil"

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
		txid := txtest.MockTxid()
		_, err := testDB.NamedExec(`
		INSERT INTO transactions (direction, address, txid, vout, received_tx_at)
			VALUES (:direction, :address,:txid, :vout, :received_tx_at)`,
			map[string]interface{}{
				"direction":      txtest.MockDirection(),
				"address":        address,
				"txid":           txid,
				"vout":           vout,
				"received_tx_at": time.Now(),
			},
		)
		return err
	}

	t.Run("can insert positive vout", func(t *testing.T) {
		vout := gofakeit.Number(1, math.MaxInt32-1)

		err := insertMockTransaction(vout)
		assert.NoError(t, err)
	})

	t.Run("can insert 0 vout", func(t *testing.T) {
		vout := 0

		err := insertMockTransaction(vout)
		assert.NoError(t, err)
	})

	t.Run("can not insert negative vout", func(t *testing.T) {
		vout := gofakeit.Number(math.MinInt32+2, -1)

		err := insertMockTransaction(vout)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), ErrConstraintPositiveVout.Error())
	})
}

func TestTransactionsPositiveExpiry(t *testing.T) {
	insertMockTransaction := func(expiry int) error {
		txid := txtest.MockTxid()
		_, err := testDB.NamedExec(`
		INSERT INTO transactions (direction, address, txid, vout, expiry, received_tx_at)
			VALUES (:direction, :address,:txid, :vout, :expiry, :received_tx_at)`,
			map[string]interface{}{
				"direction":      txtest.MockDirection(),
				"address":        address,
				"txid":           txid,
				"vout":           0,
				"expiry":         expiry,
				"received_tx_at": gofakeit.Date(),
			},
		)
		return err
	}

	t.Run("can insert positive expiry", func(t *testing.T) {
		expiry := gofakeit.Number(1, math.MaxInt64-1)

		err := insertMockTransaction(expiry)
		assert.NoError(t, err)
	})

	t.Run("can insert 0 expiry", func(t *testing.T) {
		err := insertMockTransaction(0)
		assert.NoError(t, err)
	})

	t.Run("can not insert negative expiry", func(t *testing.T) {
		expiry := gofakeit.Number(math.MinInt64+2, -1)

		err := insertMockTransaction(expiry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), ErrConstraintPositiveExpiry.Error())
	})
}

func TestTransactionsTxidAndVoutMustBeUnique(t *testing.T) {
	insertMockTransaction := func(txid string, vout int) error {
		_, err := testDB.NamedExec(`
		INSERT INTO transactions (direction, address, txid, vout, received_tx_at)
			VALUES (:direction, :address,:txid, :vout, :received_tx_at)`,
			map[string]interface{}{
				"direction":      txtest.MockDirection(),
				"address":        address,
				"txid":           txid,
				"vout":           vout,
				"received_tx_at": time.Now(),
			},
		)
		return err
	}

	t.Run("can insert two transactions with same txid but different vout", func(t *testing.T) {
		txid := txtest.MockTxid()
		// to test for vouts of big sizes as well, we randomize the vout
		// we don't want to generate a random number for both, as it is a chance they are the same
		vout := gofakeit.Number(1, math.MaxInt32-1)

		err := insertMockTransaction(txid, 0)
		assert.NoError(t, err)

		err = insertMockTransaction(txid, vout)
		assert.NoError(t, err)
	})

	t.Run("can insert two transactions with different txid but same vout", func(t *testing.T) {
		txid1 := txtest.MockTxid()
		txid2 := txtest.MockTxid()
		vout := gofakeit.Number(0, math.MaxInt32-1)

		err := insertMockTransaction(txid1, vout)
		assert.NoError(t, err)

		err = insertMockTransaction(txid2, vout)
		assert.NoError(t, err)
	})

	t.Run("can insert two transactions with different txid and different vout", func(t *testing.T) {
		txid1 := txtest.MockTxid()
		txid2 := txtest.MockTxid()
		vout := gofakeit.Number(1, math.MaxInt32-1)

		err := insertMockTransaction(txid1, 0)
		assert.NoError(t, err)

		err = insertMockTransaction(txid2, vout)
		assert.NoError(t, err)
	})

	t.Run("can not insert two transactions with same txid and vout", func(t *testing.T) {
		txid := txtest.MockTxid()
		vout := gofakeit.Number(0, math.MaxInt32-1)

		err := insertMockTransaction(txid, vout)
		assert.NoError(t, err)

		err = insertMockTransaction(txid, vout)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), ErrConstraintTxidVoutUnique.Error())
	})

}

func TestTransactionsTxidLength(t *testing.T) {
	insertMockTransaction := func(txid string) error {
		_, err := testDB.NamedExec(`
		INSERT INTO transactions (direction, address, txid, vout, received_tx_at)
			VALUES (:direction, :address,:txid, :vout, :received_tx_at)`,
			map[string]interface{}{
				"direction":      txtest.MockDirection(),
				"address":        address,
				"txid":           txid,
				"vout":           0,
				"received_tx_at": time.Now(),
			},
		)
		return err
	}

	t.Run("can insert txid of length 64", func(t *testing.T) {
		err := insertMockTransaction(txtest.MockTxid())
		assert.NoError(t, err)
	})

	t.Run("can not insert txid of length 0", func(t *testing.T) {
		err := insertMockTransaction("")
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), ErrConstraintTxidLength.Error())
	})

	t.Run("can not insert txid of length less than 64", func(t *testing.T) {
		err := insertMockTransaction(testutil.MockStringOfLength(gofakeit.Number(1, 63)))
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), ErrConstraintTxidLength.Error())
	})

	t.Run("can not insert txid of length greater than 64", func(t *testing.T) {
		err := insertMockTransaction(testutil.MockStringOfLength(gofakeit.Number(65, 256)))
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), ErrConstraintTxidLength.Error())
	})
}

func TestTransactionsOnchainMustHaveTxidIfConfirmedOrSettled(t *testing.T) {
	insertMockTransaction := func(txid *string, confirmedAt *time.Time, confirmedAtBlock *int, settledAt *time.Time, receivedAt *time.Time) error {
		var vout *int
		if txid != nil {
			// to not kick of the txid_or_vout_cant_exist_alone constraint
			num := gofakeit.Number(0, math.MaxInt32-1)
			vout = &num
		}

		_, err := testDB.NamedExec(`
		INSERT INTO transactions (direction, address, txid, vout, confirmed_at, confirmed_at_block, settled_at, received_tx_at)
			VALUES (:direction, :address,:txid, :vout, :confirmed_at, :confirmed_at_block, :settled_at, :received_tx_at)`,
			map[string]interface{}{
				"direction":          txtest.MockDirection(),
				"address":            address,
				"txid":               txid,
				"vout":               vout,
				"confirmed_at":       confirmedAt,
				"confirmed_at_block": confirmedAtBlock,
				"settled_at":         settledAt,
				"received_tx_at":     receivedAt,
			},
		)

		return err
	}

	now := time.Now()

	t.Run("can insert transaction with confirmed_at and txid", func(t *testing.T) {
		txid := txtest.MockTxid()
		received := gofakeit.Date()
		err := insertMockTransaction(&txid, &now, nil, nil, &received)

		assert.NoError(t, err)
	})

	t.Run("can insert transaction with confirmed_at_block and txid", func(t *testing.T) {
		txid := txtest.MockTxid()
		confirmedAtBlock := gofakeit.Number(100, 2000000)
		received := gofakeit.Date()
		err := insertMockTransaction(&txid, nil, &confirmedAtBlock, nil, &received)

		assert.NoError(t, err)
	})

	t.Run("can insert transaction with settled_at and txid", func(t *testing.T) {
		txid := txtest.MockTxid()
		received := gofakeit.Date()
		err := insertMockTransaction(&txid, nil, nil, &now, &received)

		assert.NoError(t, err)
	})

	t.Run("can not be confirmed_at if txid is not present", func(t *testing.T) {
		err := insertMockTransaction(nil, &now, nil, nil, nil)
		assert.NotNil(t, err)

		assert.Contains(t, err.Error(), ErrMustHaveTxidIfConfirmedAt.Error())
	})

	t.Run("can not be confirmed_at_block if txid is not present", func(t *testing.T) {
		confirmedAtBlock := gofakeit.Number(100, 2000000)
		err := insertMockTransaction(nil, nil, &confirmedAtBlock, nil, nil)
		assert.NotNil(t, err)

		assert.Contains(t, err.Error(), ErrMustHaveTxidIfConfirmedAtBlock.Error())
	})

	t.Run("can not be settled_at if txid is not present", func(t *testing.T) {
		err := insertMockTransaction(nil, nil, nil, &now, nil)

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), ErrMustHaveTxidOrPaymentRequestIfSettledAt.Error())
	})
}

func TestTransactionsTxidOrVoutCantExistAlone(t *testing.T) {
	insertMockTransaction := func(txid *string, vout *int, receivedAt *time.Time) error {
		_, err := testDB.NamedExec(`
		INSERT INTO transactions (direction, address, txid, vout, received_tx_at)
			VALUES (:direction, :address,:txid, :vout, :received_tx_at)`,
			map[string]interface{}{
				"direction":      txtest.MockDirection(),
				"address":        address,
				"txid":           txid,
				"vout":           vout,
				"received_tx_at": receivedAt,
			},
		)
		return err
	}

	t.Run("can insert transaction with txid and vout", func(t *testing.T) {
		txid := txtest.MockTxid()
		vout := gofakeit.Number(0, math.MaxInt32)
		received := gofakeit.Date()

		err := insertMockTransaction(&txid, &vout, &received)
		assert.NoError(t, err)
	})

	t.Run("can not insert transaction with just txid", func(t *testing.T) {
		txid := txtest.MockTxid()
		received := gofakeit.Date()

		err := insertMockTransaction(&txid, nil, &received)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), ErrConstraintTxidOrVoutCantExistAlone.Error())
	})

	t.Run("can not insert transaction with just vout", func(t *testing.T) {
		vout := gofakeit.Number(0, math.MaxInt32)

		err := insertMockTransaction(nil, &vout, nil)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), ErrConstraintTxidOrVoutCantExistAlone.Error())
	})
}

func TestTransactionsHashMustExistIfPreimageIsDefined(t *testing.T) {
	insertMockTransaction := func(preimage *[]byte, hashedPreimage *[]byte) error {
		paymentRequest := "pay_req"
		_, err := testDB.NamedExec(`
		INSERT INTO transactions (direction, payment_request, preimage, hashed_preimage)
			VALUES (:direction, :payment_request, :preimage, :hashed_preimage)`,
			map[string]interface{}{
				"direction":       txtest.MockDirection(),
				"payment_request": paymentRequest,
				"preimage":        preimage,
				"hashed_preimage": hashedPreimage,
			},
		)
		return err
	}

	t.Run("can insert transaction with hash and preimage", func(t *testing.T) {
		preimage := txtest.MockPreimage()
		hash := txtest.MockHash([]byte("a really bad preimage"))

		err := insertMockTransaction(&preimage, &hash)
		assert.NoError(t, err)
	})

	t.Run("can insert transaction with just payment hash", func(t *testing.T) {
		hash := txtest.MockHash([]byte("a really bad preimage"))

		err := insertMockTransaction(nil, &hash)
		assert.NoError(t, err)
	})

	t.Run("can not insert transaction with just preimage", func(t *testing.T) {
		preimage := txtest.MockPreimage()

		err := insertMockTransaction(&preimage, nil)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), ErrConstraintHashMustExistIfPreimage.Error())
	})
}

func TestTransactionsPaymentRequestMustExistForOtherFieldsToExist(t *testing.T) {
	insertMockTransaction := func(paymentRequest, memo *string, hashedPreimage *[]byte) error {
		_, err := testDB.NamedExec(`
		INSERT INTO transactions (direction, payment_request, memo, hashed_preimage)
			VALUES (:direction, :payment_request, :memo, :hashed_preimage)`,
			map[string]interface{}{
				"direction":       txtest.MockDirection(),
				"payment_request": paymentRequest,
				"memo":            memo,
				"hashed_preimage": hashedPreimage,
			},
		)
		return err
	}

	t.Run("can insert transaction with payment request and memo", func(t *testing.T) {
		paymentRequest := "pay_req"
		memo := gofakeit.HipsterSentence(3)

		err := insertMockTransaction(&paymentRequest, &memo, nil)
		assert.NoError(t, err)
	})

	t.Run("can insert transaction with payment request and hashed_preimage", func(t *testing.T) {
		paymentRequest := "pay_req"
		hashedPreimage := txtest.MockHash([]byte("a bad preimage"))

		err := insertMockTransaction(&paymentRequest, nil, &hashedPreimage)
		assert.NoError(t, err)
	})

	t.Run("can not insert transaction with just memo", func(t *testing.T) {
		memo := gofakeit.HipsterSentence(3)

		err := insertMockTransaction(nil, &memo, nil)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), ErrConstraintEitherOnChainOrOffchain.Error())
	})
	t.Run("can not insert transaction with just hashed_preimage", func(t *testing.T) {
		memo := gofakeit.HipsterSentence(3)
		hashedPreimage := txtest.MockHash([]byte("a bad preimage"))

		err := insertMockTransaction(nil, &memo, &hashedPreimage)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), ErrConstraintEitherOnChainOrOffchain.Error())
	})

	t.Run("can not insert transaction with just hashed_preimage and memo", func(t *testing.T) {
		memo := gofakeit.HipsterSentence(3)
		hashedPreimage := txtest.MockHash([]byte("a bad preimage"))

		err := insertMockTransaction(nil, &memo, &hashedPreimage)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), ErrConstraintEitherOnChainOrOffchain.Error())
	})
}

func TestTransactionsMustEitherBeOnchainOrOffchain(t *testing.T) {
	user := userstestutil.CreateUserOrFail(t, testDB)

	t.Run("can insert onchain transaction", func(t *testing.T) {
		_, err := txtest.InsertFakeOnchain(t, testDB, user.ID)
		assert.NoError(t, err)
	})
	t.Run("can insert offchain transaction", func(t *testing.T) {
		_, err := txtest.InsertFakeOffchain(t, testDB, user.ID)
		assert.NoError(t, err)
	})
	t.Run("can not insert transaction with address and payment_request", func(t *testing.T) {
		insertMockTransaction := func() error {
			paymentRequest := "fake payment request"
			_, err := testDB.NamedExec(`
		INSERT INTO transactions (direction, address, payment_request)
			VALUES (:direction, :address, :payment_request)`,
				map[string]interface{}{
					"direction":       txtest.MockDirection(),
					"address":         address,
					"payment_request": paymentRequest,
				},
			)
			return err
		}

		err := insertMockTransaction()
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), ErrConstraintEitherOnChainOrOffchain.Error())
	})

}

func TestTransactionsAmountMilliSat(t *testing.T) {
	insertMockTransaction := func(amountMilliSat int) error {
		_, err := testDB.NamedExec(`
		INSERT INTO transactions (direction, address, amount_milli_sat)
			VALUES (:direction, :address, :amount_milli_sat)`,
			map[string]interface{}{
				"direction":        txtest.MockDirection(),
				"address":          address,
				"amount_milli_sat": amountMilliSat,
			},
		)
		return err
	}

	t.Run("inserting transaction with 0 amount succeeds", func(t *testing.T) {
		err := insertMockTransaction(0)
		assert.NoError(t, err)
	})

	t.Run("inserting transaction with greater than 0 amount succeeds", func(t *testing.T) {
		amount := gofakeit.Number(1, math.MaxInt64-1)

		err := insertMockTransaction(amount)
		assert.NoError(t, err)
	})

	t.Run("inserting transaction with negative amount fails", func(t *testing.T) {
		amount := gofakeit.Number(math.MinInt64+2, -1)

		err := insertMockTransaction(amount)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), ErrConstraintPositiveAmountMilliSat.Error())
	})
}
