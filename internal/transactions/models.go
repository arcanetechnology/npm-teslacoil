package transactions

import (
	"time"

	"github.com/jinzhu/gorm"
	"gitlab.com/arcanecrypto/lpp/internal/users"
)

type direction string

const (
	inbound  direction = "inbound"
	outbound direction = "outbound"
)

// NewTransaction contains all information required to create a transaction
type NewTransaction struct {
	UserID      uint `json:"user_id"` // userID of the user this withdrawal belongs to
	Invoice     string
	Status      string
	Description string
	Direction   direction
	Amount      int
}

// Transaction is a database table
type Transaction struct {
	gorm.Model
	UserID      uint
	User        users.User `json:"-"`
	Invoice     string
	Status      string
	Description string
	Direction   direction `sql:"type:direction"`
	Amount      int       `gorm:"not null"`
	SettledAt   time.Time // If this is not 0 or null, it means the invoice is settled
}
