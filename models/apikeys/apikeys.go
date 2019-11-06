package apikeys

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	origerrors "errors"
	"fmt"
	"time"

	"github.com/brianvoe/gofakeit"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/db"
)

var log = build.Log

// Key is the database representation of our API keys
type Key struct {
	HashedKey   []byte     `db:"hashed_key" json:"hashedKey"`
	LastLetters string     `db:"last_letters" json:"lastLetters"`
	UserID      int        `db:"user_id" json:"userId"`
	Description string     `db:"description" json:"description,omitempty"`
	CreatedAt   time.Time  `db:"created_at" json:"createdAt"`
	UpdatedAt   time.Time  `db:"updated_at" json:"-"`
	DeletedAt   *time.Time `db:"deleted_at" json:"-"`

	Permissions

	// TODO add expiration
	// TODO add IP whitelisting
}

// Permissions is the types of permissions our API keys can have
type Permissions struct {
	// ReadWallet indicates if this API key can read information about
	// the users balance, transaction details and other wallet-related data.
	ReadWallet bool `db:"read_wallet" json:"readWallet"`

	// CreateInvoice indicates if this API key can create invoices. This
	// includes both offchain and onchain invoices.
	CreateInvoice bool `db:"create_invoice" json:"createInvoice"`

	// SendTransaction indicates if this API key can send transactions.
	// This includes both offchain and onchain invoices.
	// TODO: set up limits per API key
	SendTransaction bool `db:"send_transaction" json:"sendTransaction"`

	// EditAccount indicates if this API key can edit account information
	EditAccount bool `db:"edit_account" json:"editAccount"`
}

// AllPermissions is a set of permissions that grants the user every right
var AllPermissions = Permissions{
	ReadWallet:      true,
	CreateInvoice:   true,
	SendTransaction: true,
	EditAccount:     true,
}

// IsEmpty returns true if all fields are set to false
func (p Permissions) IsEmpty() bool {
	return !p.ReadWallet && !p.CreateInvoice && !p.SendTransaction && !p.EditAccount
}

// New creates a new API key for the given user. It returns both the inserted
// DB struct as well as the raw API key. It's not possible to retrieve the raw
// API key at a later point in time.
func New(d db.Inserter, userId int, permissions Permissions, description string) (key uuid.UUID, apiKey Key, err error) {
	if permissions.IsEmpty() {
		err = errors.New("API key cannot have zero permissions")
		return
	}

	key = uuid.NewV4()

	hasher := sha256.New()
	// according to godoc, this operation never fails
	_, _ = hasher.Write(key.Bytes())
	hashedKey := hasher.Sum(nil)

	keyStr := key.String()
	apiKey = Key{
		HashedKey:   hashedKey,
		LastLetters: keyStr[len(keyStr)-4:],
		UserID:      userId,
		Permissions: permissions,
		Description: description,
	}
	query := `
	INSERT INTO api_keys (hashed_key, user_id, last_letters, read_wallet, 
	                      send_transaction, edit_account, create_invoice,
	                      description)
	VALUES (:hashed_key, :user_id, :last_letters, :read_wallet, 
	        :send_transaction, :edit_account, :create_invoice,
	        :description) 
	RETURNING hashed_key, user_id, last_letters, read_wallet, send_transaction, 
	    edit_account, create_invoice, created_at, updated_at, deleted_at, description`
	rows, err := d.NamedQuery(query, apiKey)
	if err != nil {
		err = fmt.Errorf("could not insert API key: %w", err)
		return
	}

	if !rows.Next() {
		err = sql.ErrNoRows
		return
	}

	var inserted Key
	if err = rows.StructScan(&inserted); err != nil {
		err = fmt.Errorf("could not read API key from DB: %w", err)
		return
	}

	if err = rows.Close(); err != nil {
		return
	}

	return key, inserted, nil
}

// Delete deletes the API key associated with the given user ID and hash, if
// such a key exists
func Delete(d *db.DB, userId int, hash []byte) (Key, error) {
	del := time.Now()
	key := Key{
		UserID:    userId,
		DeletedAt: &del,
		HashedKey: hash,
	}
	query := `UPDATE api_keys 
		SET deleted_at = :deleted_at 
		WHERE user_id = :user_id AND hashed_key = :hashed_key
		RETURNING *`
	rows, err := d.NamedQuery(query, key)
	if err != nil {
		return Key{}, err
	}
	if !rows.Next() {
		return Key{}, fmt.Errorf("couldn't delete API key: %w", sql.ErrNoRows)
	}

	var res Key
	if err := rows.StructScan(&res); err != nil {
		return Key{}, err
	}

	return res, nil
}

// Get retrieves the API key in our DB that matches the given UUID, if such
// a key exists.
func Get(d *db.DB, key uuid.UUID) (Key, error) {

	hasher := sha256.New()
	// according to godoc, this operation never fails
	_, _ = hasher.Write(key.Bytes())
	hashedKey := hasher.Sum(nil)

	query := `SELECT *
	FROM api_keys
	WHERE hashed_key = $1 AND deleted_at IS NULL
	LIMIT 1`
	apiKey := Key{}
	if err := d.Get(&apiKey, query, hashedKey); err != nil {
		return Key{}, errors.Wrap(err, "API key not found")
	}
	return apiKey, nil
}

// GetByHash retrieves the API in our DB that matches the given hash, and user
// ID if such a key exists
func GetByHash(d db.Getter, userId int, hash []byte) (Key, error) {
	log.WithFields(
		logrus.Fields{
			"hash":   hex.EncodeToString(hash),
			"userId": userId,
		}).Info("Getting API key by hash")

	query := `SELECT * FROM api_keys 
		WHERE hashed_key = $1 AND user_id = $2 AND deleted_at IS NULL 
		LIMIT 1`
	var key Key
	err := d.Get(&key, query, hash, userId)

	return key, err
}

// GetByUserId gets all API keys associated with the given user ID
func GetByUserId(d db.Selecter, userId int) ([]Key, error) {
	query := `SELECT * FROM api_keys WHERE user_id = $1 AND deleted_at IS NULL`

	// we want to explicitly return empty list and not null, for JSON serialization purposes
	// noinspection ALL
	keys := []Key{}
	if err := d.Select(&keys, query, userId); err != nil {
		if origerrors.Is(err, sql.ErrNoRows) {
			return []Key{}, nil
		}
		return nil, err
	}
	return keys, nil
}

// RandomPermissionSet generates a set of random permissions where at least one
// of the fields are set to true
func RandomPermissionSet() Permissions {
	perm := Permissions{
		ReadWallet:      gofakeit.Bool(),
		CreateInvoice:   gofakeit.Bool(),
		SendTransaction: gofakeit.Bool(),
		EditAccount:     gofakeit.Bool(),
	}
	if perm.IsEmpty() {
		return RandomPermissionSet()
	}
	return perm
}
