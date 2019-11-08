package balance

import (
	"errors"
	"fmt"
	"math"

	"gitlab.com/arcanecrypto/teslacoil/build"

	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/db"
)

var log = build.AddSubLogger("BLNC")

const (
	milliSatsPerSat     = 1000
	milliSatsPerBitcoin = 100000000000 // 100 000 million
)

var (
	ErrUserHasNegativeBalance = errors.New("user has negative balance")
)

// Balance is a type we use to easily convert between different denominations of BTC(sats, millisats, Bitcoins)
type Balance int

// NewBalanceFromSats creates a Balance type from satoshis by multiplying sats with 1000
func NewBalanceFromSats(sats int64) Balance {
	return Balance(sats * milliSatsPerSat)
}

// MilliSats converts a Balance type to an int
func (b Balance) MilliSats() int64 {
	return int64(b)
}

// Sats converts a Balance type to an int by dividing with 1000
func (b Balance) Sats() int64 {
	sats := math.Floor(float64(b) / float64(milliSatsPerSat))
	return int64(sats)
}

// Bitcoins converts a Balance type to a btc amount by dividing with milliSatsPerBitcoin
func (b Balance) Bitcoins() float64 {
	return float64(b / milliSatsPerBitcoin)
}

func (b Balance) String() string {
	return fmt.Sprintf("%d msats", b.MilliSats())
}

// ForUser calculates the balance for a user ID
// TODO parallellize this
func ForUser(db *db.DB, userID int) (Balance, error) {
	incoming, err := IncomingForUser(db, userID)
	if err != nil {
		return -1, err
	}

	outgoing, err := OutgoingForUser(db, userID)
	if err != nil {
		return -1, err
	}

	balance := incoming - outgoing
	bLogger := log.WithFields(logrus.Fields{
		"userId":   userID,
		"incoming": incoming,
		"outgoing": outgoing,
		"balance":  balance,
	})
	if balance < 0 {
		bLogger.Error("User has negative balance!")
		// TODO: create some monitoring service that shuts everything down if this happens
		return -1, ErrUserHasNegativeBalance
	}

	bLogger.Trace("Calculated user balance")
	return balance, nil
}

// IncomingForUser calculates the users incoming balance.
// A offchain payment is included if invoice_status == 'COMPLETED' and settled_at is defined
// A onchain transaction is included if confirmed_at is defined
func IncomingForUser(db *db.DB, userID int) (Balance, error) {
	var res balanceResult
	query := `SELECT COALESCE(SUM(amount_milli_sat), 0) AS balance FROM transactions 
		WHERE direction = 'INBOUND' AND user_id=$1 AND (
		    (settled_at IS NOT NULL AND invoice_status = 'COMPLETED') OR
		    (confirmed_at IS NOT NULL) -- only check for invoices that are confirmed paid
		)`
	if err := db.Get(&res, query, userID); err != nil {
		return 0, err
	}
	incoming := Balance(res.BalanceMilliSat)
	return incoming, nil
}

func OutgoingForUser(db *db.DB, userID int) (Balance, error) {
	var res balanceResult
	query := `SELECT COALESCE(SUM(amount_milli_sat), 0) AS balance FROM transactions 
		WHERE direction = 'OUTBOUND' AND user_id=$1 AND
		      (invoice_status != 'FLOPPED' OR txid IS NOT NULL) -- include CREATED, SENT and COMPLETED invoices in outgoing balance`
	if err := db.Get(&res, query, userID); err != nil {
		return 0, err
	}
	outgoing := Balance(res.BalanceMilliSat)
	return outgoing, nil
}

type balanceResult struct {
	BalanceMilliSat uint64 `db:"balance"`
}
