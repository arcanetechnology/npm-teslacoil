package transactions

import (
	"errors"
	"fmt"
	"time"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"gitlab.com/arcanecrypto/teslacoil/db"
)

// Transaction is the DB type for a transaction in Teslacoil. This
// includes both offchain and onchain TXs. This struct is only responsible
// for handling DB serialization and deserialization.
type Transaction struct {
	// common fields
	ID          int     `db:"id"`
	UserID      int     `db:"user_id"`
	CallbackURL *string `db:"callback_url"`
	// CustomerOrderId is an optional field where the user can specify an
	// order ID of their choosing. The only place this is used is when hitting
	// the callback URL of a transaction.
	CustomerOrderId *string    `db:"customer_order_id"`
	Expiry          int64      `db:"expiry"` // Encoded into invoice if offchain, internally only if onchain
	Expired         bool       `db:"-"`
	ExpiresAt       time.Time  `db:"-"`
	Direction       Direction  `db:"direction"`
	AmountMSat      int64      `db:"amount_milli_sat"`
	Description     *string    `db:"description"`
	CreatedAt       time.Time  `db:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at"`
	DeletedAt       *time.Time `db:"deleted_at"`

	// onchain fields
	Address          *string    `db:"address"`
	Txid             *string    `db:"txid"`
	Vout             *int       `db:"vout"`
	ConfirmedAtBlock *int       `db:"confirmed_at_block"`
	ConfirmedAt      *time.Time `db:"confirmed_at"`

	// offchain fields
	PaymentRequest *string    `db:"payment_request"`
	Preimage       *[]byte    `db:"preimage"`
	HashedPreimage *[]byte    `db:"hashed_preimage"`
	SettledAt      *time.Time `db:"settled_at"` // If defined, it means the  invoice is settled
	Memo           *string    `db:"memo"`
	Status         *Status    `db:"invoice_status"`
}

// WithAdditionalFields adds useful fields that's derived from information stored
// in the DB.
func (t Transaction) WithAdditionalFields() Transaction {
	t.ExpiresAt = t.GetExpiryDate()
	t.Expired = t.IsExpired()

	return t
}

// GetExpiryDate converts the Expiry field to a more human-friendly format
func (t Transaction) GetExpiryDate() time.Time {
	return t.CreatedAt.Add(time.Second * time.Duration(t.Expiry))
}

// IsExpired calculates whether a transaction is expired or not
func (t Transaction) IsExpired() bool {
	expiresAt := t.CreatedAt.Add(time.Second * time.Duration(t.Expiry))

	// Return whether the expiry date is before the time now
	// We get the UTC time because the db is in UTC time
	return expiresAt.Before(time.Now().UTC())
}

// ToOnChain converst a transaction into an onchain transaction
func (t Transaction) ToOnchain() (Onchain, error) {
	if t.Address == nil {
		return Onchain{}, errors.New("transaction was offchain")
	}
	on := Onchain{
		ID:               t.ID,
		UserID:           t.UserID,
		CallbackURL:      t.CallbackURL,
		CustomerOrderId:  t.CustomerOrderId,
		Expiry:           t.Expiry,
		Expired:          t.Expired,
		ExpiresAt:        t.ExpiresAt,
		Direction:        t.Direction,
		AmountSat:        t.AmountMSat / 1000,
		Description:      t.Description,
		ConfirmedAtBlock: t.ConfirmedAtBlock,
		Address:          *t.Address,
		Txid:             t.Txid,
		Vout:             t.Vout,
		SettledAt:        t.SettledAt,
		ConfirmedAt:      t.ConfirmedAt,
		CreatedAt:        t.CreatedAt,
		UpdatedAt:        t.UpdatedAt,
		DeletedAt:        t.DeletedAt,
	}

	return on, nil
}

// ToOffchain converst a transaction into an offchain transaction
func (t Transaction) ToOffChain() (Offchain, error) {
	if t.PaymentRequest == nil {
		return Offchain{}, errors.New("TX was onchain")
	}
	off := Offchain{
		ID:              t.ID,
		UserID:          t.UserID,
		CallbackURL:     t.CallbackURL,
		CustomerOrderId: t.CustomerOrderId,
		Expiry:          t.Expiry,
		Expired:         t.Expired,
		ExpiresAt:       t.ExpiresAt,
		AmountSat:       t.AmountMSat / 1000,
		AmountMSat:      t.AmountMSat,
		Description:     t.Description,
		Direction:       t.Direction,
		HashedPreimage:  *t.HashedPreimage,
		PaymentRequest:  *t.PaymentRequest,
		Preimage:        *t.Preimage,
		Memo:            t.Memo,
		Status:          *t.Status,
		SettledAt:       t.SettledAt,
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
		DeletedAt:       t.DeletedAt,
	}

	// if preimage is NULL in DB, default is empty slice and not null
	if t.Preimage != nil && len(*t.Preimage) == 0 {
		off.Preimage = nil
	}

	return off, nil
}

// Offchain is the db-type for an offchain transaction
type Offchain struct {
	ID          int     `json:"id"`
	UserID      int     `json:"userId"`
	CallbackURL *string `json:"callbackUrl"`
	// CustomerOrderId is an optional field where the user can specify an
	// order ID of their choosing. The only place this is used is when hitting
	// the callback URL of a transaction.
	CustomerOrderId *string `json:"customerOrderId"`

	Expiry    int64     `json:"-"`
	Expired   bool      `json:"expired"`
	ExpiresAt time.Time `json:"expiry"`

	AmountSat      int64     `json:"amountSat"`
	AmountMSat     int64     `json:"amountMSat"`
	Description    *string   `json:"description"`
	Direction      Direction `json:"direction"`
	HashedPreimage []byte    `json:"hash"`
	PaymentRequest string    `json:"paymentRequest"`
	Preimage       []byte    `json:"preimage"`
	Memo           *string   `json:"memo"`
	Status         Status    `json:"status"`

	SettledAt *time.Time `json:"settledAt"` // If defined, it means the  invoice is settled
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"-"`
	DeletedAt *time.Time `json:"-"`
}

// ToTransaction converts a Offchain struct into a Transaction
func (o Offchain) ToTransaction() Transaction {
	return Transaction{
		ID:              o.ID,
		UserID:          o.UserID,
		CallbackURL:     o.CallbackURL,
		CustomerOrderId: o.CustomerOrderId,
		Expiry:          o.Expiry,
		Expired:         o.Expired,
		ExpiresAt:       o.ExpiresAt,
		Direction:       o.Direction,
		Description:     o.Description,
		PaymentRequest:  &o.PaymentRequest,
		Preimage:        &o.Preimage,
		HashedPreimage:  &o.HashedPreimage,
		AmountMSat:      o.AmountMSat,
		SettledAt:       o.SettledAt,
		Memo:            o.Memo,
		Status:          &o.Status,
		CreatedAt:       o.CreatedAt,
		UpdatedAt:       o.UpdatedAt,
		DeletedAt:       o.DeletedAt,
	}
}

// Onchain is the struct for an onchain transaction
type Onchain struct {
	ID          int     `json:"id"`
	UserID      int     `json:"userId"`
	CallbackURL *string `json:"callbackUrl"`
	// CustomerOrderId is an optional field where the user can specify an
	// order ID of their choosing. The only place this is used is when hitting
	// the callback URL of a transaction.
	CustomerOrderId *string `json:"customerOrderId"`

	Expiry    int64     `json:"expiry"`
	Expired   bool      `db:"-"`
	ExpiresAt time.Time `db:"-"`

	Direction Direction `json:"direction"`

	// TODO make nullable
	AmountSat        int64   `json:"amountSat"`
	Description      *string `json:"description"`
	ConfirmedAtBlock *int    `json:"confirmedAtBlock"`
	Address          string  `json:"address"`
	Txid             *string `json:"txid"`
	Vout             *int    `json:"vout"`

	// TODO doc
	SettledAt   *time.Time `json:"settledAt"`
	ConfirmedAt *time.Time `json:"confirmedAt"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"-"`
	DeletedAt   *time.Time `json:"-"`
}

// ToTransaction converts a Onchain struct into a Transaction
func (o Onchain) ToTransaction() Transaction {
	return Transaction{
		ID:               o.ID,
		UserID:           o.UserID,
		CallbackURL:      o.CallbackURL,
		CustomerOrderId:  o.CustomerOrderId,
		Expiry:           o.Expiry,
		AmountMSat:       o.AmountSat * 1000,
		Expired:          o.Expired,
		ExpiresAt:        o.ExpiresAt,
		Direction:        o.Direction,
		Description:      o.Description,
		ConfirmedAtBlock: o.ConfirmedAtBlock,
		// Preimage:         nil, // otherwise we get empty slice
		// HashedPreimage:   nil, // otherwise we get empty slice
		Address:     &o.Address,
		Txid:        o.Txid,
		Vout:        o.Vout,
		SettledAt:   o.SettledAt,
		ConfirmedAt: o.ConfirmedAt,
		CreatedAt:   o.CreatedAt,
		UpdatedAt:   o.UpdatedAt,
		DeletedAt:   o.DeletedAt,
	}
}

// MarkAsConfirmed updates the transaction stored in the DB
// with Confirmed = true and ConfirmedAt = Now().
func (o Onchain) MarkAsConfirmed(db db.Inserter, height int) (Onchain, error) {

	if o.Txid == nil {
		return Onchain{}, errors.New("cannot mark a TX as confirmed when it hasn't received any money")
	}

	now := time.Now()
	o.ConfirmedAt = &now
	o.ConfirmedAtBlock = &height

	query := `UPDATE transactions
		SET confirmed_at = :confirmed_at, confirmed = :confirmed
		WHERE id=:id`

	rows, err := db.NamedQuery(query, &o)
	if err != nil {
		return Onchain{}, nil
	}

	var updated Onchain
	if err := rows.StructScan(&updated); err != nil {
		return Onchain{}, err
	}

	return updated, nil
}

// AddReceivedMoney saves a TX consisting of a TXID, a vout and an amount to the
// DB transaction. If the Onchain transaction already has received money (i.e.
// has a TXID) the method errors.
func (o Onchain) AddReceivedMoney(db db.Inserter, txid chainhash.Hash, vout int,
	amountSat int64) (Onchain, error) {

	if o.Txid != nil {
		return Onchain{}, ErrTxHasTxid
	}
	txidStr := txid.String()
	o.Txid = &txidStr
	o.Vout = &vout
	o.AmountSat = amountSat

	tx := o.ToTransaction()

	rows, err := db.NamedQuery(
		`UPDATE transactions SET txid = :txid, vout = :vout, amount_milli_sat = :amount_milli_sat WHERE id = :id`,
		&tx)
	if err != nil {
		return Onchain{}, err
	}

	var inserted Transaction
	if err := rows.StructScan(&inserted); err != nil {
		return Onchain{}, err
	}

	insertedOnchain, err := inserted.ToOnchain()
	if err != nil {
		return Onchain{}, err
	}

	return insertedOnchain, nil
}

func insertTransaction(db db.Inserter, t Transaction) (Transaction, error) {
	createTransactionQuery := `
	INSERT INTO transactions (user_id, callback_url, customer_order_id, expiry, direction, amount_milli_sat, description, 
	                          confirmed_at_block, confirmed_at, address, txid, vout, payment_request, preimage, 
	                          hashed_preimage, settled_at, memo, invoice_status)
	VALUES (:user_id, :callback_url, :customer_order_id, :expiry, :direction, :amount_milli_sat, :description, 
	        :confirmed_at_block, :confirmed_at, :address, :txid, :vout, :payment_request, :preimage, 
	        :hashed_preimage, :settled_at, :memo, :invoice_status)
	RETURNING id, user_id, callback_url, customer_order_id, expiry, direction, amount_milli_sat, description, 
	    confirmed_at_block, confirmed_at, address, txid, vout, payment_request, preimage, 
	    hashed_preimage, settled_at, memo, invoice_status, created_at, updated_at, deleted_at`

	rows, err := db.NamedQuery(createTransactionQuery, t)
	if err != nil {
		return Transaction{}, fmt.Errorf("could not insert transaction: %w", err)
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
			log.WithError(err).Error("could not scan result into transaction struct")
			return Transaction{}, fmt.Errorf("could not insert transaction: %w", err)
		}
	}

	return transaction, nil
}
