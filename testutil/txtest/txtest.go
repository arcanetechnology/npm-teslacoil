package txtest

import (
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gitlab.com/arcanecrypto/teslacoil/db"

	"github.com/brianvoe/gofakeit"
	"github.com/btcsuite/btcutil"
	"gitlab.com/arcanecrypto/teslacoil/ln"
	"gitlab.com/arcanecrypto/teslacoil/models/transactions"
)

func genPreimage() []byte {
	p := make([]byte, 32)
	_, _ = rand.Read(p)
	return p
}

func genTxid() string {
	p := make([]byte, 32)
	_, _ = rand.Read(p)
	return hex.EncodeToString(p)

}

func genStatus() transactions.Status {
	s := []transactions.Status{transactions.FAILED, transactions.OPEN, transactions.SUCCEEDED}
	return s[rand.Intn(3)]
}

func genMaybeString(fn func() string) *string {
	var res *string
	if gofakeit.Bool() {
		r := fn()
		res = &r
	}
	return res
}

func genDirection() transactions.Direction {
	direction := transactions.OUTBOUND
	if gofakeit.Bool() {
		direction = transactions.INBOUND
	}
	return direction
}

func int64Between(min, max int64) int64 {
	return rand.Int63n(max-min+1) + min
}

func GenOffchain(userID int) transactions.Offchain {
	amountMSat := rand.Int63n(ln.MaxAmountMsatPerInvoice)

	var preimage []byte
	var settledAt *time.Time
	var hashedPreimage []byte
	if gofakeit.Bool() {
		preimage = genPreimage()
		now := time.Now()
		start := now.Add(-(time.Hour * 24 * 60)) // 60 days ago
		s := gofakeit.DateRange(start, now)
		settledAt = &s
		h := sha256.Sum256(hashedPreimage)
		hashedPreimage = h[:]
	} else {
		hashedPreimage = genPreimage()
	}

	return transactions.Offchain{
		UserID:          userID,
		CallbackURL:     genMaybeString(gofakeit.URL),
		CustomerOrderId: genMaybeString(gofakeit.Word),
		Expiry:          int64(gofakeit.Number(100, 100000)),
		Direction:       genDirection(),
		Description: genMaybeString(func() string {
			return gofakeit.Sentence(gofakeit.Number(1, 10))
		}),
		PaymentRequest: "DO ME LATER",
		Preimage:       preimage,
		HashedPreimage: hashedPreimage,
		AmountMSat:     amountMSat,
		SettledAt:      settledAt,
		Memo: genMaybeString(func() string {
			return gofakeit.Sentence(gofakeit.Number(1, 10))
		}),

		Status: genStatus(),
	}
}

func GenOnchain(userID int) transactions.Onchain {
	now := time.Now()
	start := now.Add(-(time.Hour * 24 * 60)) // 60 days ago

	var txid *string
	var vout *int
	var amountSat *int64
	if gofakeit.Bool() {
		t := genTxid() // do me later
		txid = &t
		v := gofakeit.Number(0, 12)
		vout = &v
		a := int64Between(0, btcutil.MaxSatoshi)
		amountSat = &a
	}

	var confirmedAtBlock *int
	var confirmedAt *time.Time
	if txid != nil && gofakeit.Bool() {
		cA := gofakeit.DateRange(start, now)
		confirmedAt = &cA
		c := gofakeit.Number(1, 1000000)
		confirmedAtBlock = &c
	}

	var settledAt *time.Time
	if txid != nil && gofakeit.Bool() {
		s := gofakeit.DateRange(start, now)
		settledAt = &s
	}

	var expiry *int64
	if gofakeit.Bool() {
		e := int64(gofakeit.Number(100, 100000))
		expiry = &e
	}

	return transactions.Onchain{
		UserID:          userID,
		CallbackURL:     genMaybeString(gofakeit.URL),
		CustomerOrderId: genMaybeString(gofakeit.Word),
		SettledAt:       settledAt,
		Expiry:          expiry,
		Direction:       genDirection(),
		AmountSat:       amountSat,
		Description: genMaybeString(func() string {
			return gofakeit.Sentence(gofakeit.Number(1, 10))
		}),
		ConfirmedAtBlock: confirmedAtBlock,
		Address:          "DO ME LATER",
		Txid:             txid,
		Vout:             vout,
		ConfirmedAt:      confirmedAt,
	}
}

func InsertFakeIncomingOnchainorFail(t *testing.T, db *db.DB, userID int) transactions.Onchain {
	onchain := GenOnchain(userID)
	onchain.Direction = transactions.INBOUND
	tx, err := transactions.InsertOnchain(db, onchain)
	require.NoError(t, err, onchain)
	return tx
}

func InsertFakeOutgoingOnchainorFail(t *testing.T, db *db.DB, userID int) transactions.Onchain {
	onchain := GenOnchain(userID)
	onchain.Direction = transactions.OUTBOUND
	tx, err := transactions.InsertOnchain(db, onchain)
	require.NoError(t, err, onchain)
	return tx
}

func InsertFakeOnChainOrFail(t *testing.T, db *db.DB, userID int) transactions.Onchain {
	if gofakeit.Bool() {
		return InsertFakeIncomingOnchainorFail(t, db, userID)
	}
	return InsertFakeOutgoingOnchainorFail(t, db, userID)
}

func InsertFakeIncomingOffchainOrFail(t *testing.T, db *db.DB, userID int) transactions.Offchain {
	offchain := GenOffchain(userID)
	offchain.Direction = transactions.INBOUND
	tx, err := transactions.InsertOffchain(db, offchain)
	require.NoError(t, err)
	return tx
}

func InsertFakeOutgoingOffchainOrFail(t *testing.T, db *db.DB, userID int) transactions.Offchain {
	offchain := GenOffchain(userID)
	offchain.Direction = transactions.OUTBOUND
	tx, err := transactions.InsertOffchain(db, offchain)
	require.NoError(t, err)
	return tx
}

func InsertFakeOffChainOrFail(t *testing.T, db *db.DB, userID int) transactions.Offchain {
	if gofakeit.Bool() {
		return InsertFakeIncomingOffchainOrFail(t, db, userID)
	}
	return InsertFakeOutgoingOffchainOrFail(t, db, userID)
}

func InsertFakeIncomingOrFail(t *testing.T, db *db.DB, userID int) transactions.Transaction {
	if gofakeit.Bool() {
		return InsertFakeIncomingOffchainOrFail(t, db, userID).ToTransaction()
	}
	return InsertFakeIncomingOnchainorFail(t, db, userID).ToTransaction()
}

func fundedOnchain(minAmountSats, userId int) transactions.Onchain {
	on := GenOnchain(userId)

	sats := int64(gofakeit.Number(minAmountSats, btcutil.SatoshiPerBitcoin*10))
	on.AmountSat = &sats

	txid := genTxid()
	on.Txid = &txid

	vout := gofakeit.Number(0, 10)
	on.Vout = &vout

	block := gofakeit.Number(1, 600000)
	blockTime := gofakeit.Date()

	on.ConfirmedAtBlock = &block
	on.ConfirmedAt = &blockTime

	return on
}

func successfullOffchain(minAmountSats, userId int) transactions.Offchain {
	off := GenOffchain(userId)

	off.Status = transactions.SUCCEEDED

	s := gofakeit.Date()
	off.SettledAt = &s

	off.AmountMSat = int64(gofakeit.Number(minAmountSats*1000, ln.MaxAmountMsatPerInvoice))

	return off
}

func fundedIncoming(minAmountSats, userId int) transactions.Transaction {
	var tx transactions.Transaction
	if gofakeit.Bool() {
		tx = fundedOnchain(minAmountSats, userId).ToTransaction()
	} else {
		tx = successfullOffchain(minAmountSats, userId).ToTransaction()
	}
	tx.Direction = transactions.INBOUND
	return tx
}

func InsertFundedIncoming(t *testing.T, db db.Inserter, minAmountSats, userId int) transactions.Transaction {
	tx := fundedIncoming(minAmountSats, userId)
	if on, err := tx.ToOnchain(); err == nil {
		inserted, err := transactions.InsertOnchain(db, on)
		require.NoError(t, err)
		return inserted.ToTransaction()
	}

	if off, err := tx.ToOffchain(); err == nil {
		inserted, err := transactions.InsertOffchain(db, off)
		require.NoError(t, err)
		return inserted.ToTransaction()
	}

	require.FailNow(t, "Got neither onchain not offchain")
	panic("")
}
