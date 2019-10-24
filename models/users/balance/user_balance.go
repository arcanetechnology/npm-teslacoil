package balance

import (
	"gitlab.com/arcanecrypto/teslacoil/db"
)

type AmountUnit int

const (
	milliSatsPerSat     = 1000
	satsPerBitcoin      = 100000000    // 100 milllion
	milliSatsPerBitcoin = 100000000000 // 100 000 million
)

// Balance is a type we use to easily convert between different denominations of BTC(sats, millisats, Bitcoins)
type Balance int

// NewBalanceFromSats creates a Balance type from satoshis by multiplying sats with 1000
func NewBalanceFromSats(sats int) Balance {
	return Balance(sats * milliSatsPerSat)
}

// NewBalance creates a new balance type by casting `millisats` into a Balance type
func NewBalance(millisats int) Balance {
	return Balance(millisats)
}

// MilliSats converts a Balance type to an int
func (b Balance) MilliSats() int {
	return int(b)
}

// Sats converts a Balance type to an int by dividing with 1000
func (b Balance) Sats() int {
	return int(b / milliSatsPerSat)
}

// Bitcoins converts a Balance type to a btc amount by dividing with milliSatsPerBitcoin
func (b Balance) Bitcoins() float64 {
	return float64(b / milliSatsPerBitcoin)
}

// ForUser calculates the balance for a user ID
func ForUser(db *db.DB, userID int) (Balance, error) {
	type balanceResult struct {
		BalanceMilliSat int `db:"balance"`
	}
	var result balanceResult

	query := "SELECT COALESCE(sum(amount_milli_sat), 0) as balance from transactions WHERE (settled_at IS NOT NULL OR confirmed_at IS NOT NULL) AND user_id=$1;"

	if err := db.Get(&result, query, userID); err != nil {
		return -1, err
	}

	balance := NewBalance(result.BalanceMilliSat)

	return balance, nil
}
