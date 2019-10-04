//+build integration

package lntestutil

import (
	"context"
	"os"
	"testing"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
)

func TestMain(m *testing.M) {
	build.SetLogLevel(logrus.DebugLevel)
	os.Exit(m.Run())
}

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

func TestStartLndOrFail(t *testing.T) {
	bitcoindConf := GetBitcoindConfig(t)
	lndConf := GetLightingConfig(t)
	_, cleanupBitcoind := StartBitcoindOrFail(t, bitcoindConf)
	defer func() {
		if err := cleanupBitcoind(); err != nil {
			testutil.FatalMsg(t, err)
		}
	}()

	lnd, cleanupLnd := StartLndOrFail(t, bitcoindConf, lndConf)

	_, err := lnd.GetInfo(context.Background(), &lnrpc.GetInfoRequest{})
	if err != nil {
		testutil.FatalMsgf(t, "Could not start and communiate with lnd: %v", err)
	}

	if err := cleanupLnd(); err != nil {
		testutil.FatalMsg(t, err)
	}

	info, err := lnd.GetInfo(context.Background(), &lnrpc.GetInfoRequest{})
	if err == nil {
		testutil.FatalMsgf(t, "Could start and communiate with lnd after shutdown: %v", info)
	}

}
