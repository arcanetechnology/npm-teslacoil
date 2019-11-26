//+build integration

package nodetestutil

import (
	"context"
	"os"
	"testing"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)
}

func TestStartLndOrFail(t *testing.T) {
	bitcoindConf := bitcoindtestutil.GetBitcoindConfig(t)
	lndConf := lntestutil.GetLightingConfig(t)
	_ = StartBitcoindOrFail(t, bitcoindConf)

	lnd := StartLndOrFail(t, bitcoindConf, lndConf)
	require.False(t, t.Failed(), "test failed while starting LND!")

	_, err := lnd.GetInfo(context.Background(), &lnrpc.GetInfoRequest{})
	require.NoError(t, err)
}

func TestGetLndPairAndBitcoind(t *testing.T) {
	var test testing.T

	prevNodeLen := len(nodeCleaners)
	lnd1, lnd2, _ := GetLndPairAndBitcoind(&test)
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

	assert.False(t, test.Failed(), "test was failed")

	// two LND nodes and one bitcoind
	assert.Len(t, nodeCleaners, prevNodeLen+3)
}
