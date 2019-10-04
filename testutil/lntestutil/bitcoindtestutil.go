package lntestutil

import (
	"fmt"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcutil"
	"github.com/pkg/errors"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/bitcoind"
)

// SendTx is a helper function for sending a tx easily(to ourself)
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

func ConvertToAddressOrFail(address string, params chaincfg.Params) btcutil.Address {

	addr, err := btcutil.DecodeAddress(address, &params)
	if err != nil {
		panic(err)
	}

	return addr
}

func GenerateToSelf(numBlocks uint32, bitcoin bitcoind.TeslacoilBitcoind) ([]*chainhash.Hash, error) {
	b := bitcoin.Btcctl()
	address, err := b.GetNewAddress("")
	if err != nil {
		return nil, errors.New("could not GetNewAddress")
	}

	hash, err := bitcoind.GenerateToAddress(bitcoin, numBlocks, address)
	if err != nil {
		return nil, errors.New("could not GenerateToAddress")
	}

	return hash, nil
}
