package transactions_test

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/arcanecrypto/teslacoil/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/models/transactions"
	"gitlab.com/arcanecrypto/teslacoil/models/users/balance"
	"gitlab.com/arcanecrypto/teslacoil/testutil/txtest"
	"gitlab.com/arcanecrypto/teslacoil/testutil/userstestutil"

	"github.com/brianvoe/gofakeit"
	"github.com/btcsuite/btcd/chaincfg/chainhash"

	"gitlab.com/arcanecrypto/teslacoil/ln"
	"gitlab.com/arcanecrypto/teslacoil/testutil/lntestutil"

	"github.com/lightningnetwork/lnd/lnrpc"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
)

var (
	databaseConfig = testutil.GetDatabaseConfig("transactions")
	testDB         = testutil.InitDatabase(databaseConfig)
	testnetAddress = "tb1q40gzxjcamcny49st7m8lyz9rtmssjgfefc33at"
)

func init() {
	gofakeit.Seed(0)
}

func TestInsertOnchainTransaction(t *testing.T) {
	t.Parallel()
	user := userstestutil.CreateUserOrFail(t, testDB)
	for i := 0; i < 20; i++ {
		t.Run("inserting arbitrary onchain "+strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			onchain := txtest.MockOnchain(user.ID)

			inserted, err := transactions.InsertOnchain(testDB, onchain)
			require.NoError(t, err, onchain)

			onchain.CreatedAt = inserted.CreatedAt
			onchain.UpdatedAt = inserted.UpdatedAt
			if onchain.SettledAt != nil {
				assert.WithinDuration(t, *onchain.SettledAt, *inserted.SettledAt, time.Millisecond*300)
				onchain.SettledAt = inserted.SettledAt
			}

			if onchain.ConfirmedAt != nil {
				assert.WithinDuration(t, *onchain.ConfirmedAt, *inserted.ConfirmedAt, time.Millisecond*500)
				onchain.ConfirmedAt = inserted.ConfirmedAt
			}

			if onchain.ReceivedMoneyAt != nil {
				assert.WithinDuration(t, *onchain.ReceivedMoneyAt, *inserted.ReceivedMoneyAt, time.Millisecond*100)
				onchain.ReceivedMoneyAt = inserted.ReceivedMoneyAt
			}

			// ID should be created by DB for us
			testutil.AssertNotEqual(t, onchain.ID, inserted.ID)
			onchain.ID = inserted.ID
			diff := cmp.Diff(onchain, inserted)
			if diff != "" {
				testutil.FatalMsg(t, diff)
			}

			foundTx, err := transactions.GetTransactionByID(testDB, inserted.ID, user.ID)
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

			allTXs, err := transactions.GetAllTransactions(testDB, user.ID)
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
			offchain := txtest.MockOffchain(user.ID)

			inserted, err := transactions.InsertOffchain(testDB, offchain)
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

			foundTx, err := transactions.GetTransactionByID(testDB, inserted.ID, user.ID)
			if err != nil {
				testutil.FatalMsg(t, err)
			}

			foundOffChain, err := foundTx.ToOffchain()
			if err != nil {
				testutil.FatalMsg(t, err)
			}

			diff = cmp.Diff(foundOffChain, inserted)
			if diff != "" {
				testutil.FatalMsg(t, diff)
			}

			allTXs, err := transactions.GetAllTransactions(testDB, user.ID)
			if err != nil {
				testutil.FatalMsg(t, err)
			}
			found := false
			for _, tx := range allTXs {
				off, err := tx.ToOffchain()
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

	const email1 = "email1@example.com"
	const password1 = "password1"
	const email2 = "email2@example.com"
	const password2 = "password2"
	amount1 := rand.Int63n(ln.MaxAmountMsatPerInvoice)
	address1 := gofakeit.Word()
	amount2 := rand.Int63n(ln.MaxAmountMsatPerInvoice)
	address2 := gofakeit.Word()

	user := userstestutil.CreateUserOrFail(t, testDB)

	foo := "foo"
	testCases := []struct {
		email          string
		password       string
		expectedResult transactions.Transaction
	}{
		{

			email1,
			password1,
			transactions.Transaction{
				UserID:         user.ID,
				AmountMilliSat: &amount1,
				Address:        &address1,
				Description:    &foo,
				Direction:      transactions.INBOUND,
			},
		},
		{

			email2,
			password2,
			transactions.Transaction{
				UserID:         user.ID,
				AmountMilliSat: &amount2,
				Address:        &address2,
				Description:    &foo,
				Direction:      transactions.INBOUND,
			},
		},
	}

	for _, test := range testCases {
		t.Run(fmt.Sprintf("GetTransactionByID() for transaction with amount %d", test.expectedResult.AmountMilliSat),
			func(t *testing.T) {
				t.Parallel()

				var transaction transactions.Transaction
				if onchain, err := test.expectedResult.ToOnchain(); err == nil {
					inserted, err := transactions.InsertOnchain(testDB, onchain)
					require.NoError(t, err)
					transaction = inserted.ToTransaction()
				} else if offchain, err := test.expectedResult.ToOffchain(); err == nil {
					inserted, err := transactions.InsertOffchain(testDB, offchain)
					require.NoError(t, err)
					transaction = inserted.ToTransaction()
				} else {
					require.FailNow(t, "Not onchain nor offchain", test.expectedResult)
				}

				transaction, err := transactions.GetTransactionByID(testDB, transaction.ID, test.expectedResult.UserID)
				require.NoError(t, err)

				test.expectedResult.ID = transaction.ID
				assert.Equal(t, transaction.Address, test.expectedResult.Address)
				if test.expectedResult.AmountMilliSat != nil {
					require.NotNil(t, transaction.AmountMilliSat)
					assert.InDelta(t, *transaction.AmountMilliSat, *test.expectedResult.AmountMilliSat, 1000)
				}
				assert.Equal(t, transaction.Direction, test.expectedResult.Direction)
				assert.Equal(t, transaction.Description, test.expectedResult.Description)
				assert.Equal(t, transaction.UserID, test.expectedResult.UserID)
			})
	}
}

func TestWithdrawOnChain(t *testing.T) {
	t.Parallel()

	mockBitcoin := bitcoind.TeslacoilBitcoindMockClient{}

	t.Run("ignores amount and withdraws all the users balance", func(t *testing.T) {

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
				balance:   500,
				amountSat: 0,
			},
		}

		for _, test := range testCases {
			user := CreateUserWithBalanceOrFail(t, testDB, test.balance)
			mockLNcli := lntestutil.LightningMockClient{
				SendCoinsResponse: lnrpc.SendCoinsResponse{
					Txid: txtest.MockTxid(),
				},
			}
			_, err := transactions.WithdrawOnChain(testDB, mockLNcli, mockBitcoin, transactions.WithdrawOnChainArgs{
				UserID:    user.ID,
				AmountSat: test.amountSat,
				Address:   testnetAddress,
				SendAll:   true,
			})
			require.NoError(t, err)

			bal, err := balance.ForUser(testDB, user.ID)
			require.NoError(t, err)
			assert.Equal(t, int64(0), bal.MilliSats())

		}
	})

	const maxSats = btcutil.SatoshiPerBitcoin * 1000
	for i := 0; i < 5; i++ {
		t.Run("withdrawing should decrease users balance no."+strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()

			mockLNcli := lntestutil.LightningMockClient{
				SendCoinsResponse: lnrpc.SendCoinsResponse{
					Txid: txtest.MockTxid(),
				},
			}
			initial := gofakeit.Number(1337, maxSats)
			user := CreateUserWithBalanceOrFail(t, testDB, int64(initial))

			withdrawAmount := gofakeit.Number(1, initial)
			_, err := transactions.WithdrawOnChain(testDB, mockLNcli, mockBitcoin, transactions.WithdrawOnChainArgs{
				UserID:    user.ID,
				AmountSat: int64(withdrawAmount),
				Address:   "fooooooo",
			})

			require.NoError(t, err)

			balanceAfter, err := balance.ForUser(testDB, user.ID)
			require.NoError(t, err)

			assert.Equal(t, balanceAfter.Sats(), int64(initial-withdrawAmount))

		})
	}

	t.Run("withdraw more than balance fails", func(t *testing.T) {
		user := CreateUserWithBalanceOrFail(t, testDB,
			500)

		mockLNcli := lntestutil.LightningMockClient{
			SendCoinsResponse: lnrpc.SendCoinsResponse{
				Txid: txtest.MockTxid(),
			},
		}
		_, err := transactions.WithdrawOnChain(testDB, mockLNcli, mockBitcoin, transactions.WithdrawOnChainArgs{
			UserID:    user.ID,
			AmountSat: 5000,
			Address:   testnetAddress,
		})

		assert.Equal(t, err, transactions.ErrBalanceTooLow)
	})
	t.Run("withdraw negative amount fails", func(t *testing.T) {
		user := CreateUserWithBalanceOrFail(t, testDB, 500)
		mockLNcli := lntestutil.LightningMockClient{
			SendCoinsResponse: lnrpc.SendCoinsResponse{
				Txid: txtest.MockTxid(),
			},
		}

		_, err := transactions.WithdrawOnChain(testDB, mockLNcli, mockBitcoin, transactions.WithdrawOnChainArgs{
			UserID:    user.ID,
			AmountSat: -5000,
			Address:   testnetAddress,
		})

		assert.Equal(t, err, transactions.ErrNonPositiveAmount)
	})
	t.Run("withdraw 0 amount fails", func(t *testing.T) {
		user := CreateUserWithBalanceOrFail(t, testDB, 500)
		mockLNcli := lntestutil.LightningMockClient{
			SendCoinsResponse: lnrpc.SendCoinsResponse{
				Txid: txtest.MockTxid(),
			},
		}
		_, err := transactions.WithdrawOnChain(testDB, mockLNcli, mockBitcoin, transactions.WithdrawOnChainArgs{
			UserID:    user.ID,
			AmountSat: 0,
			Address:   testnetAddress,
		})

		assert.Equal(t, err, transactions.ErrNonPositiveAmount)
	})

	t.Run("inserting bad txid fails", func(t *testing.T) {
		mockLNcli := lntestutil.LightningMockClient{
			SendCoinsResponse: lnrpc.SendCoinsResponse{
				Txid: "I am a bad txid",
			},
		}
		user := CreateUserWithBalanceOrFail(t, testDB, 10000)

		_, err := transactions.WithdrawOnChain(testDB, mockLNcli, mockBitcoin, transactions.WithdrawOnChainArgs{
			UserID:    user.ID,
			AmountSat: 5000,
			Address:   testnetAddress,
			SendAll:   true,
		})

		require.Error(t, err)
	})
}

func TestOnchain_AddReceivedMoney(t *testing.T) {
	t.Parallel()
	user := userstestutil.CreateUserOrFail(t, testDB)

	t.Run("should fail to save negative amount", func(t *testing.T) {
		t.Parallel()
		transaction := CreateOnChainOrFail(t, user.ID)
		if _, err := transaction.PersistReceivedMoney(testDB,
			chainhash.HashH([]byte("some hash")),
			1,
			-1337,
		); err == nil {
			testutil.FatalMsg(t, "Was able to add negative amount")
		}
	})

	t.Run("should fail to save negative vout", func(t *testing.T) {
		t.Parallel()
		transaction := CreateOnChainOrFail(t, user.ID)
		if _, err := transaction.PersistReceivedMoney(testDB,
			chainhash.HashH([]byte("some hash")),
			-1,
			1337,
		); err == nil {
			testutil.FatalMsg(t, "Was able to add negative vout")
		}
	})

	t.Run("add money", func(t *testing.T) {
		t.Parallel()
		const amountSat = 1337

		hash, err := chainhash.NewHash([]byte(testutil.MockStringOfLength(32)))
		if err != nil {
			testutil.FatalMsgf(t, "should be able to create hash: %+v", err)
		}

		transaction := CreateOnChainOrFail(t, user.ID)
		if _, err := transaction.PersistReceivedMoney(testDB, *hash, 0, amountSat); err != nil {
			testutil.FatalMsgf(t, "SaveTxToTransaction(): %+v", err)
		}

		found, err := transactions.GetTransactionByID(testDB, transaction.ID, transaction.UserID)
		if err != nil {
			testutil.FatalMsgf(t, "should be able to GetTransactionByID: %+v", err)
		}

		foundOnChain, err := found.ToOnchain()
		if err != nil {
			testutil.FatalMsg(t, err)
		}

		require.NotNil(t, foundOnChain.Vout)
		assert.Equal(t, *foundOnChain.Vout, 0)

		require.NotNil(t, foundOnChain.Txid)
		assert.Equal(t, *foundOnChain.Txid, hash.String())

		require.NotNil(t, foundOnChain.AmountSat)
		assert.Equal(t, *foundOnChain.AmountSat, int64(amountSat))

	})

	t.Run("if TX has money spent to it, should fail to receive money again", func(t *testing.T) {
		t.Parallel()

		transaction := CreateOnChainOrFail(t, user.ID)
		hash := chainhash.HashH([]byte("hm a hash"))
		const vout = 0
		const amountSat = 901237

		once, err := transaction.PersistReceivedMoney(testDB, hash, vout, amountSat)
		assert.NoError(t, err)

		require.NotNil(t, once.AmountSat)
		require.NotNil(t, once.Txid)
		require.NotNil(t, once.Vout)
		assert.Equal(t, *once.AmountSat, int64(amountSat))
		assert.Equal(t, *once.Txid, hash.String())
		assert.Equal(t, *once.Vout, vout)

		otherHash := chainhash.HashH([]byte("another hash"))
		_, err = transaction.PersistReceivedMoney(testDB, otherHash, vout, amountSat)
		assert.Error(t, err)
	})
}

func TestOnchain_MarkAsConfirmed(t *testing.T) {
	t.Parallel()

	t.Run("should fail to mark a TX as confirmed if it hasn't received any money", func(t *testing.T) {
		t.Parallel()
		user := userstestutil.CreateUserOrFail(t, testDB)
		transaction := CreateOnChainOrFail(t, user.ID)

		const confHeight = 100
		if _, err := transaction.MarkAsConfirmed(testDB, confHeight); err == nil {
			testutil.FatalMsgf(t, "marked TX as confirmed without spending any money to it!")
		}
	})

	t.Run("should mark transaction as confirmed and set confirmedAt", func(t *testing.T) {
		user := userstestutil.CreateUserOrFail(t, testDB)
		transaction := CreateOnChainOrFail(t, user.ID)

		spent, err := transaction.PersistReceivedMoney(testDB,
			chainhash.HashH([]byte("foobar")),
			0,
			1337,
		)
		if err != nil {
			testutil.FatalMsg(t, err)
		}

		const confHeight = 100
		confirmed, err := spent.MarkAsConfirmed(testDB, confHeight)
		if err != nil {
			testutil.FatalMsgf(t, "could not mark as confirmed: %+v", err)
		}

		found, err := transactions.GetTransactionByID(testDB, confirmed.ID, user.ID)
		if err != nil {
			testutil.FatalMsg(t, err)
		}

		foundOnChain, err := found.ToOnchain()
		if err != nil {
			testutil.FatalMsg(t, err)
		}

		if foundOnChain.ConfirmedAt == nil {
			testutil.FatalMsgf(t, "ConfirmedAt should have a value")
		}

		testutil.AssertNotEqual(t, foundOnChain.ConfirmedAtBlock, nil)
		testutil.AssertEqual(t, *foundOnChain.ConfirmedAtBlock, confHeight)

		// we received some money
		transaction.AmountSat = spent.AmountSat
		transaction.Txid = spent.Txid
		transaction.Vout = spent.Vout
		transaction.ReceivedMoneyAt = spent.ReceivedMoneyAt

		// we confirmed the TX
		transaction.ConfirmedAtBlock = confirmed.ConfirmedAtBlock
		assert.WithinDuration(t, *foundOnChain.ConfirmedAt, *confirmed.ConfirmedAt, time.Second)
		transaction.ConfirmedAt = foundOnChain.ConfirmedAt

		// but apart from that they should be the same
		diff := cmp.Diff(foundOnChain, transaction)
		if diff != "" {
			testutil.FatalMsg(t, diff)
		}
	})
}

func TestNewDepositWithMoney(t *testing.T) {
	user := userstestutil.CreateUserOrFail(t, testDB)
	const expectedAddr = "mw3gPvWixiiShySrr87igcSubQc9TPqUGV"
	const scriptPubkey = "76a914aa59828a194ddef9d1d4244000f0d3636c1bb78888ac"
	spkBytes, err := hex.DecodeString(scriptPubkey)
	require.NoError(t, err)

	sats := int64(gofakeit.Number(1, btcutil.SatoshiPerBitcoin*100))
	out := wire.NewTxOut(sats, spkBytes)
	tx := wire.NewMsgTx(0)
	tx.AddTxOut(out)

	onchain, err := transactions.NewDepositWithMoney(testDB, transactions.WithMoneyArgs{
		Tx:          tx,
		OutputIndex: 0,
		UserID:      user.ID,
		Chain:       chaincfg.RegressionNetParams,
	})

	require.NoError(t, err)

	assert.Equal(t, expectedAddr, onchain.Address)

	// this should have money spent to it
	require.NotNil(t, onchain.Txid)
	require.NotNil(t, onchain.Vout)
	require.NotNil(t, onchain.AmountSat)
	assert.Equal(t, tx.TxHash().String(), *onchain.Txid)
	assert.Equal(t, 0, *onchain.Vout)
	assert.Equal(t, out.Value, *onchain.AmountSat)

	assert.Equal(t, user.ID, onchain.UserID)

	foundTx, err := transactions.GetOnchainByID(testDB, onchain.ID, user.ID)
	require.NoError(t, err)

	assert.Nil(t, foundTx.ConfirmedAt)
	assert.Nil(t, foundTx.ConfirmedAtBlock)
}

func TestNewDeposit(t *testing.T) {
	mockLn := lntestutil.GetLightningMockClient()
	user := userstestutil.CreateUserOrFail(t, testDB)

	onchain, err := transactions.NewDeposit(testDB, mockLn, user.ID)
	require.NoError(t, err)

	// this should have not money spent to it
	assert.Nil(t, onchain.Txid)
	assert.Nil(t, onchain.Vout)
	assert.Nil(t, onchain.AmountSat)

	assert.Equal(t, user.ID, onchain.UserID)

	require.Nil(t, onchain.Description)
}

func TestNewDepositWithDescription(t *testing.T) {
	mockLn := lntestutil.GetLightningMockClient()
	user := userstestutil.CreateUserOrFail(t, testDB)
	description := gofakeit.Sentence(gofakeit.Number(1, 12))

	onchain, err := transactions.NewDepositWithDescription(testDB, mockLn, user.ID, description)
	require.NoError(t, err)

	// this should have not money spent to it
	assert.Nil(t, onchain.Txid)
	assert.Nil(t, onchain.Vout)
	assert.Nil(t, onchain.AmountSat)

	assert.Equal(t, user.ID, onchain.UserID)

	require.NotNil(t, onchain.Description)
	assert.Equal(t, description, *onchain.Description)
}

func TestGetOrCreateDeposit(t *testing.T) {
	t.Parallel()

	mockLn := lntestutil.GetRandomLightningMockClient()

	t.Run("can get latest address", func(t *testing.T) {
		t.Parallel()
		user := userstestutil.CreateUserOrFail(t, testDB)
		tx := txtest.InsertFakeIncomingWithoutTxidOnchainorFail(t, testDB, user.ID)

		deposit, err := transactions.GetOrCreateDeposit(
			testDB, mockLn, user.ID, false, "")
		require.NoError(t, err)
		assert.Equal(t, deposit, tx)
	})

	t.Run("can create new deposit if user has no empty address", func(t *testing.T) {
		t.Parallel()
		user := userstestutil.CreateUserOrFail(t, testDB)

		deposit, err := transactions.GetOrCreateDeposit(
			testDB, mockLn, user.ID, false, "")
		require.NoError(t, err)
		assert.Equal(t, deposit.UserID, user.ID)
	})

	t.Run("forceNewAddress always returns a new address", func(t *testing.T) {
		t.Parallel()
		user := userstestutil.CreateUserOrFail(t, testDB)
		tx, err := txtest.InsertFakeOnchainWithoutTxid(t, testDB, user.ID)
		require.NoError(t, err)

		deposit1, err := transactions.GetOrCreateDeposit(
			testDB, mockLn, user.ID, true, "")
		require.NoError(t, err)
		assert.NotEqual(t, deposit1.Address, tx.Address)

		deposit2, err := transactions.GetOrCreateDeposit(
			testDB, mockLn, user.ID, true, "")
		require.NoError(t, err)
		assert.NotEqual(t, deposit2.Address, deposit1.Address)
	})

	t.Run("can get latest when user only has offchain transactions", func(t *testing.T) {
		t.Parallel()
		user := userstestutil.CreateUserOrFail(t, testDB)

		_ = txtest.InsertFakeOffChainOrFail(t, testDB, user.ID)

		deposit, err := transactions.GetOrCreateDeposit(
			testDB, mockLn, user.ID, true, "")
		assert.NoError(t, err)
		assert.Equal(t, deposit.UserID, user.ID)
		assert.NotEmpty(t, deposit.Address)

	})

}

func CreateOnChainOrFail(t *testing.T, userID int) transactions.Onchain {

	bar := "bar"
	tx := transactions.Onchain{
		UserID:      userID,
		Address:     "foo",
		Description: &bar,
		Direction:   transactions.INBOUND,
	}

	inserted, err := transactions.InsertOnchain(testDB, tx)

	if err != nil {
		testutil.FatalMsgf(t, "should be able to insertTransaction. Error:  %+v",
			err)
	}

	return inserted
}
