package apikeys

import (
	"crypto/sha256"
	"database/sql"
	origerrors "errors"
	"time"

	"github.com/pkg/errors"

	uuid "github.com/satori/go.uuid"
	"gitlab.com/arcanecrypto/teslacoil/db"
)

// Key is the database representation of our API keys
type Key struct {
	HashedKey []byte     `db:"hashed_key"`
	UserID    int        `db:"user_id"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`

	// TODO add scopes
	// TODO add expiration
	// TODO add IP whitelisting
}

// New creates a new API key for the given user. It returns both the inserted
// DB struct as well as the raw API key. It's not possible to retrieve the raw
// API key at a later point in time.
func New(d *db.DB, userId int) (uuid.UUID, Key, error) {
	key := uuid.NewV4()

	hasher := sha256.New()
	// according to godoc, this operation never fails
	_, _ = hasher.Write(key.Bytes())
	hashedKey := hasher.Sum(nil)

	apiKey := Key{
		HashedKey: hashedKey,
		UserID:    userId,
	}
	query := `INSERT INTO api_keys 
	VALUES (:hashed_key, :user_id) 
	RETURNING hashed_key, user_id, created_at, updated_at, deleted_at `
	rows, err := d.NamedQuery(query, apiKey)
	if err != nil {
		return uuid.UUID{}, Key{}, errors.Wrap(err, "could not insert API key")
	}
	inserted := Key{}
	if rows.Next() {
		if err := rows.Scan(
			&inserted.HashedKey,
			&inserted.UserID,
			&inserted.CreatedAt,
			&inserted.UpdatedAt,
			&inserted.DeletedAt,
		); err != nil {
			return uuid.UUID{}, Key{}, errors.Wrap(err, "could not scan API key")
		}
	} else {
		return uuid.UUID{}, Key{}, errors.Wrap(sql.ErrNoRows, "could not scan API key")
	}

	if err := rows.Close(); err != nil {
		return uuid.UUID{}, Key{}, err
	}
	return key, inserted, nil
}

func Get(d *db.DB, key uuid.UUID) (Key, error) {

	hasher := sha256.New()
	// according to godoc, this operation never fails
	_, _ = hasher.Write(key.Bytes())
	hashedKey := hasher.Sum(nil)

	query := `SELECT hashed_key, user_id, created_at, updated_at
	FROM api_keys
	WHERE hashed_key = $1 AND deleted_at IS NULL
	LIMIT 1`
	apiKey := Key{}
	if err := d.Get(&apiKey, query, hashedKey); err != nil {
		return Key{}, errors.Wrap(err, "API key not found")
	}
	return apiKey, nil
}

// GetByUserId gets all API keys associated with the given user ID
func GetByUserId(d *db.DB, userId int) ([]Key, error) {
	query := `SELECT * FROM api_keys WHERE user_id = $1`
	var keys []Key
	if err := d.Select(&keys, query, userId); err != nil {
		if origerrors.Is(err, sql.ErrNoRows) {
			return []Key{}, nil
		}
		return nil, err
	}
	return keys, nil
}
