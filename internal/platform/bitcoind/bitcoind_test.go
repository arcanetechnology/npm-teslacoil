//+build integration

package bitcoind_test

import (
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/asyncutil"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/bitcoindtestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/nodetestutil"
)

var log = build.Log

func TestMain(m *testing.M) {
	build.SetLogLevel(logrus.DebugLevel)
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

		err = asyncutil.Await(8, 100*time.Millisecond, check)
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

		err = asyncutil.Await(15, 100*time.Millisecond, check)
		if err != nil {
			testutil.FatalMsgf(t, "expected to receive %d events, but received %d", blocksToMine, eventsReceived)
		}

	})

}
