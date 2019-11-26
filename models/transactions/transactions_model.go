package transactions

import (
	"database/sql"
	"encoding"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/models/users/balance"

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
	AmountMilliSat *int64 `db:"amount_milli_sat"`

	// InternalTransfer marks whether this payment was a transaction to
	// another teslacoil user
	InternalTransfer bool `db:"internal_transfer"`

	Description *string    `db:"description"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
	DeletedAt   *time.Time `db:"deleted_at"`

	// onchain fields
	Address          *string    `db:"address"`
	Txid             *string    `db:"txid"`
	Vout             *int       `db:"vout"`
	ReceivedMoneyAt  *time.Time `db:"received_tx_at"`
	ConfirmedAtBlock *int       `db:"confirmed_at_block"`
	ConfirmedAt      *time.Time `db:"confirmed_at"`

	// offchain fields
	PaymentRequest *string         `db:"payment_request"`
	Preimage       *[]byte         `db:"preimage"`
	HashedPreimage *[]byte         `db:"hashed_preimage"`
	SettledAt      *time.Time      `db:"settled_at"` // If defined, it means the  invoice is settled
	Memo           *string         `db:"memo"`
	Status         *OffchainStatus `db:"invoice_status"`
	Error          *string         `db:"payment_error"`
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
	if t.AmountMilliSat != nil {
		a := *t.AmountMilliSat / 1000
		amountSat = &a
	}
	on := Onchain{
		ID:               t.ID,
		UserID:           t.UserID,
		CallbackURL:      t.CallbackURL,
		CustomerOrderId:  t.CustomerOrderId,
		SettledAt:        t.SettledAt,
		Expiry:           t.Expiry,
		Direction:        t.Direction,
		AmountSat:        amountSat,
		Description:      t.Description,
		InternalTransfer: t.InternalTransfer,

		ConfirmedAtBlock: t.ConfirmedAtBlock,
		ConfirmedAt:      t.ConfirmedAt,

		Address:         *t.Address,
		Txid:            t.Txid,
		ReceivedMoneyAt: t.ReceivedMoneyAt,
		Vout:            t.Vout,

		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
		DeletedAt: t.DeletedAt,
	}

	return on, nil
}

// ToOffchain converst a transaction into an offchain transaction
func (t Transaction) ToOffchain() (Offchain, error) {
	if t.PaymentRequest == nil || t.AmountMilliSat == nil {
		return Offchain{}, errors.New("TX was onchain")
	}

	a := balance.Balance(*t.AmountMilliSat)
	amountSat := a.Sats()

	off := Offchain{
		ID:               t.ID,
		UserID:           t.UserID,
		CallbackURL:      t.CallbackURL,
		CustomerOrderId:  t.CustomerOrderId,
		Expiry:           *t.Expiry,
		InternalTransfer: t.InternalTransfer,
		AmountSat:        amountSat,
		AmountMilliSat:   *t.AmountMilliSat,
		Description:      t.Description,
		Direction:        t.Direction,
		Preimage:         *t.Preimage,
		HashedPreimage:   *t.HashedPreimage,
		PaymentRequest:   *t.PaymentRequest,
		Memo:             t.Memo,
		Status:           *t.Status,
		Error:            t.Error,
		SettledAt:        t.SettledAt,
		CreatedAt:        t.CreatedAt,
		UpdatedAt:        t.UpdatedAt,
		DeletedAt:        t.DeletedAt,
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

	HexHashedPreimage string    `json:"hash"`
	HexPreimage       string    `json:"preimage"`
	Expired           bool      `json:"expired"`
	ExpiresAt         time.Time `json:"expiresAt"`
	AmountSat         int64     `json:"amountSat"`
	Type              txType    `json:"type"`
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

	Expiry           int64 `json:"-"`
	InternalTransfer bool  `json:"-"`

	AmountSat      int64          `json:"amountSat"`
	AmountMilliSat int64          `json:"amountMilliSat"`
	Description    *string        `json:"description,omitempty"`
	Direction      Direction      `json:"direction"`
	HashedPreimage []byte         `json:"-"`
	Preimage       []byte         `json:"-"`
	PaymentRequest string         `json:"paymentRequest"`
	Memo           *string        `json:"memo,omitempty"`
	Status         OffchainStatus `json:"status"`
	Error          *string        `json:"error,omitempty"` // PaymentError from LND

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
		ID:               o.ID,
		UserID:           o.UserID,
		CallbackURL:      o.CallbackURL,
		CustomerOrderId:  o.CustomerOrderId,
		Expiry:           &o.Expiry,
		Direction:        o.Direction,
		Description:      o.Description,
		InternalTransfer: o.InternalTransfer,
		PaymentRequest:   &o.PaymentRequest,
		Preimage:         &o.Preimage,
		HashedPreimage:   &o.HashedPreimage,
		AmountMilliSat:   &o.AmountMilliSat,
		SettledAt:        o.SettledAt,
		Memo:             o.Memo,
		Status:           &o.Status,
		Error:            o.Error,
		CreatedAt:        o.CreatedAt,
		UpdatedAt:        o.UpdatedAt,
		DeletedAt:        o.DeletedAt,
	}
}

func (o Offchain) withAdditionalFields() offchainWithDerived {
	expiresAt := o.CreatedAt.Add(time.Second * time.Duration(o.Expiry))
	return offchainWithDerived{
		offchainNoJson:    offchainNoJson(o),
		HexPreimage:       hex.EncodeToString(o.Preimage),
		HexHashedPreimage: hex.EncodeToString(o.HashedPreimage),
		Type:              lightning,
		Expired:           expiresAt.Before(time.Now()),
		ExpiresAt:         expiresAt,
		AmountSat:         o.AmountMilliSat / 1000,
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

// OffchainStatus constants are used for mapping lnd's invoice/payment states
// to our internal states
const (
	// Offchain_CREATED is used to represent to Invoice_OPEN
	Offchain_CREATED OffchainStatus = "CREATED"
	// Offchain_SENT is used to represent Payment_IN_FLIGHT
	Offchain_SENT OffchainStatus = "SENT"
	// Offchain_COMPLETED is used to represent Invoice_SETTLED or Payment_SUCCEEDED
	Offchain_COMPLETED OffchainStatus = "COMPLETED"
	// Offchain_FLOPPED is used to represent Payment_FAILED
	Offchain_FLOPPED OffchainStatus = "FLOPPED"
)

var _ encoding.TextMarshaler = Offchain_COMPLETED

// InvoiceStateToTeslaState maps lnd's InvoiceState to our OffchainStatus
// InvoiceState are states for invoices belonging to our node, created
// using lncli.AddInvoice()
// Example usage: status := InvoiceStateToTeslaState[invoice.State]
var InvoiceStateToTeslaState = map[lnrpc.Invoice_InvoiceState]OffchainStatus{
	lnrpc.Invoice_OPEN:    Offchain_CREATED,
	lnrpc.Invoice_SETTLED: Offchain_COMPLETED,
}

// PaymentStateToTeslaState maps lnd's PaymentStatus to our OffchainStatus
// PaymentStatus are states of payments, e.g: outbound payments (lncli.SendPayment())
// Example usage: status := PaymentStateToTeslaState[payment.Status]
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

// MarkAsCompleted marks the given payment request as completed at the given date
func (o Offchain) MarkAsCompleted(database db.InsertGetter, preimage []byte,
	callbacker HttpPoster) (Offchain, error) {
	updateOffchainTxQuery := `UPDATE transactions
		SET preimage = :preimage, internal_transfer = :internal_transfer, settled_at = :settled_at, invoice_status = :invoice_status
		WHERE id = :id ` + txReturningSql

	log.WithFields(logrus.Fields{
		"paymentRequest": o.PaymentRequest,
		"id":             o.ID,
	}).Info("Marking invoice as completed")

	now := time.Now()
	o.SettledAt = &now
	o.Status = Offchain_COMPLETED
	o.Preimage = preimage

	tx := o.ToTransaction()
	rows, err := database.NamedQuery(updateOffchainTxQuery, &tx)
	if err != nil {
		log.WithError(err).Error("couldnt mark invoice as completed")
		return Offchain{}, err
	}
	// we defer CloseRows in case rows.Next() or StructScan fails
	defer db.CloseRows(rows)

	if !rows.Next() {
		return Offchain{}, fmt.Errorf("could not mark invoice as completed: %w", sql.ErrNoRows)
	}

	var updated Transaction
	if err = rows.StructScan(&updated); err != nil {
		return Offchain{}, err
	}

	updatedOffchain, err := updated.ToOffchain()
	if err != nil {
		return Offchain{}, err
	}

	// we close rows here to free up the connection because postCallback
	// needs to use the database
	db.CloseRows(rows)
	// call the callback URL(if exists)
	if updatedOffchain.CallbackURL != nil {
		if err = postCallback(database, updatedOffchain, callbacker); err != nil {
			// don't return here, we don't want this to fail the entire
			// operation
			log.WithError(err).Error("Could not POST to callback URL")
		}
	} else {
		log.WithField("id", updatedOffchain.ID).Debug("invoice did not have callback URL")
	}

	return updatedOffchain, nil
}

// MarkAsFlopped marks the transaction as failed
func (o Offchain) MarkAsFlopped(database db.Inserter, reason string) (Offchain, error) {
	updateOffchainTxQuery := `UPDATE transactions 
		SET invoice_status = :invoice_status, payment_error = :payment_error
		WHERE id = :id ` + txReturningSql

	log.WithFields(logrus.Fields{
		"paymentRequest": o.PaymentRequest,
		"id":             o.ID,
		"reason":         reason,
	}).Info("Marking invoice as failed")

	o.Status = Offchain_FLOPPED
	o.Error = &reason
	tx := o.ToTransaction()
	rows, err := database.NamedQuery(updateOffchainTxQuery, &tx)
	if err != nil {
		log.WithError(err).Errorf("Couldn't mark invoice as failed")
		return Offchain{}, err
	}
	defer db.CloseRows(rows)
	if !rows.Next() {
		return Offchain{}, fmt.Errorf("couldn't mark invoice as failed: %w", sql.ErrNoRows)
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
	if o.Error != nil {
		fragments = append(fragments, fmt.Sprintf("Error: %s", *o.Error))
	}

	fragments = append(fragments,
		fmt.Sprintf("Expiry: %d", o.Expiry),
		fmt.Sprintf("Direction: %s", o.Direction),
		fmt.Sprintf("AmountMilliSat: %d", o.AmountMilliSat),
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

	InternalTransfer bool    `json:"-"`
	Description      *string `json:"description,omitempty"`

	// Some onchain TXs may have an expiry time associated with them. Typically
	// this would be done where a merchant wants to give an offer to the consumer
	// without comitting to the price for too long.
	Expiry *int64 `json:"expiry,omitempty"`

	SettledAt *time.Time `json:"settledAt,omitempty"`

	ConfirmedAtBlock *int       `json:"confirmedAtBlock,omitempty"`
	ConfirmedAt      *time.Time `json:"confirmedAt,omitempty"`

	Address string `json:"address"`
	// The timestamp this TX got spent money to. We send it as createdAt when encoding
	// to JSON, to match the format of offchain TXs.
	ReceivedMoneyAt *time.Time `json:"createdAt,omitempty"`
	AmountSat       *int64     `json:"amountSat,omitempty"`
	Txid            *string    `json:"txid,omitempty"`
	Vout            *int       `json:"vout,omitempty"`

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
		e := expiresAt.Before(time.Now())
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
	var amountMilliSat *int64
	if o.AmountSat != nil {
		a := *o.AmountSat * 1000
		amountMilliSat = &a
	}
	return Transaction{
		ID:               o.ID,
		UserID:           o.UserID,
		CallbackURL:      o.CallbackURL,
		CustomerOrderId:  o.CustomerOrderId,
		AmountMilliSat:   amountMilliSat,
		SettledAt:        o.SettledAt,
		Expiry:           o.Expiry,
		Direction:        o.Direction,
		InternalTransfer: o.InternalTransfer,
		Description:      o.Description,

		ConfirmedAtBlock: o.ConfirmedAtBlock,
		ConfirmedAt:      o.ConfirmedAt,

		// Preimage:         nil, // otherwise we get empty slice
		// HashedPreimage:   nil, // otherwise we get empty slice

		Address:         &o.Address,
		Txid:            o.Txid,
		Vout:            o.Vout,
		ReceivedMoneyAt: o.ReceivedMoneyAt,

		CreatedAt: o.CreatedAt,
		UpdatedAt: o.UpdatedAt,
		DeletedAt: o.DeletedAt,
	}
}

// MarkAsConfirmed updates the transaction stored in the DB
// with Confirmed = true and ConfirmedAt = Now().
func (o Onchain) MarkAsConfirmed(database db.Inserter, height int) (Onchain, error) {

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

	rows, err := database.NamedQuery(query, &tx)
	if err != nil {
		return Onchain{}, err
	}
	defer db.CloseRows(rows)

	if !rows.Next() {
		return Onchain{}, fmt.Errorf("could not mark TX as confirmed: %w", sql.ErrNoRows)
	}

	var updatedTx Transaction
	if err = rows.StructScan(&updatedTx); err != nil {
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
func (o Onchain) PersistReceivedMoney(database db.Inserter, txid chainhash.Hash, vout int,
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
	receivedAt := time.Now()
	o.ReceivedMoneyAt = &receivedAt

	tx := o.ToTransaction()

	rows, err := database.NamedQuery(
		`UPDATE transactions SET txid = :txid, vout = :vout, amount_milli_sat = :amount_milli_sat,
			received_tx_at = :received_tx_at
			WHERE id = :id AND txid IS NULL AND vout IS NULL AND amount_milli_sat IS NULL `+txReturningSql,
		&tx)
	if err != nil {
		return Onchain{}, err
	}
	defer db.CloseRows(rows)

	if !rows.Next() {
		return Onchain{}, fmt.Errorf("could not update TX when adding received money: %w", sql.ErrNoRows)
	}

	var inserted Transaction
	if err = rows.StructScan(&inserted); err != nil {
		return Onchain{}, err
	}

	insertedOnchain, err := inserted.ToOnchain()
	if err != nil {
		return Onchain{}, err
	}

	return insertedOnchain, nil
}

// GetByID performs this query:
// `SELECT * FROM transactions WHERE id=id AND user_id=userID`,
// where id is the primary key of the table(autoincrementing)
func GetByID(database *db.DB, id int, userID int) (Transaction, error) {
	if id < 0 || userID < 0 {
		return Transaction{}, fmt.Errorf("GetByID(): neither id nor userID can be less than 0")
	}

	query := "SELECT * FROM transactions WHERE id=$1 AND user_id=$2 LIMIT 1"

	var transaction Transaction
	if err := database.Get(&transaction, query, id, userID); err != nil {
		log.WithError(err).WithField("id", id).Error("Could not get transaction")
		return transaction, fmt.Errorf("could not get transaction: %w", err)
	}

	return transaction, nil
}

// CountForUser returns the number of transactions in the DB related to the given user
func CountForUser(database db.Getter, userID int) (int, error) {
	var res int
	// count all onchain TXs that have been spent to + all LN TXs
	query := `SELECT COUNT(*) FROM transactions 
				WHERE user_id=$1 AND (received_tx_at IS NOT NULL OR payment_request IS NOT NULL)`
	err := database.Get(&res, query, userID)
	return res, err
}

type SortingDirection int

func (s *SortingDirection) UnmarshalText(text []byte) error {
	str := string(text)
	switch strings.ToLower(str) {
	case "asc":
		*s = SortAscending
	case "desc":
		*s = SortDescending
	default:
		return fmt.Errorf("unknown sorting direction: %s", str)
	}
	return nil
}

func (s SortingDirection) String() string {
	if s == SortAscending {
		return "ASC"
	} else {
		return "DESC"
	}
}

const (
	SortDescending SortingDirection = iota
	SortAscending
)

type GetAllParams struct {
	Offset       int
	Limit        int
	MaxMilliSats *int64
	MinMilliSats *int64 // Millisats
	End          *time.Time
	Start        *time.Time
	Sort         SortingDirection
	Direction    *Direction // If set, only include this direction in result
	Expired      *bool      // If set only include TXs with expiry status that match this argument
}

func (g GetAllParams) toFields() logrus.Fields {
	fields := logrus.Fields{
		"offset":       g.Offset,
		"limit":        g.Limit,
		"maxMilliSats": g.MaxMilliSats,
		"minMilliSats": g.MinMilliSats,
		"sort":         g.Sort.String(),
		"end":          g.End,
		"start":        g.Start,
	}

	if g.Direction != nil {
		fields["direction"] = *g.Direction
	}
	if g.Expired != nil {
		fields["expired"] = *g.Expired
	}

	return fields
}

// GetAll selects all the transactions for a user
func GetAll(database *db.DB, userID int, params GetAllParams) ([]Transaction, error) {
	if params.Limit == 0 {
		params.Limit = math.MaxInt64
	}

	argCounter := 1
	args := []interface{}{userID}

	whereQuery := fmt.Sprintf(`WHERE user_id=$%d AND (received_tx_at IS NOT NULL OR payment_request IS NOT NULL)`, argCounter)
	argCounter += 1

	if params.MaxMilliSats != nil {
		whereQuery = fmt.Sprintf(` %s AND amount_milli_sat < $%d`, whereQuery, argCounter)
		args = append(args, *params.MaxMilliSats)
		argCounter += 1
	}
	if params.MinMilliSats != nil {
		whereQuery = fmt.Sprintf(` %s AND amount_milli_sat > $%d`, whereQuery, argCounter)
		args = append(args, *params.MinMilliSats)
		argCounter += 1
	}

	if params.Direction != nil {
		whereQuery = fmt.Sprintf(` %s AND direction = $%d`, whereQuery, argCounter)
		args = append(args, *params.Direction)
		argCounter += 1
	}

	if params.Expired != nil {
		var operand = ">"
		if *params.Expired {
			operand = "<"
		}
		whereQuery = fmt.Sprintf(" %s AND ((expiry * INTERVAL '1 second') + created_at) %s now()", whereQuery, operand)
	}

	if params.End != nil {
		// when dealing with onchain TXs we count their received_tx_at as their creation date,
		// not when the address was registered in the DB
		whereQuery = fmt.Sprintf(` %s AND (CASE 
			WHEN received_tx_at IS NOT NULL THEN received_tx_at 
			ELSE created_at
		END) < $%d`, whereQuery, argCounter)
		args = append(args, *params.End)
		argCounter += 1
	}

	if params.Start != nil {
		// when dealing with onchain TXs we count their received_tx_at as their creation date,
		// not when the address was registered in the DB
		whereQuery = fmt.Sprintf(` %s AND (CASE 
			WHEN received_tx_at IS NOT NULL THEN received_tx_at 
			ELSE created_at
		END) > $%d`, whereQuery, argCounter)
		args = append(args, *params.Start)
		argCounter += 1
	}

	// Using OFFSET is not ideal, but until we start seeing
	// performance problems it's fine
	query := `SELECT *
		FROM transactions ` + whereQuery +
		// when dealing with onchain TXs we count their received_tx_at as their creation date,
		// not when the address was registered in the DB
		fmt.Sprintf(` 
		ORDER BY (CASE
			WHEN received_tx_at IS NOT NULL THEN received_tx_at
			ELSE created_at 
		END) %s
		LIMIT $%d
		OFFSET $%d`, params.Sort, argCounter, argCounter+1)
	args = append(args, params.Limit, params.Offset)

	log.WithFields(params.toFields()).WithFields(logrus.Fields{
		"userId": userID,
	}).Trace("Getting all TXs")

	// we need to initialize this variable because an empty SELECT will not update `transactions`
	transactions := []Transaction{}
	err := database.Select(&transactions, query, args...)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	return transactions, nil
}

func (o Onchain) String() string {
	fragments := []string{
		fmt.Sprintf("Onchain: {ID: %d", o.ID),
		fmt.Sprintf("UserID: %d", o.UserID),
		fmt.Sprintf("Expiry: %d", o.Expiry),
		fmt.Sprintf("Address: %s", o.Address),
		fmt.Sprintf("Direction: %s", o.Direction),
		fmt.Sprintf("InternalTransfer: %t", o.InternalTransfer),
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
	if o.ReceivedMoneyAt != nil {
		fragments = append(fragments, fmt.Sprintf("ReceivedMoneyAt: %s", *o.ReceivedMoneyAt))
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

const txReturningSql = ` RETURNING id, user_id, callback_url, customer_order_id, expiry, direction, amount_milli_sat, internal_transfer,
	    description, confirmed_at_block, confirmed_at, address, txid, vout, received_tx_at, payment_request, preimage, 
	    hashed_preimage, settled_at, memo, invoice_status, payment_error, created_at, updated_at, deleted_at`

func insertTransaction(database db.Inserter, t Transaction) (Transaction, error) {
	createTransactionQuery := `
	INSERT INTO transactions (user_id, callback_url, customer_order_id, expiry, direction, amount_milli_sat, internal_transfer, 
	                          description, confirmed_at_block, confirmed_at, address, txid, vout, received_tx_at, payment_request, 
	                          preimage, hashed_preimage, settled_at, memo, invoice_status, payment_error)
	VALUES (:user_id, :callback_url, :customer_order_id, :expiry, :direction, :amount_milli_sat, :internal_transfer, 
	        :description, :confirmed_at_block, :confirmed_at, :address, :txid, :vout, :received_tx_at, :payment_request, 
	        :preimage, :hashed_preimage, :settled_at, :memo, :invoice_status, :payment_error)` + txReturningSql

	rows, err := database.NamedQuery(createTransactionQuery, t)
	if err != nil {
		return Transaction{}, fmt.Errorf("could not insert transaction: %w", err)
	}
	defer db.CloseRows(rows)

	var transaction Transaction
	if rows.Next() {
		if err = rows.StructScan(&transaction); err != nil {
			log.WithError(err).Error("could not scan result into transaction struct")
			return Transaction{}, fmt.Errorf("could not insert transaction: %w", err)
		}
	}

	return transaction, nil
}
