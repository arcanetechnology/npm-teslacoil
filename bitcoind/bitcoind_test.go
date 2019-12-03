//+build integration

package bitcoind_test

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit"
	"gitlab.com/arcanecrypto/teslacoil/ln"

	"github.com/stretchr/testify/require"

	"github.com/btcsuite/btcutil"
	"github.com/stretchr/testify/assert"

	"gitlab.com/arcanecrypto/teslacoil/async"

	"gitlab.com/arcanecrypto/teslacoil/bitcoind"

	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/testutil/bitcoindtestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/nodetestutil"
)

var (
	log = build.AddSubLogger("BTCD_INT_TEST")
)

func TestMain(m *testing.M) {
	build.SetLogLevels(logrus.ErrorLevel)

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
	bitcoin := nodetestutil.GetBitcoind(t)

	bitcoin.StartZmq()
	txCh := bitcoin.ZmqTxChannel()

	var eventsReceived int
	go func() {
		for {
			// We don't care for the result, juts the amount of events, therefore
			// we ignore the tx
			_ = <-txCh
			eventsReceived++
		}
	}()

	const blocksGenerated = 101
	_, err := bitcoindtestutil.GenerateToSelf(blocksGenerated, bitcoin)
	require.NoError(t, err)

	_, err = bitcoindtestutil.SendTxToSelf(bitcoin, 10)
	require.NoError(t, err)

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
	require.NoError(t, err, "expected to receive %d events, but received %d", 1+1+blocksGenerated, eventsReceived)
}

// TestBlockListener tests that the ZmqBlockChannel receives
// all mined blocks
func TestBlockListener(t *testing.T) {
	t.Parallel()

	bitcoin := nodetestutil.GetBitcoind(t)

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
	require.NoError(t, err, "could not generate %d blocks to self", blocksToMine)

	check := func() bool {
		return eventsReceived >= blocksToMine
	}

	err = async.AwaitNoBackoff(15, 100*time.Millisecond, check)
	require.NoError(t, err, "expected to receive at least %d events, but received %d", blocksToMine, eventsReceived)
}

func TestFindVout(t *testing.T) {
	bitcoin := nodetestutil.GetBitcoind(t)

	t.Run("can find vout for transaction", func(t *testing.T) {
		address, err := bitcoin.Btcctl().GetNewAddress("")
		require.NoError(t, err)

		amount := btcutil.Amount(gofakeit.Number(0, ln.MaxAmountSatPerInvoice))
		tx, err := bitcoin.Btcctl().SendToAddress(address, amount)
		require.NoError(t, err)

		rawTx, err := bitcoin.Btcctl().GetRawTransactionVerbose(tx)
		require.NoError(t, err)

		var correctVout uint32
		for _, output := range rawTx.Vout {
			outputValue, err := btcutil.NewAmount(output.Value)
			require.NoError(t, err)

			if amount == outputValue {
				correctVout = output.N
			}
		}

		vout, err := bitcoin.FindVout(tx.String(), address.String())
		assert.NoError(t, err)
		if !assert.Equal(t, correctVout, uint32(vout)) {
			log.WithFields(logrus.Fields{
				"amount":  amount,
				"address": address,
				"rawTx":   fmt.Sprintf("%+v", rawTx),
			}).Errorf("rawTx.Hex: %s", rawTx.Hex)
		}
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
		vout, err := bitcoin.FindVout("bad_txid", "fake address")
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
}
