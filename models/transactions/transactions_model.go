package transactions

import (
	"errors"
	"time"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	pkgErrors "github.com/pkg/errors"
	"gitlab.com/arcanecrypto/teslacoil/db"
)

// Transaction is the DB type for a transaction in Teslacoil. This
// includes both offchain and onchain TXs. This struct is only responsible
// for handling DB serialization and deserialization.
type Transaction struct {
	ID          int     `db:"id"`
	UserID      int     `db:"user_id"`
	CallbackURL *string `db:"callback_url"`
	// CustomerOrderId is an optional field where the user can specify an
	// order ID of their choosing. The only place this is used is when hitting
	// the callback URL of a transaction.
	CustomerOrderId *string `db:"customer_order_id"`

	// Encoded into invoice if offchain, internally only if onchain
	Expiry    int64     `db:"expiry"`
	Expired   bool      `db:"-"`
	ExpiresAt time.Time `db:"-"`

	Direction        Direction  `db:"direction"`
	AmountMSat       int64      `db:"amount_milli_sat"`
	Description      *string    `db:"description"`
	ConfirmedAtBlock *int       `db:"confirmed_at_block"`
	ConfirmedAt      *time.Time `db:"confirmed_at"`

	// onchain fields

	Address *string `db:"address"`
	Txid    *string `db:"txid"`
	Vout    *int    `db:"vout"`

	// onchain fields end

	// offchain fields

	PaymentRequest *string `db:"payment_request"`
	Preimage       *[]byte `db:"preimage"`
	HashedPreimage *[]byte `db:"hashed_preimage"`
	// If defined, it means the  invoice is settled
	SettledAt *time.Time `db:"settled_at"`
	Memo      *string    `db:"memo"`
	Status    *Status    `db:"invoice_status"`

	// offchain fields end

	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
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

// saveTxToTransaction saves a txid consisting of a txid, a vout and an amount to the
// db transaction
// If the transaction already has a txid, it returns an error
// TODO move this to onchain
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
// TODO move this to onchain
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

	/*	if _, err := users.IncreaseBalance(dbtx, users.ChangeBalance{
			UserID:    t.UserID,
			AmountSat: t.AmountSat,
		}); err != nil {
			_ = dbtx.Rollback()
			return pkgErrors.Wrapf(err, "could not credit user")
		}

		if err = dbtx.Commit(); err != nil {
			_ = dbtx.Rollback()
			return pkgErrors.Wrap(err, "could not commit changes")
		}*/

	return nil
}

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

	// If defined, it means the  invoice is settled
	SettledAt *time.Time `json:"settledAt"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"-"`
	DeletedAt *time.Time `json:"-"`
}

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

	Direction        Direction `json:"direction"`
	AmountSat        int64     `json:"amountSat"`
	Description      *string   `json:"description"`
	ConfirmedAtBlock *int      `json:"confirmedAtBlock"`
	Address          string    `json:"address"`
	Txid             *string   `json:"txid"`
	Vout             *int      `json:"vout"`

	// TODO doc
	SettledAt   *time.Time `json:"settledAt"`
	ConfirmedAt *time.Time `json:"confirmedAt"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"-"`
	DeletedAt   *time.Time `json:"-"`
}

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
