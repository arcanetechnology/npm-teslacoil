package transactions

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
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
	CustomerOrderId *string   `db:"customer_order_id"`
	Expiry          int64     `db:"expiry"` // Encoded into invoice if offchain, internally only if onchain
	Expired         bool      `db:"-"`
	ExpiresAt       time.Time `db:"-"`
	Direction       Direction `db:"direction"`

	// AmountMsat is the amount of money spent to a transaction. In the case of this being an offchain TX the field
	// is set to the amount in the associated invoice. In that case it should always be non-nil. In the case of an
	// onchain transaction, it is nil until the transaction (address) has had money spent to it.
	AmountMSat *int64 `db:"amount_milli_sat"`

	Description *string    `db:"description"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
	DeletedAt   *time.Time `db:"deleted_at"`

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

func (t Transaction) String() string {
	if on, err := t.ToOnchain(); err == nil {
		return on.String()
	}
	if off, err := t.ToOffchain(); err == nil {
		return off.String()
	}
	panic("TX is neither offchain nor onchain!")
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

// ToOnchain converts a transaction into an onchain transaction
func (t Transaction) ToOnchain() (Onchain, error) {
	if t.Address == nil {
		return Onchain{}, errors.New("transaction was offchain")
	}
	var amountSat *int64
	if t.AmountMSat != nil {
		a := *t.AmountMSat / 1000
		amountSat = &a
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
		AmountSat:        amountSat,
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
func (t Transaction) ToOffchain() (Offchain, error) {
	if t.PaymentRequest == nil || t.AmountMSat == nil {
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
		AmountSat:       *t.AmountMSat / 1000,
		AmountMSat:      *t.AmountMSat,
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
		AmountMSat:      &o.AmountMSat,
		SettledAt:       o.SettledAt,
		Memo:            o.Memo,
		Status:          &o.Status,
		CreatedAt:       o.CreatedAt,
		UpdatedAt:       o.UpdatedAt,
		DeletedAt:       o.DeletedAt,
	}
}

func (o Offchain) WithAdditionalFields() Offchain {
	withFields := o.ToTransaction().WithAdditionalFields()
	offWithFields, err := withFields.ToOffchain()
	if err != nil {
		panic("Could not convert back to offchain TX!")
	}
	return offWithFields
}

// MarkAsPaid marks the given payment request as paid at the given date
func (o Offchain) MarkAsPaid(db db.Inserter, paidAt time.Time) (Offchain, error) {
	updateOffchainTxQuery := `UPDATE transactions
		SET settled_at = :settled_at, invoice_status = :invoice_status
		WHERE id = :id ` + txReturningSql

	log.WithField("paymentRequest", o.PaymentRequest).Info("Marking invoice as paid")

	o.SettledAt = &paidAt
	o.Status = SUCCEEDED
	tx := o.ToTransaction()
	rows, err := db.NamedQuery(updateOffchainTxQuery, &tx)
	if err != nil {
		log.WithError(err).Error("Couldn't mark invoice as paid")
		return Offchain{}, err
	}

	if !rows.Next() {
		return Offchain{}, fmt.Errorf("could not mark invoice as paid: %w", sql.ErrNoRows)
	}

	var updated Transaction
	if err := rows.StructScan(&updated); err != nil {
		return Offchain{}, err
	}

	updatedOffchain, err := updated.ToOffchain()
	if err != nil {
		return Offchain{}, err
	}

	return updatedOffchain, nil
}

// MarkAsFailed marks the transaction as failed
func (o Offchain) MarkAsFailed(db db.Inserter) (Offchain, error) {
	updateOffchainTxQuery := `UPDATE transactions 
		SET invoice_status = :invoice_status
		WHERE id = :id ` + txReturningSql

	log.WithField("paymentRequest", o.PaymentRequest).Info("Marking invoice as paid")

	o.Status = FAILED
	tx := o.ToTransaction()
	rows, err := db.NamedQuery(updateOffchainTxQuery, &tx)
	if err != nil {
		log.WithError(err).Errorf("Couldn't mark invoice as failed")
		return Offchain{}, err
	}
	if !rows.Next() {
		return Offchain{}, fmt.Errorf("couldn't mark invoice as failed: %w", sql.ErrNoRows)
	}

	var updated Transaction
	if err := rows.StructScan(&updated); err != nil {
		return Offchain{}, err
	}

	updatedOffchain, err := updated.ToOffchain()
	if err != nil {
		return Offchain{}, err
	}

	return updatedOffchain, nil
}

func (o Offchain) String() string {
	fragments := []string{
		fmt.Sprintf("Offchain: {ID: %d", o.ID),
		fmt.Sprintf("UserID: %d", o.UserID),
		fmt.Sprintf("PaymentRequest: %s", o.PaymentRequest),
		fmt.Sprintf("Preimage: %x", o.Preimage),
		fmt.Sprintf("HashedPreimage: %x", o.HashedPreimage),
		fmt.Sprintf("Status: %s", o.Status),
	}

	if o.Memo != nil {
		fragments = append(fragments, fmt.Sprintf("Memo: %s", *o.Memo))
	}
	if o.Description != nil {
		fragments = append(fragments, fmt.Sprintf("Description: %s", *o.Description))
	}
	if o.CallbackURL != nil {
		fragments = append(fragments, fmt.Sprintf("CallbackURL: %s", *o.CallbackURL))
	}

	fragments = append(fragments,
		fmt.Sprintf("Expiry: %d", o.Expiry),
		fmt.Sprintf("Direction: %s", o.Direction),
		fmt.Sprintf("AmountSat: %d", o.AmountSat),
		fmt.Sprintf("AmountMSat: %d", o.AmountMSat),
		fmt.Sprintf("Expired: %t", o.Expired),
		fmt.Sprintf("ExpiresAt: %v", o.ExpiresAt),
		fmt.Sprintf("SettledAt: %v", o.SettledAt),
		fmt.Sprintf("CreatedAt: %v", o.CreatedAt),
		fmt.Sprintf("UpdatedAt: %v", o.UpdatedAt),
		fmt.Sprintf("DeletedAt: %v }", o.DeletedAt),
	)

	return strings.Join(fragments, ", ")
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

	AmountSat        *int64  `json:"amountSat"`
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
	var amountMsat *int64
	if o.AmountSat != nil {
		a := *o.AmountSat * 1000
		amountMsat = &a
	}
	return Transaction{
		ID:               o.ID,
		UserID:           o.UserID,
		CallbackURL:      o.CallbackURL,
		CustomerOrderId:  o.CustomerOrderId,
		Expiry:           o.Expiry,
		AmountMSat:       amountMsat,
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

	tx := o.ToTransaction()
	query := `UPDATE transactions
		SET confirmed_at = :confirmed_at, confirmed_at_block = :confirmed_at_block
		WHERE id = :id` + txReturningSql

	rows, err := db.NamedQuery(query, &tx)
	if err != nil {
		return Onchain{}, err
	}

	if !rows.Next() {
		return Onchain{}, fmt.Errorf("could not mark TX as confirmed: %w", sql.ErrNoRows)
	}

	var updatedTx Transaction
	if err := rows.StructScan(&updatedTx); err != nil {
		return Onchain{}, err
	}

	updatedOnchain, err := tx.ToOnchain()
	if err != nil {
		return Onchain{}, err
	}

	return updatedOnchain, nil
}

// PersistReceivedMoney saves a TX consisting of a TXID, a vout and an amount to the
// DB transaction. If the Onchain transaction already has received money (i.e.
// has a TXID) the method errors.
func (o Onchain) PersistReceivedMoney(db db.Inserter, txid chainhash.Hash, vout int,
	amountSat int64) (Onchain, error) {

	if vout < 0 {
		return Onchain{}, errors.New("vout must be non-negative when adding money to onchain TX")
	}

	if amountSat < 1 {
		return Onchain{}, errors.New("amount must be positive when adding money to onchain TX")
	}

	if o.Txid != nil {
		return Onchain{}, ErrTxHasTxid
	}
	txidStr := txid.String()
	o.Txid = &txidStr
	o.Vout = &vout
	o.AmountSat = &amountSat

	tx := o.ToTransaction()

	rows, err := db.NamedQuery(
		`UPDATE transactions SET txid = :txid, vout = :vout, amount_milli_sat = :amount_milli_sat 
			WHERE id = :id AND txid IS NULL AND vout IS NULL AND amount_milli_sat IS NULL `+txReturningSql,
		&tx)
	if err != nil {
		return Onchain{}, err
	}

	if !rows.Next() {
		return Onchain{}, fmt.Errorf("could not update TX when adding received money: %w", sql.ErrNoRows)
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

func (o Onchain) String() string {
	fragments := []string{
		fmt.Sprintf("Onchain: {ID: %d", o.ID),
		fmt.Sprintf("UserID: %d", o.UserID),
		fmt.Sprintf("Expiry: %d", o.Expiry),
		fmt.Sprintf("Address: %s", o.Address),
		fmt.Sprintf("Direction: %s", o.Direction),
	}

	if o.Description != nil {
		fragments = append(fragments, fmt.Sprintf("Description: %s", *o.Description))
	}
	if o.CallbackURL != nil {
		fragments = append(fragments, fmt.Sprintf("CallbackURL: %s", *o.CallbackURL))
	}
	if o.AmountSat != nil {
		fragments = append(fragments, fmt.Sprintf("AmountSat: %d", *o.AmountSat))
	}
	if o.Txid != nil {
		fragments = append(fragments, fmt.Sprintf("Txid: %s", *o.Txid))
	}
	if o.Vout != nil {
		fragments = append(fragments, fmt.Sprintf("Vout: %d", *o.Vout))
	}
	if o.ConfirmedAtBlock != nil {
		fragments = append(fragments, fmt.Sprintf("ConfirmedAtBlock: %d", *o.ConfirmedAtBlock))
	}
	if o.ConfirmedAt != nil {
		fragments = append(fragments, fmt.Sprintf("ConfirmedAt: %s", *o.ConfirmedAt))
	}
	if o.SettledAt != nil {
		fragments = append(fragments, fmt.Sprintf("SettledAt: %s", *o.SettledAt))
	}

	return strings.Join(fragments, ", ")
}

const txReturningSql = ` RETURNING id, user_id, callback_url, customer_order_id, expiry, direction, amount_milli_sat, description, 
	    confirmed_at_block, confirmed_at, address, txid, vout, payment_request, preimage, 
	    hashed_preimage, settled_at, memo, invoice_status, created_at, updated_at, deleted_at`

func insertTransaction(db db.Inserter, t Transaction) (Transaction, error) {
	createTransactionQuery := `
	INSERT INTO transactions (user_id, callback_url, customer_order_id, expiry, direction, amount_milli_sat, description, 
	                          confirmed_at_block, confirmed_at, address, txid, vout, payment_request, preimage, 
	                          hashed_preimage, settled_at, memo, invoice_status)
	VALUES (:user_id, :callback_url, :customer_order_id, :expiry, :direction, :amount_milli_sat, :description, 
	        :confirmed_at_block, :confirmed_at, :address, :txid, :vout, :payment_request, :preimage, 
	        :hashed_preimage, :settled_at, :memo, :invoice_status)` + txReturningSql

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
