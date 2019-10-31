package transactions

import (
	"context"
	"database/sql"
	"encoding"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"gitlab.com/arcanecrypto/teslacoil/models/users/balance"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/db"
)

var log = build.Log

var (
	ErrNonPositiveAmount = errors.New("cannot send non-positiv amount")
	ErrTxHasTxid         = errors.New("transaction already has txid, cant overwrite")

	// ErrBalanceTooLow means the user tried to withdraw too much money
	ErrBalanceTooLow = errors.New("balance is too low")
)

// Direction is the direction of a transaction, seen from the users perspective
type Direction string

func (d Direction) MarshalText() (text []byte, err error) {
	lower := strings.ToLower(string(d))
	return []byte(lower), nil
}

var _ encoding.TextMarshaler = INBOUND

// Status is the status of a lightning payment
type Status string

func (s Status) MarshalText() (text []byte, err error) {
	lower := strings.ToLower(string(s))
	return []byte(lower), nil
}

var _ encoding.TextMarshaler = SUCCEEDED

const (
	INBOUND  Direction = "INBOUND"
	OUTBOUND Direction = "OUTBOUND"

	SUCCEEDED Status = "SUCCEEDED"
	FAILED    Status = "FAILED"
	OPEN      Status = "OPEN"
)

// InsertOnchain inserts the given onchain TX into the DB
func InsertOnchain(db db.Inserter, onchain Onchain) (Onchain, error) {
	converted := onchain.ToTransaction()
	tx, err := insertTransaction(db, converted)
	if err != nil {
		return Onchain{}, err
	}
	insertedOnchain, err := tx.ToOnchain()
	if err != nil {
		return Onchain{}, fmt.Errorf("could not convert inserted TX to onchain TX: %w", err)
	}
	// update the sats field
	if tx.AmountMSat != nil {
		sats := *tx.AmountMSat / 1000
		insertedOnchain.AmountSat = &sats
	}
	return insertedOnchain, nil
}

// InsertOffchain inserts the given offchain TX into the DB
func InsertOffchain(db db.Inserter, offchain Offchain) (Offchain, error) {
	tx, err := insertTransaction(db, offchain.ToTransaction())
	if err != nil {
		return Offchain{}, err
	}
	insertedOffchain, err := tx.ToOffchain()
	if err != nil {
		return Offchain{}, fmt.Errorf("could not convert inserted TX to offchain TX: %w", err)
	}

	// if preimage is NULL in DB, default is empty slice and not null
	if tx.Preimage != nil && len(*tx.Preimage) == 0 {
		insertedOffchain.Preimage = nil
	}

	return insertedOffchain, nil

}

// GetAllTransactions selects all the transactions for a user
func GetAllTransactions(d *db.DB, userID int) ([]Transaction, error) {
	return GetAllTransactionsLimitOffset(d, userID, math.MaxInt32, 0)
}

// GetAllTransactionsLimit selects `limit` transactions for a user without an offset
func GetAllTransactionsLimit(d *db.DB, userID int, limit int) ([]Transaction, error) {
	return GetAllTransactionsLimitOffset(d, userID, limit, 0)
}

// GetAllTransactionsOffset selects all transactions for a given user with an `offset`
func GetAllTransactionsOffset(d *db.DB, userID int, offset int) ([]Transaction, error) {
	return GetAllTransactionsLimitOffset(d, userID, math.MaxInt32, offset)
}

// GetAllTransactionsLimitOffset selects all transactions for a userID from the DB.
func GetAllTransactionsLimitOffset(d *db.DB, userID int, limit int, offset int) (
	[]Transaction, error) {
	// Using OFFSET is not ideal, but until we start seeing
	// performance problems it's fine
	query := `SELECT *
		FROM transactions
		WHERE user_id=$1
		ORDER BY created_at
		LIMIT $2
		OFFSET $3`

	transactions := []Transaction{}
	err := d.Select(&transactions, query, userID, limit, offset)
	if err != nil {
		log.WithError(err).WithFields(logrus.Fields{
			"limit":   limit,
			"offset":  offset,
			"usuerId": userID,
		}).Error("Could not get transactions")
		return transactions, err
	}

	return transactions, nil
}

func GetOnchainByID(d *db.DB, id int, userID int) (Onchain, error) {
	tx, err := GetTransactionByID(d, id, userID)
	if err != nil {
		return Onchain{}, err
	}
	onchain, err := tx.ToOnchain()
	if err != nil {
		return Onchain{}, fmt.Errorf("requested TX was not onchain TX: %w", err)
	}
	return onchain, nil
}

func GetOffchainByID(d *db.DB, id int, userID int) (Offchain, error) {
	tx, err := GetTransactionByID(d, id, userID)
	if err != nil {
		return Offchain{}, err
	}
	offchain, err := tx.ToOffchain()
	if err != nil {
		return Offchain{}, fmt.Errorf("requested TX was not offchain TX: %w", err)
	}
	return offchain, nil
}

// GetTransactionByID performs this query:
// `SELECT * FROM transactions WHERE id=id AND user_id=userID`,
// where id is the primary key of the table(autoincrementing)
func GetTransactionByID(d *db.DB, id int, userID int) (Transaction, error) {
	if id < 0 || userID < 0 {
		return Transaction{}, fmt.Errorf("GetByID(): neither id nor userID can be less than 0")
	}

	query := "SELECT * FROM transactions WHERE id=$1 AND user_id=$2 LIMIT 1"

	var transaction Transaction
	if err := d.Get(&transaction, query, id, userID); err != nil {
		log.WithError(err).WithField("id", id).Error("Could not get transaction")
		return transaction, fmt.Errorf("could not get transaction: %w", err)
	}

	return transaction, nil
}

// TxListener receives tx's from the zmqTxChannel and checks whether
// any of the tx outputs is a deposit to one of our addresses. If an
// output is a deposit to us, we save the txid+vout in the DB.
// Every output of a tx is always checked, and if a single tx has
// several outputs which are a deposit to teslacoil, we save each
// txid+vout as a unique entry in the DB
//
// NOTE: This must be run as a goroutine
func TxListener(db *db.DB, zmqRawTxCh chan *wire.MsgTx, chainCfg chaincfg.Params) {
	for {
		tx := <-zmqRawTxCh

		// To listen for deposits, we loop through every output of
		// the tx, and check if any of the addresses exists in our db
		for vout, output := range tx.TxOut {
			// to extract the address, we first need to parse the output-script
			script, err := txscript.ParsePkScript(output.PkScript)
			if err != nil {
				// we continue to keep listening for new trasactions
				continue
			}

			address, err := script.Address(&chainCfg)
			if err != nil {
				log.WithError(err).Error("could not extract address from script")
				continue
			}

			// Because it is possible to deposit to an on-chain address
			// several times, we expect up to several transactions returned
			// from the SELECT query
			query := "SELECT * FROM transactions WHERE address=$1"
			var result []Transaction
			if err = db.Select(&result, query, address.EncodeAddress()); err != nil {
				log.WithError(err).Errorf("query SELECT * FROM transactions WHERE address=%v failed",
					address.EncodeAddress())
				continue
			}
			if len(result) == 0 {
				// address does not belong to us
				continue
			}
			log.WithFields(logrus.Fields{"transactions": result, "address": address.EncodeAddress()}).
				Tracef("found transactions for address")

			amountSat := output.Value
			txHash := tx.TxHash()
			for i, transaction := range result {
				onchain, err := transaction.ToOnchain()
				if err != nil {
					break
				}

				updated, err := onchain.PersistReceivedMoney(db, txHash, vout, amountSat)
				switch {
				case err == nil:
					// if we get here, it means the txhash+vout was successfully
					// saved to a transaction, and we don't need to loop through more
					// transactions
					log.WithFields(logrus.Fields{
						"address":   updated.Address,
						"txid":      updated.Txid,
						"vout":      updated.Vout,
						"amountSat": updated.AmountSat,
						"userId":    updated.UserID,
					}).Info("Added received money to onchain TX")
				case i == len(result)-1:
					// we reached the last found transaction without being able to save
					// the txid. This means the user deposited to an address he has used
					// before, without creating a new deposit using our API
					deposit, err := NewDepositWithMoney(db, WithMoneyArgs{
						Tx:          tx,
						OutputIndex: vout,
						UserID:      transaction.UserID,
						Chain:       chainCfg,
					})
					if err != nil {
						log.WithError(err).WithFields(logrus.Fields{
							"userId": transaction.UserID,
							"txid":   transaction.Txid,
							"vout":   transaction.Vout,
						}).Error("Could not credit new deposit with money")
					} else {
						log.WithFields(logrus.Fields{
							"address":   deposit.Address,
							"txid":      deposit.Txid,
							"vout":      deposit.Vout,
							"amountSat": deposit.AmountSat,
							"userId":    deposit.UserID,
						}).Info("Added new deposit with money")
					}
				case errors.Is(err, ErrTxHasTxid):
					log.WithError(err).Error("could not save txid to deposit")
				}
			}
		}
	}
}

// BlockListener receives parsed blocks from the zmqBlockChannel and
// for every unconfirmed transaction in our database, checks whether
// it is now confirmed by looking up the txid using bitcoind RPC.
// If the transaction is now confirmed, the transaction is marked
// as confirmed and we credit the user with the transaction amount
//
// NOTE: This must be run as a goroutine
func BlockListener(db *db.DB, bitcoindRpc bitcoind.RpcClient, ch chan *wire.MsgBlock) {
	const confirmationLimit = 3

	for {
		rawBlock := <-ch
		hash := rawBlock.Header.BlockHash()
		block, err := bitcoindRpc.GetBlockVerbose(&hash)
		if err != nil {
			log.WithError(err).Error("Could not query bitcoind for block")
			continue
		}

		// query for all onchain TXs which aren't confirmed but has money credited to them
		query := "SELECT * FROM transactions WHERE address NOTNULL AND confirmed_at IS NULL AND txid NOTNULL"
		var queryResult []Transaction
		if err := db.Select(&queryResult, query); err != nil {
			if err != sql.ErrNoRows {
				log.WithError(err).Errorf("query %q failed", query)
			}
			continue
		}

		for _, transaction := range queryResult {
			onchain, err := transaction.ToOnchain()
			if err != nil {
				log.WithError(err).Error("Transaction was not an onchain TX")
				continue
			}
			txHash, err := chainhash.NewHashFromStr(*onchain.Txid)
			if err != nil {
				log.WithError(err).Errorf("could not create chainhash from txid %q", *onchain.Txid)
				continue
			}
			rawTx, err := bitcoindRpc.GetRawTransactionVerbose(txHash)
			if err != nil {
				log.WithError(err).Errorf("could not get transaction with hash %q from bitcoind", txHash)
				continue
			}

			if rawTx.Confirmations >= confirmationLimit {
				log.WithFields(logrus.Fields{
					"txid":          onchain.Txid,
					"vout":          onchain.Vout,
					"confirmations": rawTx.Confirmations,
				}).Info("tx is confirmed")

				if len(rawTx.Vout) < *onchain.Vout {
					// something really weird has happened, the transaction changed? we saved it wrong?
					log.WithFields(logrus.Fields{
						"rawTx.Vout":   rawTx.Vout,
						"onchain.Vout": onchain.Vout,
					}).Panic("saved transaction outpoint is greater than the number of outputs, check the logic")
				}

				var output btcjson.Vout
				for _, out := range rawTx.Vout {
					if out.N == uint32(*onchain.Vout) {
						output = out
					}
				}

				outputValue, err := btcutil.NewAmount(output.Value)
				if err != nil {
					logrus.WithError(err).Error("Could not convert to btcutil.Amount")
					continue
				}
				onchainValue := btcutil.Amount(*onchain.AmountSat)
				if outputValue != onchainValue {
					log.WithFields(logrus.Fields{"value": output.Value, "amount": onchain.AmountSat}).
						Errorf("actual outputValue and expected amount not equal, check logic")
					continue
				}

				height := block.Height
				confirmationHeight := height - int64(rawTx.Confirmations)
				if onchain, err := onchain.MarkAsConfirmed(db, int(confirmationHeight)); err != nil {
					log.WithError(err).Error("could not mark transaction as confirmed")
				} else {
					log.WithFields(logrus.Fields{
						"txid":   onchain.Txid,
						"vout":   onchain.Vout,
						"userId": onchain.UserID,
					}).Info("Marked transaction as confirmed")
				}

			}
		}
	}
}

func SendOnChain(lncli lnrpc.LightningClient, args *lnrpc.SendCoinsRequest) (
	string, error) {

	if args.Amount < 1 {
		return "", ErrNonPositiveAmount
	}

	// don't pass the args directly to lnd, to safeguard
	// against ever supplying the SendAll flag
	lnArgs := &lnrpc.SendCoinsRequest{
		Amount:     args.Amount,
		Addr:       args.Addr,
		TargetConf: args.TargetConf,
		SatPerByte: args.SatPerByte,
	}

	res, err := lncli.SendCoins(context.Background(), lnArgs)
	if err != nil {
		return "", err
	}

	return res.Txid, nil
}

// WithdrawOnChainArgs withdraws on-chain
type WithdrawOnChainArgs struct {
	UserID int `json:"-"`
	// The amount in satoshis to send
	AmountSat int64 `json:"amountSat" binding:"required"`
	// The address to send coins to
	Address string `json:"address" binding:"required"`
	// The target number of blocks the transaction should be confirmed by
	TargetConf int `json:"targetConf"`
	// A manual fee rate set in sat/byte that should be used
	SatPerByte int `json:"satPerByte"`
	// If set, amount field will be ignored, and the entire balance will be sent
	SendAll bool `json:"sendAll"`
	// A personal description for the transaction
	Description string `json:"description"`
}

// WithdrawOnChain attempts to send amountSat coins to an address
// using our function SendOnChain
// If the user does not have enough balance, the transaction is aborted
// See WithdrawOnChainArgs for more information about the possible arguments
func WithdrawOnChain(db *db.DB, lncli lnrpc.LightningClient, bitcoin bitcoind.TeslacoilBitcoind,
	args WithdrawOnChainArgs) (Onchain, error) {
	bal, err := balance.ForUser(db, args.UserID)
	if err != nil {
		return Onchain{}, err
	}

	// We dont pass sendAll to lncli, as that would send the entire nodes
	// balance to the address
	if args.SendAll {
		args.AmountSat = bal.Sats()
	}

	if args.AmountSat > bal.Sats() {
		log.WithFields(logrus.Fields{
			"balanceSats":       bal.Sats(),
			"requestedSendSats": args.AmountSat,
			"userId":            args.UserID,
		}).Error("User tried to withdraw more than their balance")
		return Onchain{}, ErrBalanceTooLow
	}

	txid, err := SendOnChain(lncli, &lnrpc.SendCoinsRequest{
		Addr:       args.Address,
		Amount:     args.AmountSat,
		TargetConf: int32(args.TargetConf),
		SatPerByte: int64(args.SatPerByte),
	})
	if err != nil {
		log.WithError(err).WithField("amountSats", args.AmountSat).Error("Could not send money onchain")
		return Onchain{}, err
	}

	vout, err := bitcoin.FindVout(txid, args.AmountSat)
	if err != nil {
		log.WithError(err).Error("Could not find output for sent TX")
		return Onchain{}, fmt.Errorf("could not find output for sent TX: %w", err)
	}

	txToInsert := Onchain{
		UserID:    args.UserID,
		Address:   args.Address,
		AmountSat: &args.AmountSat,
		Txid:      &txid,
		Vout:      &vout,
		Direction: OUTBOUND,
	}

	if args.Description != "" {
		txToInsert.Description = &args.Description
	}

	transaction, err := InsertOnchain(db, txToInsert)
	if err != nil {
		log.WithError(err).WithField("tx", txToInsert).Error("Could not insert onchain TX when withdrawing")
		return Onchain{}, err
	}

	return transaction, nil
}

func NewDeposit(d *db.DB, lncli lnrpc.LightningClient, userID int) (Onchain, error) {
	return NewDepositWithDescription(d, lncli, userID, "")
}

type WithMoneyArgs struct {
	Tx          *wire.MsgTx
	OutputIndex int
	UserID      int
	Chain       chaincfg.Params
}

// NewDepositWithMoney creates a new deposit address in our DB, and marks this
// as spent to (giving it an associated satoshi value, TXID and vout index).
// Note that the function doesn't set the confirmation status of the deposit.
// If the deposit is confirmed, this would have to be handled in a separate
// function call to Onchain.MarkAsConfirmed.
func NewDepositWithMoney(db *db.DB, args WithMoneyArgs) (Onchain, error) {
	if len(args.Tx.TxOut) < args.OutputIndex {
		return Onchain{}, errors.New("vout not found in TX")
	}

	vout := args.Tx.TxOut[args.OutputIndex]
	// to extract the address, we first need to parse the output-script
	script, err := txscript.ParsePkScript(vout.PkScript)
	if err != nil {
		return Onchain{}, err
	}

	address, err := script.Address(&args.Chain)
	if err != nil {
		return Onchain{}, err
	}
	txid := args.Tx.TxHash().String()
	txToInsert := Onchain{
		UserID:    args.UserID,
		Direction: INBOUND,
		AmountSat: &vout.Value,
		Address:   address.EncodeAddress(),
		Txid:      &txid,
		Vout:      &args.OutputIndex,
	}

	transaction, err := InsertOnchain(db, txToInsert)
	if err != nil {
		return Onchain{}, fmt.Errorf("could not insert new inbound transaction: %w", err)
	}

	return transaction, nil
}

// NewDepositWithDescription retrieves a new address from lnd, and saves the address
// in a new deposit. When we're processing blocks and transactions these deposits
// are checked for received funds.
func NewDepositWithDescription(db *db.DB, lncli lnrpc.LightningClient, userID int,
	description string) (Onchain, error) {
	address, err := lncli.NewAddress(context.Background(), &lnrpc.NewAddressRequest{
		// This type means lnd will force-create a new address
		Type: lnrpc.AddressType_WITNESS_PUBKEY_HASH,
	})

	if err != nil {
		return Onchain{}, fmt.Errorf("lncli could not create new address: %w", err)
	}

	txToInsert := Onchain{
		UserID:    userID,
		Address:   address.Address,
		Direction: INBOUND,
	}

	if description != "" {
		txToInsert.Description = &description
	}

	transaction, err := InsertOnchain(db, txToInsert)
	if err != nil {
		return Onchain{}, fmt.Errorf("could not insert new inbound transaction: %w", err)
	}

	return transaction, nil
}

// GetOrCreateDeposit attempts to retreive a deposit whose address has not yet received any coins
// It does this by selecting the last inserted deposit whose txid == "" (order by id desc)
// If the ForceNewAddress argument is true, or no deposit is found, the function creates a new deposit
func GetOrCreateDeposit(db *db.DB, lncli lnrpc.LightningClient, userID int, forceNewAddress bool,
	description string) (Onchain, error) {
	log.WithFields(logrus.Fields{"userID": userID, "forceNewAddress": forceNewAddress,
		"description": description}).Tracef("Getting or creating a new deposit")
	// If forceNewAddress is supplied, we return a new deposit instantly
	if forceNewAddress {
		return NewDepositWithDescription(db, lncli, userID, description)
	}

	// Get the latest INBOUND transaction whose txid is empty from the DB
	query := "SELECT * from transactions WHERE user_id=$1 AND direction='INBOUND' AND txid ISNULL ORDER BY id DESC LIMIT 1;"
	var deposit Transaction
	err := db.Get(&deposit, query, userID)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		// no deposit exists yet
		log.Debug("SELECT found no transactions, creating new deposit")
		return NewDepositWithDescription(db, lncli, userID, description)
	case err == nil:
		// we found a deposit
		return deposit.ToOnchain()
	default:
		return Onchain{}, fmt.Errorf("db.Get in GetOrCreateDeposit could not find a deposit: %w", err)
	}
}
