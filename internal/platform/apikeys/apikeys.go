package apikeys

import (
	"database/sql"
	"time"

	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"
	"gitlab.com/arcanecrypto/teslacoil/internal/users"
)

// Key is the database representation of our API keys
type Key struct {
	// TODO don't store the key directly, but a hashed version
	Key       uuid.UUID  `db:"api_key"`
	UserID    int        `db:"user_id"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`

	// TODO add scopes
	// TODO add expiration
	// TODO add IP whitelisting
}

func New(d *db.DB, user users.User) (Key, error) {
	key := uuid.NewV4()
	apiKey := Key{
		Key:    key,
		UserID: user.ID,
	}
	query := `INSERT INTO api_keys 
	VALUES (:api_key, :user_id) 
	RETURNING api_key, user_id, created_at, updated_at, deleted_at `
	rows, err := d.NamedQuery(query, apiKey)
	if err != nil {
		return Key{}, errors.Wrap(err, "could not insert API key")
	}
	inserted := Key{}
	if rows.Next() {
		if err := rows.Scan(
			&inserted.Key,
			&inserted.UserID,
			&inserted.CreatedAt,
			&inserted.UpdatedAt,
			&inserted.DeletedAt,
		); err != nil {
			return Key{}, errors.Wrap(err, "could not scan API key")
		}
	} else {
		return Key{}, errors.Wrap(sql.ErrNoRows, "could not scan API key")
	}

	if err := rows.Close(); err != nil {
		return Key{}, err
	}
	return inserted, nil
}

func Get(d *db.DB, key uuid.UUID) (Key, error) {
	query := `SELECT api_key, user_id, created_at, updated_at, deleted_at 
	FROM api_keys
	WHERE api_key = $1
	LIMIT 1`
	apiKey := Key{}
	if err := d.Get(&apiKey, query, key.String()); err != nil {
		return Key{}, errors.Wrap(err, "API key not found")
	}
	return apiKey, nil
}
