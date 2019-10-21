package transactions

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"reflect"
	"time"

	"gitlab.com/arcanecrypto/teslacoil/models/users"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"

	"github.com/google/go-cmp/cmp"
	"github.com/lightningnetwork/lnd/lnrpc"
	pkgErrors "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/models/payments"
)

var log = build.Log

var (
	ErrTxHasTxid = pkgErrors.New("transaction already has txid, cant overwrite")
	// ErrBalanceTooLowForWithdrawal means the user tried to withdraw too much money
	ErrBalanceTooLowForWithdrawal = errors.New("cannot withdraw, balance is too low")
)

// Transaction is the db and json type for an on-chain transaction
type Transaction struct {
	ID          int                `db:"id" json:"id"`
	UserID      int                `db:"user_id" json:"userId"`
	Address     string             `db:"address" json:"address"`
	Txid        *string            `db:"txid" json:"txid"`
	Vout        *int               `db:"vout" json:"vout"`
	Direction   payments.Direction `db:"direction" json:"direction"`
	AmountSat   int64              `db:"amount_sat" json:"amountSat"`
	Description *string            `db:"description" json:"description"`
	Confirmed   bool               `db:"confirmed" json:"confirmed"`

	ConfirmedAt *time.Time `db:"confirmed_at" json:"confirmedAt"`
	CreatedAt   time.Time  `db:"created_at" json:"createdAt"`
	UpdatedAt   time.Time  `db:"updated_at" json:"-"`
	DeletedAt   *time.Time `db:"deleted_at" json:"-"`
}

func insertTransaction(db *db.DB, t Transaction) (Transaction, error) {
	if t.AmountSat < 0 || t.UserID == 0 || t.Address == "" {
		return Transaction{}, pkgErrors.New("invalid transaction, missing some fields")
	}

	if t.Txid == nil && t.Vout != nil || t.Txid != nil && t.Vout == nil {
		return Transaction{}, pkgErrors.New("txid and vout must either both be defined, or neither be defined")
	}

	tLogger := log.WithFields(logrus.Fields{
		"userId":    t.UserID,
		"address":   t.Address,
		"amountSat": t.AmountSat,
	})
	if t.Txid != nil {
		tLogger = tLogger.WithFields(logrus.Fields{
			"txid": *t.Txid,
			"vout": *t.Vout,
		})
	}
	tLogger.Info("inserting transaction")

	createTransactionQuery := `
	INSERT INTO transactions (user_id, address, txid, vout, direction, amount_sat,  description, confirmed)
	VALUES (:user_id, :address, :txid, :vout, :direction, :amount_sat, :description, :confirmed)
	RETURNING id, user_id, address, txid, vout, direction, amount_sat, description, confirmed, confirmed_at,
			  created_at, updated_at, deleted_at`

	rows, err := db.NamedQuery(createTransactionQuery, t)
	if err != nil {
		return Transaction{}, pkgErrors.Wrap(err, "could not insert transaction")
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			log.WithError(err).Error("could not close rows")
		}
	}()

	var transaction Transaction
	if rows.Next() {
		if err = rows.StructScan(&transaction); err != nil {
			log.WithError(err).Error("could not scan result into transaction variable: ")
			return Transaction{}, pkgErrors.Wrap(err, "could not insert transaction")
		}
	}

	return transaction, nil
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
	var query string
	// Using OFFSET is not ideal, but until we start seeing
	// performance problems it's fine
	query = `SELECT *
		FROM transactions
		WHERE user_id=$1
		ORDER BY created_at
		LIMIT $2
		OFFSET $3`

	transactions := []Transaction{}
	err := d.Select(&transactions, query, userID, limit, offset)
	if err != nil {
		log.Error(err)
		return transactions, err
	}

	return transactions, nil
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
		log.Error(err)
		return transaction, pkgErrors.Wrap(err, "could not get transaction")
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
func TxListener(db *db.DB, lncli lnrpc.LightningClient, zmqRawTxCh chan *wire.MsgTx,
	chainCfg chaincfg.Params) {
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

				err = transaction.saveTxToTransaction(db, txHash, vout, amountSat)
				switch {
				case err == nil:
					// if we get here, it means the txhash+vout was successfully
					// saved to a transaction, and we don't need to loop through more
					// transactions
					break
				case i == len(result)-1:
					// we reached the last found transaction without being able to save
					// the txid. This means the user deposited to an address he has used
					// before, without creating a new deposit using our API

					txid := txHash.String()
					_, err = NewDepositWithFields(db, lncli, transaction.UserID,
						"", &vout, &txid, amountSat)
					if err != nil {
						log.WithError(err).Errorf("could not create new deposit for %d with txid %s:%d",
							transaction.UserID, txid, vout)
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
		// we don't actually use the block contents for anything, because
		// we query bitcoind directly for the status of every transaction
		// TODO?: Check all the transactions to see whether they are
		//  a deposit to us, but is not saved in our DB yet
		<-ch

		query := "SELECT * FROM transactions WHERE confirmed = false and txid NOTNULL"
		queryResult := []Transaction{}
		if err := db.Select(&queryResult, query); err != nil {
			if err != sql.ErrNoRows {
				log.WithError(err).Errorf("query %q failed", query)
			}
			continue
		}
		log.Tracef("found transactions: %+v", queryResult)

		for _, transaction := range queryResult {
			txHash, err := chainhash.NewHashFromStr(*transaction.Txid)
			if err != nil {
				log.WithError(err).Errorf("could not create chainhash from txid %q", *transaction.Txid)
				continue
			}
			rawTx, err := bitcoindRpc.GetRawTransactionVerbose(txHash)
			if err != nil {
				log.WithError(err).Errorf("could not get transaction with hash %q from bitcoind", txHash)
				continue
			}

			if rawTx.Confirmations >= confirmationLimit {
				log.Infof("tx %s:%d has %d confirmations", *transaction.Txid, *transaction.Vout, rawTx.Confirmations)

				if len(rawTx.Vout) < *transaction.Vout {
					// something really weird has happened, the transaction changed? we saved it wrong?
					log.Panic("saved transaction outpoint is greater than the number of outputs, check the logic")
				}

				var output btcjson.Vout
				for _, out := range rawTx.Vout {
					if out.N == uint32(*transaction.Vout) {
						output = out
					}
				}

				if math.Round(btcutil.SatoshiPerBitcoin*output.Value) != float64(transaction.AmountSat) {
					log.WithFields(logrus.Fields{"value": output.Value, "amount": transaction.AmountSat}).
						Errorf("actual outputValue and expected amount not equal, check logic")
					continue
				}

				err = transaction.markAsConfirmed(db)
				if err != nil {
					log.WithError(err).Error("could not mark transaction as confirmed")
				}
			}
		}
	}
}

// saveTxToTransaction saves a txid consisting of a txid, a vout and an amount to the
// db transaction
// If the transaction already has a txid, it returns an error
func (t Transaction) saveTxToTransaction(db *db.DB, txHash chainhash.Hash, vout int,
	amountSat int64) error {

	if t.Txid != nil {
		return ErrTxHasTxid
	}

	uQuery := `UPDATE transactions SET txid = $1, vout = $2, amount_sat = $3 WHERE id = $4`
	dbTx := db.MustBegin()
	results, err := dbTx.Exec(uQuery, txHash.String(), vout, amountSat, t.ID)
	if err != nil {
		_ = dbTx.Rollback()
		return pkgErrors.Wrap(err, "could not update transaction")
	}

	rowsAffected, err := results.RowsAffected()
	if err != nil {
		_ = dbTx.Rollback()
		return pkgErrors.Wrap(err, "could not retreive num rows affected")
	}
	if rowsAffected != 1 {
		_ = dbTx.Rollback()
		return pkgErrors.Errorf("expected 1 row to be affected, however query updated %d rows", rowsAffected)
	}

	if err = dbTx.Commit(); err != nil {
		_ = dbTx.Rollback()
		return pkgErrors.Wrap(err, "could not commit dbTx")
	}

	return nil
}

// markAsConfirmed updates the transaction stored in the db
// with Confirmed = true and ConfirmedAt = Now(). After updating the transaction
// it attempts to credit the user with the tx amount. Should anything fail, all
// changes are rolled back
func (t Transaction) markAsConfirmed(db *db.DB) error {

	query := `UPDATE transactions
		SET confirmed_at = $1, confirmed = true
		WHERE id=$2`

	dbtx := db.MustBegin()
	rows, err := dbtx.Exec(query, time.Now(), t.ID)
	if err != nil {
		_ = dbtx.Rollback()
		return pkgErrors.Wrapf(err, "query %q for transaction.ID %d failed",
			query, t.ID)
	}

	rowsAffected, _ := rows.RowsAffected()
	if rowsAffected != 1 {
		_ = dbtx.Rollback()
		return pkgErrors.Errorf("expected 1 row to be affected, however query updated %d rows", rowsAffected)
	}

	if _, err := users.IncreaseBalance(dbtx, users.ChangeBalance{
		UserID:    t.UserID,
		AmountSat: t.AmountSat,
	}); err != nil {
		_ = dbtx.Rollback()
		return pkgErrors.Wrapf(err, "could not credit user")
	}

	if err = dbtx.Commit(); err != nil {
		_ = dbtx.Rollback()
		return pkgErrors.Wrap(err, "could not commit changes")
	}

	return nil
}

func SendOnChain(lncli lnrpc.LightningClient, args *lnrpc.SendCoinsRequest) (
	string, error) {
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
	args WithdrawOnChainArgs) (*Transaction, error) {

	user, err := users.GetByID(db, args.UserID)
	if err != nil {
		return nil, pkgErrors.Wrap(err, "withdrawonchain could not get user")
	}

	// We dont pass sendAll to lncli, as that would send the entire nodes
	// balance to the address
	if args.SendAll {
		args.AmountSat = user.Balance
	}

	if user.Balance < args.AmountSat {
		return nil, ErrBalanceTooLowForWithdrawal
	}

	tx := db.MustBegin()
	user, err = users.DecreaseBalance(tx, users.ChangeBalance{
		UserID:    user.ID,
		AmountSat: args.AmountSat,
	})
	if err != nil {
		if txErr := tx.Rollback(); txErr != nil {
			log.Error("txErr: ", txErr)
		}
		return nil, pkgErrors.Wrap(err, "could not decrease balance")
	}

	txid, err := SendOnChain(lncli, &lnrpc.SendCoinsRequest{
		Addr:       args.Address,
		Amount:     args.AmountSat,
		TargetConf: int32(args.TargetConf),
		SatPerByte: int64(args.SatPerByte),
	})
	if err != nil {
		if txErr := tx.Rollback(); txErr != nil {
			log.WithError(txErr).Error("could not rollback tx")
		}
		return nil, pkgErrors.Wrap(err, "could not send on-chain")
	}

	vout, err := bitcoin.FindVout(txid, args.AmountSat)
	if err != nil {
		log.WithError(err).Error("could not find output")
	}

	txToInsert := Transaction{
		UserID:    user.ID,
		Address:   args.Address,
		AmountSat: args.AmountSat,
		Txid:      &txid,
		Vout:      &vout,
		Direction: payments.OUTBOUND,
	}

	if args.Description != "" {
		txToInsert.Description = &args.Description
	}

	transaction, err := insertTransaction(db, txToInsert)
	if err != nil {
		if txErr := tx.Rollback(); txErr != nil {
			log.Error("txErr: ", txErr)
		}
		return nil, pkgErrors.Wrap(err, "could not insert transaction")
	}

	err = tx.Commit()
	if err != nil {
		return nil, pkgErrors.Wrap(err, "could not commit transaction")
	}

	log.Debugf("transaction: %+v", transaction)

	return &transaction, nil
}

func NewDeposit(d *db.DB, lncli lnrpc.LightningClient, userID int,
	description string) (Transaction, error) {
	return NewDepositWithFields(d, lncli, userID, description, nil, nil, 0)
}

// NewDepositWithFields retrieves a new address from lnd, and saves the address
// in a new 'UNCONFIRMED', 'INBOUND' transaction together with the UserID
// Returns the same transaction as insertTransaction(), in full
func NewDepositWithFields(db *db.DB, lncli lnrpc.LightningClient, userID int,
	description string, vout *int, txid *string, amountSat int64) (Transaction, error) {

	address, err := lncli.NewAddress(context.Background(), &lnrpc.NewAddressRequest{
		// This type means lnd will force-create a new address
		Type: lnrpc.AddressType_WITNESS_PUBKEY_HASH,
	})

	if err != nil {
		return Transaction{}, pkgErrors.Wrap(err, "lncli could not create NewAddress")
	}

	txToInsert := Transaction{
		UserID:      userID,
		Address:     address.Address,
		Direction:   payments.INBOUND,
		Description: &description,
		Txid:        txid,
		Vout:        vout,
		AmountSat:   amountSat,
	}

	if description != "" {
		txToInsert.Description = &description
	}

	transaction, err := insertTransaction(db, txToInsert)
	if err != nil {
		return Transaction{}, pkgErrors.Wrap(err, "could not insert new inbound transaction")
	}

	return transaction, nil
}

// GetOrCreateDeposit attempts to retreive a deposit whose address has not yet received any coins
// It does this by selecting the last inserted deposit whose txid == "" (order by id desc)
// If the ForceNewAddress argument is true, or no deposit is found, the function creates a new deposit
func GetOrCreateDeposit(db *db.DB, lncli lnrpc.LightningClient, userID int, forceNewAddress bool,
	description string) (Transaction, error) {
	log.WithFields(logrus.Fields{"userID": userID, "forceNewAddress": forceNewAddress,
		"description": description}).Tracef("GetOrCreateDeposit")
	// If forceNewAddress is supplied, we return a new deposit instantly
	if forceNewAddress {
		return NewDeposit(db, lncli, userID, description)
	}

	// Get the latest INBOUND transaction whose txid is empty from the DB
	query := "SELECT * from transactions WHERE user_id=$1 AND direction='INBOUND' AND txid ISNULL ORDER BY id DESC LIMIT 1;"
	var deposit Transaction
	err := db.Get(&deposit, query, userID)

	switch {
	case err != nil && err == sql.ErrNoRows:
		// no deposit exists yet
		log.Debug("SELECT found no transactions, creating new deposit")
		return NewDeposit(db, lncli, userID, description)
	case err == nil:
		// we found a deposit
		return deposit, nil
	default:
		return Transaction{}, pkgErrors.Wrap(err, "db.Get in GetOrCreateDeposit could not find a deposit")
	}
}

// ExactlyEqual checks whether the two transactions are exactly
// equal, including all postgres-fields, such as DeletedAt, CreatedAt etc.
func (t Transaction) ExactlyEqual(t2 Transaction) (bool, string) {
	if !reflect.DeepEqual(t, t2) {
		return false, cmp.Diff(t, t2)
	}

	return true, ""
}

// Equal checks whether the Transaction is equal to another, and
// returns an explanation of the diff if not equal
// Equal does not compare db-tables unique for every entry, such
// as CreatedAt, UpdatedAt, DeletedAt and ID
func (t Transaction) Equal(t2 Transaction) (bool, string) {
	// These four fields do not decide whether the transaction is
	// equal another or not, use ExactlyEqual if you want to compare
	t.CreatedAt = t2.CreatedAt
	t.UpdatedAt = t2.UpdatedAt
	t.DeletedAt, t2.DeletedAt = nil, nil
	t.ID = t2.ID

	if !reflect.DeepEqual(t, t2) {
		return false, cmp.Diff(t, t2)
	}

	return true, ""
}
