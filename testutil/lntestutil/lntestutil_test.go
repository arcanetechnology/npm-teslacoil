//+build integration

package lntestutil

import (
	"context"
	"os"
	"testing"

	"gitlab.com/arcanecrypto/teslacoil/internal/platform/bitcoind"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
)

func TestMain(m *testing.M) {
	build.SetLogLevel(logrus.DebugLevel)
	os.Exit(m.Run())
}

func TestStartLndOrFail(t *testing.T) {
	bitcoindConf := bitcoind.GetBitcoindConfig(t)
	lndConf := GetLightingConfig(t)
	_, cleanupBitcoind := bitcoind.StartBitcoindOrFail(t, bitcoindConf)
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
