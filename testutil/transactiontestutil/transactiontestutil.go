package transactiontestutil

import (
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
	"time"

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
		Expiry:          gofakeit.Int64(),
		Direction:       genDirection(),
		AmountSat:       amountMSat / 1000,
		Description: genMaybeString(func() string {
			return gofakeit.Sentence(gofakeit.Number(0, 10))
		}),
		PaymentRequest: "DO ME LATER",
		Preimage:       preimage,
		HashedPreimage: hashedPreimage,
		AmountMSat:     amountMSat,
		SettledAt:      settledAt,
		Memo: genMaybeString(func() string {
			return gofakeit.Sentence(gofakeit.Number(0, 10))
		}),

		Status: genStatus(),
	}
}

func GenOnchain(userID int) transactions.Onchain {
	now := time.Now()
	start := now.Add(-(time.Hour * 24 * 60)) // 60 days ago

	var settledAt *time.Time
	if gofakeit.Bool() {
		s := gofakeit.DateRange(start, now)
		settledAt = &s
	}

	var confirmedAtBlock *int
	var confirmedAt *time.Time
	if gofakeit.Bool() {
		cA := gofakeit.DateRange(start, now)
		confirmedAt = &cA
		c := gofakeit.Number(1, 1000000)
		confirmedAtBlock = &c
	}

	var txid *string
	var vout *int
	if gofakeit.Bool() {
		t := genTxid() // do me later
		txid = &t
		v := gofakeit.Number(0, 12)
		vout = &v
	}

	return transactions.Onchain{
		UserID:          userID,
		CallbackURL:     genMaybeString(gofakeit.URL),
		CustomerOrderId: genMaybeString(gofakeit.Word),
		Expiry:          gofakeit.Int64(),
		Direction:       genDirection(),
		AmountSat:       int64Between(0, btcutil.MaxSatoshi),
		Description: genMaybeString(func() string {
			return gofakeit.Sentence(gofakeit.Number(0, 10))
		}),
		ConfirmedAtBlock: confirmedAtBlock,
		Address:          "DO ME LATER",
		Txid:             txid,
		Vout:             vout,
		ConfirmedAt:      confirmedAt,
		SettledAt:        settledAt,
	}
}
