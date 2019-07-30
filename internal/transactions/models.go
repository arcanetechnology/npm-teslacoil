package transactions

import (
	"time"
)

type Direction string

const (
	inbound  Direction = "inbound"  //nolint
	outbound Direction = "outbound" //nolint
)

// NewTransaction contains all information required to create a transaction
type NewTransaction struct {
	UserID      uint64 `json:"user_id"` // userID of the user this withdrawal belongs to
	Invoice     string
	Status      string
	Description string
	Direction   Direction
	Amount      int64
}

// Transaction is a database table
type Transaction struct {
	ID             uint64     `db:"id"`
	UserID         uint64     `db:"user_id"`
	Invoice        string     `db:"invoice"`
	PreImage       *string    `db:"pre_image"`
	HashedPreImage string     `db:"hashed_pre_image"`
	CallbackURL    *string    `db:"callback_url"`
	Status         string     `db:"status"`
	Description    string     `db:"description"`
	Direction      Direction  `db:"direction"`
	Amount         int64      `db:"amount"`
	SettledAt      *time.Time `db:"settled_at"` // If this is not 0 or null, it means the invoice is settled
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
	DeletedAt      *time.Time `db:"deleted_at"`
}

// TransactionResponse contains all field that are supposed to be returned
type TransactionResponse struct {
	ID             uint64     `db:"id"`
	UserID         uint64     `db:"user_id"`
	Invoice        string     `db:"invoice"`
	PreImage       string     `db:"pre_image"`
	HashedPreImage string     `db:"hashed_pre_image"`
	CallbackURL    *string    `db:"callback_url"`
	Status         string     `db:"status"`
	Description    string     `db:"description"`
	Direction      Direction  `db:"direction"`
	Amount         int64      `db:"amount"`
	SettledAt      *time.Time `db:"settled_at"` // If this is not 0 or null, it means the invoice is settled
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
	DeletedAt      *time.Time `db:"deleted_at" json:"-"`
}

type PaymentResponse struct {
	TransactionResponse
	User struct {
		ID        uint64     `db:"u.id"`
		Balance   int64      `db:"u.balance"`
		UpdatedAt *time.Time `db:"u.updated_at"`
	}
}
