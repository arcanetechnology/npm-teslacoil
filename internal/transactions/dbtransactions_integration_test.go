//+build integration

package transactions

import (
	"testing"
	"time"

	"gitlab.com/arcanecrypto/teslacoil/internal/platform/bitcoind"

	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/lntestutil"
)

func init() {
	// we're not closing DB connections here...
	// this probably shouldn't matter, as the conn.
	// closes when the process exits anyway
	testDB = testutil.InitDatabase(databaseConfig)
	databaseConfig = testutil.GetDatabaseConfig("transactions_integration")
}

/*

func TestNewDepositWithFields(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)
}

func TestGetOrCreateDeposit(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)
}

*/

// TestTxListener tests whether the zmqTxVhannel sends the expected amount of
// It can not run in parallell, because each new block mined also creates a
// tx, thus filling us up with tx's
func TestTxListener(t *testing.T) {
	testutil.DescribeTest(t)

	lntestutil.RunWithBitcoind(t, func(bitcoin bitcoind.TeslacoilBitcoind) {

		bitcoin.StartZmq()

		txCh := bitcoin.ZmqTxChannel()

		var eventsReceived int
		go func() {
			for {
				// We don't care for the result, juts the amount of events, therefore
				// we ignore the tx
				<-txCh
				eventsReceived++
			}
		}()

		var blocksGenerated uint32 = 101
		_, err := lntestutil.GenerateToSelf(blocksGenerated, bitcoin)
		if err != nil {
			testutil.FatalMsgf(t, "could not generate to self: %+v", err)
		}

		hash, err := lntestutil.SendTxToSelf(bitcoin, 10)
		if err != nil {
			testutil.FatalMsgf(t, "could not send tx: %+v", err)
		}
		testutil.Succeedf(t, "hash: %v", hash)

		time.Sleep(1000 * time.Millisecond)

		if eventsReceived != 2+int(blocksGenerated) {
			testutil.FatalMsgf(t, "expected to receive 2 events, but received %d", eventsReceived)
		}
	})
}

func TestBlockListener(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	lntestutil.RunWithBitcoind(t, func(bitcoin bitcoind.TeslacoilBitcoind) {

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

		var blocksToMine uint32 = 3
		_, err := lntestutil.GenerateToSelf(blocksToMine, bitcoin)
		if err != nil {
			testutil.FatalMsgf(t, "could not generate %d blocks to self", blocksToMine)
		}

		time.Sleep(1500 * time.Millisecond)

		if eventsReceived != blocksToMine {
			testutil.FatalMsgf(t, "expected to receive %d events, but received %d", blocksToMine, eventsReceived)
		}
	})

}
