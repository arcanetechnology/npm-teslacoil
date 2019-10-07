//+build integration

package nodetestutil

import (
	"context"
	"os"
	"testing"

	"github.com/lightningnetwork/lnd/lnrpc"
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
