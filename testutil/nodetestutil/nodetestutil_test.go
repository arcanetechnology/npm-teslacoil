//+build integration

package nodetestutil

import (
	"context"
	"os"
	"testing"

	"github.com/lightningnetwork/lnd/lnrpc"
	"gitlab.com/arcanecrypto/teslacoil/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/bitcoindtestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/lntestutil"
)

func TestMain(m *testing.M) {

	code := m.Run()
	if err := CleanupNodes(); err != nil {
		panic(err)
	}
	os.Exit(code)
}

func TestStartBitcoindOrFail(t *testing.T) {
	conf := bitcoindtestutil.GetBitcoindConfig(t)
	client := StartBitcoindOrFail(t, conf)
	_, err := client.Btcctl().GetBlockChainInfo()
	if err != nil {
		testutil.FatalMsgf(t, "Could not start and communicate with bitcoind: %v", err)
	}
}

func TestStartLndOrFail(t *testing.T) {
	bitcoindConf := bitcoindtestutil.GetBitcoindConfig(t)
	lndConf := lntestutil.GetLightingConfig(t)
	_ = StartBitcoindOrFail(t, bitcoindConf)

	lnd := StartLndOrFail(t, bitcoindConf, lndConf)

	_, err := lnd.GetInfo(context.Background(), &lnrpc.GetInfoRequest{})
	if err != nil {
		testutil.FatalMsgf(t, "Could not start and communiate with lnd: %v", err)
	}
}

func TestRunWithBitcoindAndLndPair(t *testing.T) {
	var test testing.T

	prevNodeLen := len(nodeCleaners)
	RunWithBitcoindAndLndPair(&test, func(lnd1 lnrpc.LightningClient,
		lnd2 lnrpc.LightningClient, bitcoin bitcoind.TeslacoilBitcoind) {
		request, err := lnd2.AddInvoice(context.Background(), &lnrpc.Invoice{
			Value: 1337,
		})
		if err != nil {
			t.Error(err)
			test.Fail()
			return
		}

		if _, err := lnd1.SendPaymentSync(context.Background(), &lnrpc.SendRequest{
			PaymentRequest: request.PaymentRequest,
		}); err != nil {
			t.Error(err)
			test.Fail()
			return
		}
	})
	testutil.AssertMsg(t, !test.Failed(), "Test was failed")

	// two LND nodes and one bitcoind
	testutil.AssertEqual(t, prevNodeLen+3, len(nodeCleaners))
}
