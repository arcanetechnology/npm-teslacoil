package withdrawals

import (
	"github.com/jinzhu/gorm"
	"gitlab.com/arcanecrypto/lpp/internal/users"
)

// Withdrawal is a database table
type Withdrawal struct {
	gorm.Model
	UserID         users.User // userID of the user this withdrawal belongs to
	PaymentRequest string     `gorm:"not null"`
	Description    string
	Amount         int `gorm:"not null"`
	Fee            int
	SettledAt      int // If this is not 0, it means the invoice is settled
	InvoiceHash    string
	UUID           string
}
