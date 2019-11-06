//+build integration

package bitcoind_test

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit"
	"github.com/go-errors/errors"
	"gitlab.com/arcanecrypto/teslacoil/ln"

	"github.com/stretchr/testify/require"

	"github.com/btcsuite/btcutil"
	"github.com/stretchr/testify/assert"

	"gitlab.com/arcanecrypto/teslacoil/db"

	"gitlab.com/arcanecrypto/teslacoil/async"

	"gitlab.com/arcanecrypto/teslacoil/bitcoind"

	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/bitcoindtestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/nodetestutil"
)

var (
	log            = build.Log
	databaseConfig = testutil.GetDatabaseConfig("bitcoind")
	testDB         *db.DB
)

func TestMain(m *testing.M) {
	build.SetLogLevel(logrus.ErrorLevel)

	testDB = testutil.InitDatabase(databaseConfig)

	code := m.Run()

	if err := nodetestutil.CleanupNodes(); err != nil {
		panic(err)
	}

	os.Exit(code)
}

// TestTxListener tests whether the zmqTxVhannel sends the expected amount of
// It can not run in parallell, because each new block mined also creates a
// tx, thus filling us up with tx's
func TestTxListener(t *testing.T) {
	testutil.DescribeTest(t)

	nodetestutil.RunWithBitcoind(t, false, func(bitcoin bitcoind.TeslacoilBitcoind) {

		bitcoin.StartZmq()

		txCh := bitcoin.ZmqTxChannel()

		var eventsReceived int
		go func() {
			for {
				// We don't care for the result, juts the amount of events, therefore
				// we ignore the tx
				tx := <-txCh
				log.Error("received tx: ", tx)
				eventsReceived++
			}
		}()

		const blocksGenerated = 101
		_, err := bitcoindtestutil.GenerateToSelf(blocksGenerated, bitcoin)
		if err != nil {
			testutil.FatalMsgf(t, "could not generate to self: %+v", err)
		}

		hash, err := bitcoindtestutil.SendTxToSelf(bitcoin, 10)
		if err != nil {
			testutil.FatalMsgf(t, "could not send tx: %+v", err)
		}
		testutil.Succeedf(t, "hash: %v", hash)

		check := func() bool {
			// For some reason the channel receives a tx with one input every time it connects
			// without sending a tx or generating a block. Therefore we add 1
			const mysteriousTx = 1
			log.Info("eventsReceived", eventsReceived)
			if eventsReceived < mysteriousTx+blocksGenerated {
				return false
			}
			return true
		}

		err = async.Await(8, 100*time.Millisecond, check)
		if err != nil {
			testutil.FatalMsgf(t, "expected to receive %d events, but received %d", 1+1+blocksGenerated, eventsReceived)
		}

	})
}

// TestBlockListener tests that the ZmqBlockChannel receives
// all mined blocks
func TestBlockListener(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	nodetestutil.RunWithBitcoind(t, false, func(bitcoin bitcoind.TeslacoilBitcoind) {

		bitcoin.StartZmq()

		blockCh := bitcoin.ZmqBlockChannel()

		var eventsReceived uint32
		go func() {
			for {
				// We don't care for the result, just the amount of events, therefore
				// we ignore the tx
				<-blockCh
				eventsReceived++
			}
		}()

		const blocksToMine = 3
		_, err := bitcoindtestutil.GenerateToSelf(blocksToMine, bitcoin)
		if err != nil {
			testutil.FatalMsgf(t, "could not generate %d blocks to self: %v", blocksToMine, err)
		}

		check := func() bool {
			if eventsReceived != blocksToMine {
				return false
			}
			return true
		}

		err = async.Await(15, 100*time.Millisecond, check)
		if err != nil {
			testutil.FatalMsgf(t, "expected to receive %d events, but received %d", blocksToMine, eventsReceived)
		}

	})

}

func TestFindVout(t *testing.T) {
	nodetestutil.RunWithBitcoind(t, true, func(bitcoin bitcoind.TeslacoilBitcoind) {

		t.Run("can find vout for transaction", func(t *testing.T) {
			address, err := bitcoin.Btcctl().GetNewAddress("")
			require.NoError(t, err)

			amount := gofakeit.Number(0, ln.MaxAmountMsatPerInvoice)
			tx, err := bitcoin.Btcctl().SendToAddress(address, btcutil.Amount(amount))
			require.NoError(t, err)

			rawTx, err := bitcoin.Btcctl().GetRawTransactionVerbose(tx)
			require.NoError(t, err)

			var correctVout uint32
			for _, output := range rawTx.Vout {
				if float64(amount) == (output.Value * btcutil.SatoshiPerBitcoin) {
					correctVout = output.N
				}
			}

			vout, err := bitcoin.FindVout(tx.String(), address.String())
			assert.NoError(t, err)
			assert.Equal(t, uint32(vout), correctVout)
		})

		t.Run("can choose correct vout from several addresses", func(t *testing.T) {
			address1, err := bitcoin.Btcctl().GetNewAddress("")
			require.NoError(t, err)
			address2, err := bitcoin.Btcctl().GetNewAddress("")
			require.NoError(t, err)

			const amount = 5001
			addresses := map[btcutil.Address]btcutil.Amount{
				address1: btcutil.Amount(5000),
				// we want to check this is selected correctly
				address2: btcutil.Amount(amount),
			}
			tx, err := bitcoin.Btcctl().SendMany("", addresses)
			require.NoError(t, err)

			vout, err := bitcoin.FindVout(tx.String(), address2.String())

			rawTx, err := bitcoin.Btcctl().GetRawTransactionVerbose(tx)
			require.NoError(t, err)

			for _, output := range rawTx.Vout {
				if output.Value*btcutil.SatoshiPerBitcoin == amount {
					assert.Equal(t, uint32(vout), output.N)
				}
			}
		})

		t.Run("passing bad txid as argument returns error", func(t *testing.T) {
			vout, err := bitcoin.FindVout("bad txid", "fake address")
			assert.True(t, errors.Is(err, bitcoind.ErrNotATxid))
			assert.Equal(t, vout, -1)
		})

		t.Run("trying to find vout for bad address returns error", func(t *testing.T) {
			address, err := bitcoin.Btcctl().GetNewAddress("")
			require.NoError(t, err)

			const amount = 5000
			tx, err := bitcoin.Btcctl().SendToAddress(address, btcutil.Amount(amount))
			require.NoError(t, err)

			vout, err := bitcoin.FindVout(tx.String(), "bad address")
			assert.Error(t, err)
			assert.Equal(t, vout, -1)
		})

	})

}
