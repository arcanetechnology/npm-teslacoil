package bitcoindtestutil

import (
	"fmt"
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcutil"
	"github.com/pkg/errors"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
)

// GetBitcoindConfig returns a bitcoind config suitable for testing purposes
func GetBitcoindConfig(t *testing.T) bitcoind.Config {
	return bitcoind.Config{
		P2pPort:        testutil.GetPortOrFail(t),
		RpcPort:        testutil.GetPortOrFail(t),
		User:           "rpc_user_for_tests",
		Password:       "rpc_pass_for_tests",
		ZmqPubRawTx:    fmt.Sprintf("tcp://0.0.0.0:%d", testutil.GetPortOrFail(t)),
		ZmqPubRawBlock: fmt.Sprintf("tcp://0.0.0.0:%d", testutil.GetPortOrFail(t)),
	}

}

// GetBitcoindClientOrFail returns a bitcoind RPC client, corresponding to
// the given configuration.
func GetBitcoindClientOrFail(t *testing.T, conf bitcoind.Config) *rpcclient.Client {
	// Bitcoin Core doesn't do notifications
	var notificationHandler *rpcclient.NotificationHandlers = nil

	client, err := rpcclient.New(conf.ToConnConfig(), notificationHandler)
	if err != nil {
		testutil.FatalMsg(t, err)
	}

	return client
}

// SendTxToSelf is a helper function for sending a tx easily to
// our own address
func SendTxToSelf(bitcoin bitcoind.TeslacoilBitcoind, amountBtc float64) (*chainhash.Hash, error) {
	b := bitcoin.Btcctl()
	address, err := b.GetNewAddress("")
	if err != nil {
		return nil, fmt.Errorf("could not GetNewAddress: %+v", err)
	}

	balance, err := b.GetBalance("*")
	if err != nil {
		return nil, fmt.Errorf("could not get balance: %+v", err)
	}
	if balance.ToBTC() <= amountBtc {
		return nil, fmt.Errorf("not enough balance, try using GenerateToSelf() first")
	}

	amount, _ := btcutil.NewAmount(amountBtc)
	txHash, err := b.SendToAddress(address, amount)
	if err != nil {
		return nil, fmt.Errorf("could not send to address %v: %v", address, err)
	}

	return txHash, nil
}

// GenerateToSelf is a helper function for easily generating a block
// with the coinbase going to us
func GenerateToSelf(numBlocks uint32, bitcoin bitcoind.TeslacoilBitcoind) ([]*chainhash.Hash, error) {
	b := bitcoin.Btcctl()
	address, err := b.GetNewAddress("")
	if err != nil {
		return nil, errors.Wrap(err, "could not GetNewAddress")
	}

	hash, err := bitcoind.GenerateToAddress(bitcoin, numBlocks, address)
	if err != nil {
		return nil, errors.Wrap(err, "could not GenerateToAddress")
	}

	return hash, nil
}
