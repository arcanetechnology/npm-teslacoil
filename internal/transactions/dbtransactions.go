package transactions

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"

	"gitlab.com/arcanecrypto/teslacoil/build"

	"gitlab.com/arcanecrypto/teslacoil/internal/payments"

	"github.com/google/go-cmp/cmp"

	"github.com/jmoiron/sqlx"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/pkg/errors"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"
	"gitlab.com/arcanecrypto/teslacoil/internal/users"
)

// TransactionStatus is the status of an on-chain transaction
type TransactionStatus string

const (
	UNCONFIRMED = "UNCONFIRMED"
	CONFIRMED   = "CONFIRMED"
)

var log = build.Log

// Transaction is the db and json type for an on-chain transaction
type Transaction struct {
	ID          int                `db:"id" json:"id"`
	UserID      int                `db:"user_id" json:"userId"`
	Address     string             `db:"address" json:"address"`
	Txid        string             `db:"txid" json:"txid"`
	Outpoint    int                `db:"outpoint" json:"outpoint"`
	Direction   payments.Direction `db:"direction" json:"direction"`
	AmountSat   int64              `db:"amount_sat" json:"amountSat"`
	Description string             `db:"description" json:"description"`
	Status      TransactionStatus  `db:"status" json:"status"`

	ConfirmedAt *time.Time `db:"confirmed_at" json:"confirmedAt"`
	CreatedAt   time.Time  `db:"created_at" json:"createdAt"`
	UpdatedAt   time.Time  `db:"updated_at" json:"-"`
	DeletedAt   *time.Time `db:"deleted_at" json:"-"`
}

func insertTransaction(tx *sqlx.Tx, t Transaction) (Transaction, error) {
	if t.AmountSat < 0 || t.UserID == 0 || t.Address == "" {
		return Transaction{}, errors.New("invalid transaction, missing some fields")
	}

	createTransactionQuery := `
	INSERT INTO transactions (user_id, address, txid, outpoint, direction, amount_sat,  description, status)
	VALUES (:user_id, :address, :txid, :outpoint, :direction, :amount_sat, :description, :status)
	RETURNING id, user_id, address, txid, outpoint, direction, amount_sat, description, status, confirmed_at,
			  created_at, updated_at, deleted_at`

	rows, err := tx.NamedQuery(createTransactionQuery, t)
	if err != nil {
		return Transaction{}, errors.Wrap(err, "could not insert transaction")
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
			return Transaction{}, errors.Wrap(err, "could not insert transaction")
		}
	}

	return transaction, nil
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

func GetAllTransactions(d *db.DB, userID int) ([]Transaction, error) {
	return GetAllTransactionsLimitOffset(d, userID, math.MaxInt32, 0)
}

func GetAllTransactionsLimit(d *db.DB, userID int, limit int) ([]Transaction, error) {
	return GetAllTransactionsLimitOffset(d, userID, limit, 0)
}

func GetAllTransactionsOffset(d *db.DB, userID int, offset int) ([]Transaction, error) {
	return GetAllTransactionsLimitOffset(d, userID, math.MaxInt32, offset)
}

// GetAllTransactions selects all transactions for given userID from the DB.
func GetAllTransactionsLimitOffset(d *db.DB, userID int, limit int, offset int) (
	[]Transaction, error) {
	var query string
	// if limit is 0, we get ALL transactions
	if limit == 0 {
		limit = math.MaxInt32
	}
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

// GetByID performs this query:
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
		return transaction, errors.Wrap(err, "could not get transaction")
	}

	return transaction, nil
}

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
func WithdrawOnChain(d *db.DB, lncli lnrpc.LightningClient,
	args WithdrawOnChainArgs) (*Transaction, error) {

	user, err := users.GetByID(d, args.UserID)
	if err != nil {
		return nil, errors.Wrap(err, "withdrawonchain could not get user")
	}

	// We dont pass sendAll to lncli, as that would send the entire nodes
	// balance to the address
	if args.SendAll {
		args.AmountSat = user.Balance
	}

	if user.Balance < args.AmountSat {
		return nil, errors.New(fmt.Sprintf(
			"cannot withdraw, balance is %d, trying to withdraw %d",
			user.Balance,
			args.AmountSat))
	}

	tx := d.MustBegin()
	user, err = users.DecreaseBalance(tx, users.ChangeBalance{
		UserID:    user.ID,
		AmountSat: args.AmountSat,
	})
	if err != nil {
		if txErr := tx.Rollback(); txErr != nil {
			log.Error("txErr: ", txErr)
		}
		return nil, errors.Wrap(err, "could not decrease balance")
	}

	txid, err := SendOnChain(lncli, &lnrpc.SendCoinsRequest{
		Addr:       args.Address,
		Amount:     args.AmountSat,
		TargetConf: int32(args.TargetConf),
		SatPerByte: int64(args.SatPerByte),
	})
	if err != nil {
		if txErr := tx.Rollback(); txErr != nil {
			log.Error("txErr: ", txErr)
		}
		return nil, errors.Wrap(err, "could not send on-chain")
	}

	log.Debug("txid: ", txid)

	transaction, err := insertTransaction(tx, Transaction{
		UserID:      user.ID,
		Address:     args.Address,
		AmountSat:   args.AmountSat,
		Description: args.Description,
		Txid:        txid,
		Direction:   payments.OUTBOUND,
		Status:      "UNCONFIRMED",
	})
	if err != nil {
		if txErr := tx.Rollback(); txErr != nil {
			log.Error("txErr: ", txErr)
		}
		return nil, errors.Wrap(err, "could not insert transaction")
	}
	err = tx.Commit()
	if err != nil {
		return nil, errors.Wrap(err, "could not commit transaction")
	}

	log.Debugf("transaction: %+v", transaction)

	return &transaction, nil
}

type GetAddressArgs struct {
	// Whether to discard the old address and force create a new one
	ForceNewAddress bool `json:"forceNewAddress"`
	// A personal description for the transaction
	Description string `json:"description"`
}

// NewDeposit is a wrapper function for creating a new Deposit without a description
func NewDeposit(d *db.DB, lncli lnrpc.LightningClient, userID int) (Transaction, error) {
	return NewDepositWithDescription(d, lncli, userID, "")
}

// NewDepositWithDescription retrieves a new address from lnd, and saves the address
// in a new 'UNCONFIRMED', 'INBOUND' transaction together with the UserID
// Returns the same transaction as insertTransaction(), in full
func NewDepositWithDescription(d *db.DB, lncli lnrpc.LightningClient, userID int, description string) (Transaction, error) {
	address, err := lncli.NewAddress(context.Background(), &lnrpc.NewAddressRequest{
		// This type means lnd will force-create a new address
		Type: lnrpc.AddressType_NESTED_PUBKEY_HASH,
	})
	if err != nil {
		panic(err)
	}

	tx := d.MustBegin()
	transaction, err := insertTransaction(tx, Transaction{
		UserID:      userID,
		Address:     address.Address,
		Direction:   payments.INBOUND,
		Description: description,
		Status:      UNCONFIRMED,
	})
	if err != nil {
		log.Error(err)
		return Transaction{}, errors.Wrap(err, "could not insert new inbound transaction")
	}
	_ = tx.Commit()

	return transaction, nil
}

// GetDeposit attempts to retreive a deposit whose address has not yet received any coins
// It does this by selecting the last inserted deposit whose txid == "" (order by id desc)
// If the ForceNewAddress argument is true, or no deposit is found, the function creates a new deposit
func GetDeposit(d *db.DB, lncli lnrpc.LightningClient, userID int, args GetAddressArgs) (Transaction, error) {
	// If ForceNewAddress is supplied, we return a new deposit instantly
	if args.ForceNewAddress {
		return NewDepositWithDescription(d, lncli, userID, args.Description)
	}
	log.Infof("DepositOnChain(%d, %+v)", userID, args)

	// Get the latest INBOUND transaction whose txid is empty from the DB
	query := "SELECT * from transactions WHERE user_id=$1 AND direction='INBOUND' AND txid='' ORDER BY id DESC LIMIT 1;"
	var deposit Transaction
	err := d.Get(&deposit, query, userID)
	if err != nil {
		// In case the user has never made a deposit before, we get here
		// This error is OKAY
		if !strings.Contains(err.Error(), "no rows in result set") {
			log.Error(err)
			return Transaction{}, errors.Wrap(err, "could not get a transaction")
		}
	}

	// If the user has never made a deposit before, the query returns nothing,
	// and we need to create a new deposit
	log.Debugf("deposit  %+v", deposit)
	if deposit.UserID == 0 && deposit.Direction == "" {
		log.Debug("deposit == emptyTransaction, creating new")
		return NewDepositWithDescription(d, lncli, userID, args.Description)
	}

	// If we get here, we return the transaction the query returned
	log.Debug("returning found deposit")
	return deposit, nil
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
