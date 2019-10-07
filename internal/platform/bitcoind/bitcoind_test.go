//+build integration

package bitcoind

import (
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/build"

	"gitlab.com/arcanecrypto/teslacoil/asyncutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
)

func TestMain(m *testing.M) {
	build.SetLogLevel(logrus.DebugLevel)
	os.Exit(m.Run())
}

// TestTxListener tests whether the zmqTxVhannel sends the expected amount of
// It can not run in parallell, because each new block mined also creates a
// tx, thus filling us up with tx's
func TestTxListener(t *testing.T) {
	testutil.DescribeTest(t)

	RunWithBitcoind(t, func(bitcoin TeslacoilBitcoind) {

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
		_, err := GenerateToSelf(blocksGenerated, bitcoin)
		if err != nil {
			testutil.FatalMsgf(t, "could not generate to self: %+v", err)
		}

		hash, err := SendTxToSelf(bitcoin, 10)
		if err != nil {
			testutil.FatalMsgf(t, "could not send tx: %+v", err)
		}
		testutil.Succeedf(t, "hash: %v", hash)

		check := func() bool {
			// For some reason the channel receives a tx with one input every time it connects
			// without sending a tx or generating a block. Therefore we add 1
			const mysteriousTx = 1
			if eventsReceived != 1+mysteriousTx+blocksGenerated {
				return false
			}
			return true
		}

		err = asyncutil.Await(3, 500*time.Millisecond, check)
		if err != nil {
			testutil.FatalMsgf(t, "expected to receive %d events, but received %d", 1+1+blocksGenerated, eventsReceived)
		}
		time.Sleep(1000 * time.Millisecond)

	})
}

// TestBlockListener tests that the ZmqBlockChannel receives
// all mined blocks
func TestBlockListener(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	RunWithBitcoind(t, func(bitcoin TeslacoilBitcoind) {

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
		_, err := GenerateToSelf(blocksToMine, bitcoin)
		if err != nil {
			testutil.FatalMsgf(t, "could not generate %d blocks to self", blocksToMine)
		}

		check := func() bool {
			if eventsReceived != blocksToMine {
				return false
			}
			return true
		}

		err = asyncutil.Await(3, 500*time.Millisecond, check)
		if err != nil {
			testutil.FatalMsgf(t, "expected to receive %d events, but received %d", blocksToMine, eventsReceived)
		}

	})

}

// TestStartBitcoindOrFail tests that a test config can connect to,
// start and GetBlockchainInfo from the bitcoind rpc connection
func TestStartBitcoindOrFail(t *testing.T) {
	conf := GetBitcoindConfig(t)
	client, cleanup := StartBitcoindOrFail(t, conf)
	_, err := client.Btcctl().GetBlockChainInfo()
	if err != nil {
		testutil.FatalMsgf(t, "Could not start and communicate with bitcoind: %v", err)
	}

	if err := cleanup(); err != nil {
		testutil.FatalMsg(t, err)
	}

	if info, err := client.Btcctl().GetBlockChainInfo(); err == nil {
		testutil.FatalMsgf(t, "Got info from stopped client: %v", info)
	}
}
