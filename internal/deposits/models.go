package deposits

import (
	"github.com/jinzhu/gorm"
	"gitlab.com/arcanecrypto/lpp/internal/users"
)

// lightningInvoice is not a database table, just an internal type, hence it is not exported
type lightningInvoice struct {
}

// Deposit is a database table
type Deposit struct {
	gorm.Model
	UserID         users.User // userID of the user this withdrawal belongs to
	PaymentRequest string     `gorm:"not null"`
	Description    string
	Amount         int `gorm:"not null"`
	SettledAt      int // If this is not 0 or null, it means the invoice is settled
	InvoiceHash    string
	UUID           string
}
