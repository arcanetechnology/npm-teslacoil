package txtest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/lightningnetwork/lnd/lnwire"

	"github.com/btcsuite/btcd/btcec"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/zpay32"

	"github.com/stretchr/testify/require"
	"gitlab.com/arcanecrypto/teslacoil/db"

	"github.com/brianvoe/gofakeit"
	"github.com/btcsuite/btcutil"
	"gitlab.com/arcanecrypto/teslacoil/ln"
	"gitlab.com/arcanecrypto/teslacoil/models/transactions"
)

var (
	// these variables are used for generating a payment request
	testPrivKeyBytes, _ = hex.DecodeString("e126f68f7eafcc8b74f54d269fe206be715000f94dac067d1c04a8ca3b2db734")
	testPrivKey, _      = btcec.PrivKeyFromBytes(btcec.S256(), testPrivKeyBytes)
	messageSigner       = zpay32.MessageSigner{
		SignCompact: func(hash []byte) ([]byte, error) {
			sig, err := btcec.SignCompact(btcec.S256(),
				testPrivKey, hash, true)
			if err != nil {
				return nil, fmt.Errorf("can't sign the "+
					"message: %v", err)
			}
			return sig, nil
		},
	}
)

// MockPreimage will create a random preimage
func MockPreimage() []byte {
	p := make([]byte, 32)
	_, _ = rand.Read(p)
	return p
}

// MockHash mocks a hashed preimage
func MockHash(preimage []byte) []byte {
	h := sha256.Sum256(preimage)
	return h[:]
}

// MockTxid will create a random txid
func MockTxid() string {
	p := make([]byte, 32)
	_, _ = rand.Read(p)
	return hex.EncodeToString(p)
}

// MockOffchainStatus will create a random status
func MockOffchainStatus() transactions.OffchainStatus {
	s := []transactions.OffchainStatus{
		transactions.Offchain_CREATED,
		transactions.Offchain_SENT,
		transactions.Offchain_COMPLETED,
		transactions.Offchain_FLOPPED,
	}
	return s[rand.Intn(3)]
}

// MockMaybeString will sometimes return nil, and other times return a
// string using the argument function
func MockMaybeString(fn func() string) *string {
	var res *string
	if gofakeit.Bool() {
		r := fn()
		res = &r
	}
	return res
}

// MockDirection will create a random Direction
func MockDirection() transactions.Direction {
	if gofakeit.Int8()%2 == 0 {
		return transactions.INBOUND
	}

	return transactions.OUTBOUND
}

func int64Between(min, max int64) int64 {
	return rand.Int63n(max-min+1) + min
}

func positiveInt64() int64 {
	return rand.Int63n(math.MaxInt64)
}

// mockPaymentRequest mocks a payment request using lnd's zpay32 library
func mockPaymentRequest() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)

	description := gofakeit.HipsterSentence(4)
	amount := btcutil.Amount(rand.Int63n(ln.MaxAmountSatPerInvoice))
	amountMilli := lnwire.NewMSatFromSatoshis(amount)

	paymentHash := sha256.Sum256(b)
	invoice, err := zpay32.NewInvoice(&chaincfg.RegressionNetParams,
		paymentHash,
		time.Now(),
		zpay32.Amount(amountMilli),
		zpay32.Description(description),
	)
	if err != nil {
		panic(fmt.Errorf("could not create paymentrequest: %w", err))
	}

	paymentRequest, err := invoice.Encode(messageSigner)
	if err != nil {
		panic(fmt.Errorf("could not sign invoice: %w", err))
	}

	return paymentRequest
}

func GenOffchain(userID int) transactions.Offchain {
	amountMSat := rand.Int63n(ln.MaxAmountMsatPerInvoice)

	var preimage []byte
	var settledAt *time.Time
	var hashedPreimage []byte
	if gofakeit.Bool() {
		preimage = MockPreimage()
		h := sha256.Sum256(hashedPreimage)
		hashedPreimage = h[:]
	} else {
		hashedPreimage = MockPreimage()
	}

	status := MockOffchainStatus()
	if status == transactions.Offchain_COMPLETED {
		now := time.Now()
		start := now.Add(-(time.Hour * 24 * 60)) // 60 days ago
		s := gofakeit.DateRange(start, now)
		settledAt = &s
	}

	return transactions.Offchain{
		UserID:          userID,
		CallbackURL:     MockMaybeString(gofakeit.URL),
		CustomerOrderId: MockMaybeString(gofakeit.Word),
		Expiry:          positiveInt64(),
		Direction:       MockDirection(),
		Description: MockMaybeString(func() string {
			return gofakeit.Sentence(gofakeit.Number(1, 10))
		}),
		PaymentRequest: mockPaymentRequest(),
		Preimage:       preimage,
		HashedPreimage: hashedPreimage,
		AmountMSat:     amountMSat,
		SettledAt:      settledAt,
		Memo: MockMaybeString(func() string {
			return gofakeit.Sentence(gofakeit.Number(1, 10))
		}),

		Status: status,
	}
}

func GenOnchain(userID int) transactions.Onchain {
	now := time.Now()
	start := now.Add(-(time.Hour * 24 * 60)) // 60 days ago

	var txid *string
	var vout *int
	var amountSat *int64
	var receivedMoneyAt *time.Time
	if gofakeit.Bool() {
		t := MockTxid()
		txid = &t
		v := gofakeit.Number(0, 12)
		vout = &v
		a := int64Between(0, btcutil.SatoshiPerBitcoin)
		amountSat = &a
		r := gofakeit.Date()
		receivedMoneyAt = &r
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
		CallbackURL:     MockMaybeString(gofakeit.URL),
		CustomerOrderId: MockMaybeString(gofakeit.Word),
		Expiry:          expiry,
		Direction:       MockDirection(),
		AmountSat:       amountSat,
		ReceivedMoneyAt: receivedMoneyAt,
		Description: MockMaybeString(func() string {
			return gofakeit.Sentence(gofakeit.Number(1, 10))
		}),
		ConfirmedAtBlock: confirmedAtBlock,
		Address:          "DO ME LATER",
		Txid:             txid,
		Vout:             vout,
		ConfirmedAt:      confirmedAt,
		SettledAt:        settledAt,
	}
}

func InsertFakeOnchain(t *testing.T, db *db.DB, userID int) (transactions.Onchain, error) {
	onchain := GenOnchain(userID)
	return transactions.InsertOnchain(db, onchain)
}

func InsertFakeOffchain(t *testing.T, db *db.DB, userID int) (transactions.Offchain, error) {
	offchain := GenOffchain(userID)
	return transactions.InsertOffchain(db, offchain)
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
