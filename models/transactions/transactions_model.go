package transactions

import (
	"database/sql"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lightningnetwork/lnd/lnrpc"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"gitlab.com/arcanecrypto/teslacoil/db"
)

type txType string

const (
	lightning  txType = "lightning"
	blockchain txType = "blockchain"
)

var (
	_ json.Marshaler = Transaction{}
	_ json.Marshaler = Offchain{}
	_ json.Marshaler = Onchain{}
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
	CustomerOrderId *string `db:"customer_order_id"`

	// Expiry is the associated expiry time for a TX, if any. An offchain TX always
	// has an expiry, the one encoded into the invoice. An onchain TX may have an
	// expiry, for example when a merchant gives an offer to the consumer, but only
	// wants the offer to be valid for a certain time frame. On the other hand, a
	// TX that's a withdrawal from a merchant won't have this field set.
	Expiry *int64 `db:"expiry"`

	Direction Direction `db:"direction"`

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
	PaymentRequest *string         `db:"payment_request"`
	Preimage       *[]byte         `db:"preimage"`
	HashedPreimage *[]byte         `db:"hashed_preimage"`
	SettledAt      *time.Time      `db:"settled_at"` // If defined, it means the  invoice is settled
	Memo           *string         `db:"memo"`
	Status         *OffchainStatus `db:"invoice_status"`
}

// MarshalJSON is added to make sure that we never serialize raw Transactions
// directly, but instead convert them to the approriate specific type.
func (t Transaction) MarshalJSON() ([]byte, error) {
	if on, err := t.ToOnchain(); err == nil {
		return json.Marshal(on)
	}
	if off, err := t.ToOffchain(); err == nil {
		return json.Marshal(off)
	}
	panic("TX is neither offchain nor onchain!")
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

// IsExpired calculates whether a transaction is expired or not
func (t Transaction) IsExpired() bool {
	if t.Expiry == nil {
		return false
	}

	expiresAt := t.CreatedAt.Add(time.Second * time.Duration(*t.Expiry))

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
		ID:              t.ID,
		UserID:          t.UserID,
		CallbackURL:     t.CallbackURL,
		CustomerOrderId: t.CustomerOrderId,
		SettledAt:       t.SettledAt,
		Expiry:          t.Expiry,
		Direction:       t.Direction,
		AmountSat:       amountSat,
		Description:     t.Description,

		ConfirmedAtBlock: t.ConfirmedAtBlock,
		ConfirmedAt:      t.ConfirmedAt,

		Address: *t.Address,
		Txid:    t.Txid,
		Vout:    t.Vout,

		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
		DeletedAt: t.DeletedAt,
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
		Expiry:          *t.Expiry,
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

// offchainNoJson is a simple custom type we introduce to avoid a stack
// overflow error when serializing an offchain TX with additional fields.
// Type aliases break interface satisfaction when using embedded structs,
// which causes the JSON marshaller to fallback to using struct tags.
type offchainNoJson Offchain

// offchainWithDerived is a struct that embed a regular offchain TX, but adds
// certain fields that we derive from the other fields. This struct is used
// when serializing the data to JSON, as this ensures we include all fields the
// user might be interested in, without having to duplicate logic all over the
// place.
type offchainWithDerived struct {
	offchainNoJson

	Expired   bool      `json:"expired"`
	ExpiresAt time.Time `json:"expiresAt"`
	AmountSat int64     `json:"amountSat"`
	Type      txType    `json:"type"`
}

// Offchain is the db-type for an offchain transaction
type Offchain struct {
	ID          int     `json:"id"`
	UserID      int     `json:"userId"`
	CallbackURL *string `json:"callbackUrl,omitempty"`
	// CustomerOrderId is an optional field where the user can specify an
	// order ID of their choosing. The only place this is used is when hitting
	// the callback URL of a transaction.
	CustomerOrderId *string `json:"customerOrderId,omitempty"`

	Expiry int64 `json:"-"`

	AmountMSat     int64          `json:"amountMSat"`
	Description    *string        `json:"description,omitempty"`
	Direction      Direction      `json:"direction"`
	HashedPreimage []byte         `json:"hash"`
	PaymentRequest string         `json:"paymentRequest"`
	Preimage       []byte         `json:"preimage"`
	Memo           *string        `json:"memo,omitempty"`
	Status         OffchainStatus `json:"status"`

	SettledAt *time.Time `json:"settledAt,omitempty"` // If defined, it means the  invoice is settled
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"-"`
	DeletedAt *time.Time `json:"-"`
}

func (o Offchain) MarshalJSON() ([]byte, error) {
	return json.Marshal(o.withAdditionalFields())
}

// ToTransaction converts a Offchain struct into a Transaction
func (o Offchain) ToTransaction() Transaction {
	return Transaction{
		ID:              o.ID,
		UserID:          o.UserID,
		CallbackURL:     o.CallbackURL,
		CustomerOrderId: o.CustomerOrderId,
		Expiry:          &o.Expiry,
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

func (o Offchain) withAdditionalFields() offchainWithDerived {
	expiresAt := o.CreatedAt.Add(time.Second * time.Duration(o.Expiry))
	return offchainWithDerived{
		offchainNoJson: offchainNoJson(o),
		Type:           lightning,
		Expired:        expiresAt.Before(time.Now()),
		ExpiresAt:      expiresAt,
		AmountSat:      o.AmountMSat / 1000,
	}
}

func (o Offchain) IsExpired() bool {
	return o.withAdditionalFields().Expired
}

func (o Offchain) ExpiresAt() time.Time {
	return o.withAdditionalFields().ExpiresAt
}

// Direction is the direction of a transaction, seen from the users perspective
type Direction string

const (
	INBOUND  Direction = "INBOUND"
	OUTBOUND Direction = "OUTBOUND"
)

// MarshalText overrides the MarshalText function in the json package
func (d Direction) MarshalText() (text []byte, err error) {
	lower := strings.ToLower(string(d))
	return []byte(lower), nil
}

var _ encoding.TextMarshaler = INBOUND

// OffchainStatus is the status of a offchain transaction
type OffchainStatus string

const (
	Offchain_CREATED   OffchainStatus = "CREATED"
	Offchain_SENT      OffchainStatus = "SENT"
	Offchain_COMPLETED OffchainStatus = "COMPLETED"
	Offchain_FLOPPED   OffchainStatus = "FLOPPED"
)

var _ encoding.TextMarshaler = Offchain_COMPLETED

// InvoiceStateToTeslaState maps lnd's InvoiceState to our OffchainStatus
// InvoiceState are states for invoices belonging to our node, created
// using lncli.AddInvoice()
// Example usage: status := InvoiceStateToTeslaState[invoice.Status]
var InvoiceStateToTeslaState = map[lnrpc.Invoice_InvoiceState]OffchainStatus{
	lnrpc.Invoice_OPEN:    Offchain_CREATED,
	lnrpc.Invoice_SETTLED: Offchain_COMPLETED,
}

// PaymentStateToTeslaState maps lnd's PaymentStatus to our OffchainStatus
// PaymentStatus are states of payments, e.g: outbound payments (lncli.SendPayment())
// Example usage: status := PaymentStateToTeslaState[payment.status]
var PaymentStateToTeslaState = map[lnrpc.Payment_PaymentStatus]OffchainStatus{
	lnrpc.Payment_UNKNOWN:   Offchain_CREATED,
	lnrpc.Payment_IN_FLIGHT: Offchain_SENT,
	lnrpc.Payment_SUCCEEDED: Offchain_COMPLETED,
	lnrpc.Payment_FAILED:    Offchain_FLOPPED,
}

// MarshalText overrides the MarshalText function in the json package
func (s OffchainStatus) MarshalText() (text []byte, err error) {
	lower := strings.ToLower(string(s))
	return []byte(lower), nil
}

// MarkAsPaid marks the given payment request as paid at the given date
func (o Offchain) MarkAsPaid(db db.Inserter, paidAt time.Time) (Offchain, error) {
	updateOffchainTxQuery := `UPDATE transactions
		SET settled_at = :settled_at, invoice_status = :invoice_status
		WHERE id = :id ` + txReturningSql

	log.WithField("paymentRequest", o.PaymentRequest).Info("Marking invoice as paid")

	o.SettledAt = &paidAt
	o.Status = Offchain_COMPLETED
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
	if err = rows.StructScan(&updated); err != nil {
		return Offchain{}, err
	}

	updatedOffchain, err := updated.ToOffchain()
	if err != nil {
		return Offchain{}, err
	}

	return updatedOffchain, nil
}

// MarkAsFlopped marks the transaction as failed
func (o Offchain) MarkAsFlopped(db db.Inserter) (Offchain, error) {
	updateOffchainTxQuery := `UPDATE transactions 
		SET invoice_status = :invoice_status
		WHERE id = :id ` + txReturningSql

	log.WithField("paymentRequest", o.PaymentRequest).Info("Marking invoice as paid")

	o.Status = Offchain_FLOPPED
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
		fmt.Sprintf("AmountMSat: %d", o.AmountMSat),
		fmt.Sprintf("SettledAt: %v", o.SettledAt),
		fmt.Sprintf("CreatedAt: %v", o.CreatedAt),
		fmt.Sprintf("UpdatedAt: %v", o.UpdatedAt),
		fmt.Sprintf("DeletedAt: %v }", o.DeletedAt),
	)

	return strings.Join(fragments, ", ")
}

// onchainNoJson is a simple custom type we introduce to avoid a stack
// overflow error when serializing an onchain TX with additional fields.
// Type aliases break interface satisfaction when using embedded structs,
// which causes the JSON marshaller to fallback to using struct tags.

type onchainNoJson Onchain

// onchainWithDerived is a struct that embed a regular onchain TX, but adds
// certain fields that we derive from the other fields. This struct is used
// when serializing the data to JSON, as this ensures we include all fields the
// user might be interested in, without having to duplicate logic all over the
// place
type onchainWithDerived struct {
	onchainNoJson
	Expired   *bool      `json:"expired,omitempty"`
	ExpiresAt *time.Time `json:"expiry,omitempty"`
	Confirmed bool       `json:"confirmed"`
	Type      txType     `json:"type"`
}

// Onchain is the struct for an onchain transaction
type Onchain struct {
	ID          int     `json:"id"`
	UserID      int     `json:"userId"`
	CallbackURL *string `json:"callbackUrl,omitempty"`
	// CustomerOrderId is an optional field where the user can specify an
	// order ID of their choosing. The only place this is used is when hitting
	// the callback URL of a transaction.
	CustomerOrderId *string `json:"customerOrderId,omitempty"`

	Direction Direction `json:"direction"`

	Description *string `json:"description,omitempty"`

	// Some onchain TXs may have an expiry time associated with them. Typically
	// this would be done where a merchant wants to give an offer to the consumer
	// without comitting to the price for too long.
	Expiry *int64 `json:"expiry,omitempty"`

	// TODO doc
	SettledAt *time.Time `json:"settledAt,omitempty"`

	ConfirmedAtBlock *int       `json:"confirmedAtBlock,omitempty"`
	ConfirmedAt      *time.Time `json:"confirmedAt,omitempty"`

	AmountSat *int64  `json:"amountSat,omitempty"`
	Address   string  `json:"address"`
	Txid      *string `json:"txid,omitempty"`
	Vout      *int    `json:"vout,omitempty"`

	CreatedAt time.Time  `json:"-"`
	UpdatedAt time.Time  `json:"-"`
	DeletedAt *time.Time `json:"-"`
}

func (o Onchain) MarshalJSON() ([]byte, error) {
	return json.Marshal(o.withAdditionalFields())
}

func (o Onchain) withAdditionalFields() onchainWithDerived {
	var expiresAt *time.Time
	var expired *bool
	if o.Expiry != nil {
		eAt := o.CreatedAt.Add(time.Second * time.Duration(*o.Expiry))
		expiresAt = &eAt
		e := expiresAt.After(time.Now())
		expired = &e
	}
	return onchainWithDerived{
		onchainNoJson: onchainNoJson(o),
		Type:          blockchain,
		Expired:       expired,
		ExpiresAt:     expiresAt,
		Confirmed:     o.ConfirmedAt != nil,
	}
}

// ToTransaction converts a Onchain struct into a Transaction
func (o Onchain) ToTransaction() Transaction {
	var amountMsat *int64
	if o.AmountSat != nil {
		a := *o.AmountSat * 1000
		amountMsat = &a
	}
	return Transaction{
		ID:              o.ID,
		UserID:          o.UserID,
		CallbackURL:     o.CallbackURL,
		CustomerOrderId: o.CustomerOrderId,
		AmountMSat:      amountMsat,
		SettledAt:       o.SettledAt,
		Expiry:          o.Expiry,
		Direction:       o.Direction,
		Description:     o.Description,

		ConfirmedAtBlock: o.ConfirmedAtBlock,
		ConfirmedAt:      o.ConfirmedAt,

		// Preimage:         nil, // otherwise we get empty slice
		// HashedPreimage:   nil, // otherwise we get empty slice

		Address: &o.Address,
		Txid:    o.Txid,
		Vout:    o.Vout,

		CreatedAt: o.CreatedAt,
		UpdatedAt: o.UpdatedAt,
		DeletedAt: o.DeletedAt,
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
