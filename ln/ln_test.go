//+build integration

package ln_test

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/ln"
	"gitlab.com/arcanecrypto/teslacoil/testutil/bitcoindtestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/lntestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/nodetestutil"
)

func TestMain(m *testing.M) {
	build.SetLogLevels(logrus.InfoLevel)

	code := m.Run()
	if err := nodetestutil.CleanupNodes(); err != nil {
		logrus.WithError(err).Error("Could not clean up nodes")
	}

	os.Exit(code)
}

func TestListenShutdownGracefully(t *testing.T) {
	lnConf := lntestutil.GetLightingConfig(t)
	bitcoinConf := bitcoindtestutil.GetBitcoindConfig(t)

	_ = nodetestutil.StartBitcoindOrFail(t, bitcoinConf)
	lnd := nodetestutil.StartLndOrFail(t, bitcoinConf, lnConf)

	var lndHasStopped bool
	go func() {
		err := ln.ListenShutdown(lnd, func() {
			lndHasStopped = true
		})
		require.NoError(t, err)
	}()

	// this fails sporadically on CI for some reason. We don't actually care about 
	// the err value, as either LND is already stopped, or this makes it stop.
	_, _ = lnd.StopDaemon(context.Background(), &lnrpc.StopRequest{})

	assert.Eventually(t, func() bool {
		return lndHasStopped
	}, time.Second*20, time.Second)
}

func TestListenShutdownViolently(t *testing.T) {
	lnConf := lntestutil.GetLightingConfig(t)
	bitcoinConf := bitcoindtestutil.GetBitcoindConfig(t)

	_ = nodetestutil.StartBitcoindOrFail(t, bitcoinConf)
	lnd := nodetestutil.StartLndOrFail(t, bitcoinConf, lnConf)

	var lndHasStopped bool
	go func() {
		err := ln.ListenShutdown(lnd, func() {
			lndHasStopped = true
		})
		require.NoError(t, err)
	}()

	require.NoError(t, syscall.Kill(lnd.Pid(), syscall.SIGKILL))

	assert.Eventually(t, func() bool {
		return lndHasStopped
	}, time.Second*20, time.Second)

}
