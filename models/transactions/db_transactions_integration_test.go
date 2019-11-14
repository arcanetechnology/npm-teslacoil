//+build integration

package transactions_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/brianvoe/gofakeit"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/arcanecrypto/teslacoil/async"
	"gitlab.com/arcanecrypto/teslacoil/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/ln"
	"gitlab.com/arcanecrypto/teslacoil/models/transactions"
	"gitlab.com/arcanecrypto/teslacoil/models/users/balance"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/bitcoindtestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/nodetestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/userstestutil"
)

var log = build.AddSubLogger("TXNS_INT_TEST")

func TestMain(m *testing.M) {
	build.SetLogLevels(logrus.ErrorLevel)
	code := m.Run()

	if err := nodetestutil.CleanupNodes(); err != nil {
		panic(err.Error())
	}

	os.Exit(code)
}

// until we implement pooling of LND nodes we test everything in the same test, to
// avoid creating too many LND nodes
func TestEverything(t *testing.T) {
	t.Parallel()
	nodetestutil.RunWithBitcoindAndLndPair(t, func(lnd, lnd2 lnrpc.LightningClient, bitcoin bitcoind.TeslacoilBitcoind) {
		t.Run("create invoice", func(t *testing.T) {
			t.Parallel()
			amount := int64(gofakeit.Number(1, ln.MaxAmountSatPerInvoice))
			invoice, err := transactions.CreateInvoice(lnd, amount)
			require.NoError(t, err)
			assert.Equal(t, "", invoice.Memo)
			assert.Equal(t, amount, invoice.Value)
		})

		t.Run("create invoice with memo", func(t *testing.T) {
			t.Parallel()
			amount := int64(gofakeit.Number(1, ln.MaxAmountSatPerInvoice))
			memo := gofakeit.Sentence(gofakeit.Number(1, 12))
			invoice, err := transactions.CreateInvoiceWithMemo(lnd, amount, memo)
			require.NoError(t, err)
			assert.Equal(t, memo, invoice.Memo)
			assert.Equal(t, amount, invoice.Value)
		})

		t.Run("fail to create invoice with too large amount", func(t *testing.T) {
			t.Parallel()
			amount := int64(gofakeit.Number(1, ln.MaxAmountSatPerInvoice))
			_, err := transactions.CreateInvoice(lnd, amount+ln.MaxAmountSatPerInvoice)
			require.Error(t, err)
		})

		t.Run("fail to create invoice with too large memo", func(t *testing.T) {
			t.Parallel()
			amount := int64(gofakeit.Number(1, ln.MaxAmountSatPerInvoice))
			memo := gofakeit.Sentence(gofakeit.Number(100, 300))
			inv, err := transactions.CreateInvoiceWithMemo(lnd, amount, memo)
			require.Error(t, err, inv)
		})

		t.Run("InvoiceListener should pick up a paid invoice and POST to the callback URL", func(t *testing.T) {
			t.Parallel()
			poster := testutil.GetMockHttpPoster()
			user := userstestutil.CreateUserOrFail(t, testDB)
			channel := make(chan *lnrpc.Invoice)
			go transactions.InvoiceListener(channel, testDB, poster)

			inserted := CreateNewOffchainTxOrFail(t, testDB, lnd, transactions.NewOffchainOpts{
				UserID:      user.ID,
				AmountSat:   int64(gofakeit.Number(1, ln.MaxAmountSatPerInvoice)),
				CallbackURL: gofakeit.URL(),
			})

			payment, err := lnd2.SendPaymentSync(context.Background(), &lnrpc.SendRequest{
				PaymentRequest: inserted.PaymentRequest,
			})

			require.NoError(t, err)
			require.Equal(t, "", payment.PaymentError)

			inv, err := lnd.LookupInvoice(context.Background(), &lnrpc.PaymentHash{
				RHash: payment.PaymentHash,
			})

			require.NoError(t, err)
			channel <- inv

			err = async.RetryNoBackoff(10, time.Second, func() error {
				posts := poster.GetSentPostRequests()
				log.Error("posts ", posts)
				if posts != 1 {
					return fmt.Errorf("unexpected number of HTTP POSTs sent: %d", posts)
				}
				return nil
			})
			require.NoError(t, err)

			fetchedTx, err := transactions.GetOffchainByID(testDB, inserted.ID, user.ID)
			require.NoError(t, err)
			assert.NotNil(t, fetchedTx.SettledAt)
			assert.Equal(t, transactions.Offchain_COMPLETED, fetchedTx.Status)

		})

		t.Run("SendOnChain should result in a TX happening on the blockchain", func(t *testing.T) {
			const amount = 1337
			address, err := bitcoin.Btcctl().GetNewAddress("")
			require.NoError(t, err)

			txid, err := transactions.SendOnChain(lnd, &lnrpc.SendCoinsRequest{
				Addr:   address.String(),
				Amount: amount,
			})
			require.NoError(t, err)

			hash, err := chainhash.NewHashFromStr(txid)
			require.NoError(t, err)

			tx, err := bitcoin.Btcctl().GetRawTransactionVerbose(hash)
			require.NoError(t, err)

			var found bool
			for _, vout := range tx.Vout {
				value, err := btcutil.NewAmount(vout.Value)
				require.NoError(t, err)

				if value == btcutil.Amount(amount) {
					found = true
				}
			}
			assert.Truef(t, found, "Did not find vout in TX that got sent! Vouts: %v", tx.Vout)
		})

		t.Run("WithdrawOnChain should result in a TX on the blockchain and decrease the users balance", func(t *testing.T) {
			initialSats := gofakeit.Number(1000, ln.MaxAmountSatPerInvoice)
			withdrawAmt := int64(gofakeit.Number(1000, initialSats))
			address, err := bitcoin.Btcctl().GetNewAddress("")
			require.NoError(t, err)

			user := userstestutil.CreateUserWithBalanceOrFail(t, testDB, initialSats)
			balancePre, err := balance.ForUser(testDB, user.ID)
			require.NoError(t, err)
			assert.Equal(t, balancePre.Sats(), int64(initialSats))

			onchain, err := transactions.WithdrawOnChain(testDB, lnd, bitcoin, transactions.WithdrawOnChainArgs{
				UserID:    user.ID,
				AmountSat: withdrawAmt,
				Address:   address.String(),
			})
			require.NoError(t, err)

			assert.NotNil(t, withdrawAmt, *onchain.AmountSat)
			assert.Equal(t, withdrawAmt, *onchain.AmountSat)
			assert.Equal(t, address.String(), onchain.Address)
			assert.NotNil(t, onchain.Txid)

			hash, err := chainhash.NewHashFromStr(*onchain.Txid)
			require.NoError(t, err)

			tx, err := bitcoin.Btcctl().GetRawTransactionVerbose(hash)
			require.NoError(t, err)

			var found bool
			for _, vout := range tx.Vout {
				value, err := btcutil.NewAmount(vout.Value)
				require.NoError(t, err)

				if value == btcutil.Amount(withdrawAmt) {
					found = true
				}
			}
			assert.Truef(t, found, "Did not find vout in TX that got sent! Vouts: %v", tx.Vout)

			bal, err := balance.ForUser(testDB, user.ID)
			require.NoError(t, err)

			assert.Equal(t, bal.Sats(), int64(initialSats)-withdrawAmt)

		})

		t.Run("BlockListener should pick up new blocks and confirm user deposits", func(t *testing.T) {
			t.Parallel()

			user := userstestutil.CreateUserOrFail(t, testDB)
			deposit, err := transactions.NewDeposit(testDB, lnd, user.ID)
			require.NoError(t, err)

			blockChan := make(chan *wire.MsgBlock)
			txChan := make(chan *wire.MsgTx)
			// block listener needs TX listener to pick up the transactions first
			go transactions.TxListener(testDB, txChan, chaincfg.RegressionNetParams)
			go transactions.BlockListener(testDB, bitcoin.Btcctl(), blockChan)

			sats := gofakeit.Number(1000, btcutil.SatoshiPerBitcoin*5)

			addr, err := btcutil.DecodeAddress(deposit.Address, &chaincfg.RegressionNetParams)
			require.NoError(t, err)

			txid, err := bitcoin.Btcctl().SendToAddress(addr, btcutil.Amount(sats))
			require.NoError(t, err)

			tx, err := bitcoin.Btcctl().GetRawTransaction(txid)
			require.NoError(t, err)

			txChan <- tx.MsgTx()

			checkTx := func() error {
				foundTx, err := transactions.GetOnchainByID(testDB, deposit.ID, user.ID)
				if err != nil {
					return err
				}
				if foundTx.Txid == nil {
					return errors.New("deposit hasn't been credited")
				}
				return nil
			}
			err = async.RetryNoBackoff(10, time.Millisecond*100, checkTx)
			require.NoError(t, err)

			hashes, err := bitcoindtestutil.GenerateToSelf(6, bitcoin)
			require.NoError(t, err)
			require.NotEmpty(t, hashes)

			lastBlock := hashes[len(hashes)-1]
			block, err := bitcoin.Btcctl().GetBlock(lastBlock)
			require.NoError(t, err)

			blockChan <- block

			var foundTx transactions.Onchain
			checkConfirmedTx := func() error {
				foundTx, err = transactions.GetOnchainByID(testDB, deposit.ID, user.ID)
				if err != nil {
					return err
				}
				if foundTx.ConfirmedAt == nil {
					return errors.New("deposit hasn't been confirmed")
				}
				return nil
			}
			err = async.RetryNoBackoff(10, time.Millisecond*100, checkConfirmedTx)
			require.NoError(t, err)

			require.NotNil(t, foundTx.ConfirmedAt)
			require.NotNil(t, foundTx.ConfirmedAtBlock)
			assert.WithinDuration(t, *foundTx.ConfirmedAt, time.Now(), time.Second)

			userBalance, err := balance.ForUser(testDB, user.ID)
			require.NoError(t, err)
			assert.Equal(t, userBalance.Sats(), int64(sats))
		})

		t.Run("TxListener should pick up a users incoming TX and update DB state", func(t *testing.T) {
			t.Parallel()

			user := userstestutil.CreateUserOrFail(t, testDB)
			deposit, err := transactions.NewDeposit(testDB, lnd, user.ID)
			require.NoError(t, err)

			txChan := make(chan *wire.MsgTx)
			go transactions.TxListener(testDB, txChan, chaincfg.RegressionNetParams)

			sats := gofakeit.Number(1000, btcutil.SatoshiPerBitcoin*5)

			addr, err := btcutil.DecodeAddress(deposit.Address, &chaincfg.RegressionNetParams)
			require.NoError(t, err)

			txid, err := bitcoin.Btcctl().SendToAddress(addr, btcutil.Amount(sats))
			require.NoError(t, err)

			tx, err := bitcoin.Btcctl().GetRawTransaction(txid)
			require.NoError(t, err)

			txChan <- tx.MsgTx()

			var foundTx transactions.Onchain
			checkTx := func() error {
				foundTx, err = transactions.GetOnchainByID(testDB, deposit.ID, user.ID)
				if err != nil {
					return err
				}
				if foundTx.Txid == nil {
					return errors.New("deposit hasn't been credited")
				}
				return nil
			}
			err = async.RetryNoBackoff(10, time.Millisecond*100, checkTx)
			require.NoError(t, err)

			require.NotNil(t, foundTx.Txid)
			require.NotNil(t, foundTx.AmountSat)
			require.NotNil(t, foundTx.Vout)
			require.NotNil(t, foundTx.ReceivedMoneyAt)

			assert.Equal(t, int64(sats), *foundTx.AmountSat)
			assert.Equal(t, txid.String(), *foundTx.Txid)
			assert.WithinDuration(t, time.Now(), *foundTx.ReceivedMoneyAt, time.Millisecond*300)
		})
	})
}
