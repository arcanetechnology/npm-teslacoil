package transactions

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/arcanecrypto/teslacoil/async"

	"github.com/brianvoe/gofakeit"
	"gitlab.com/arcanecrypto/teslacoil/models/users"

	"github.com/lightningnetwork/lnd/lnrpc"
	pkgErrors "github.com/pkg/errors"
	"gitlab.com/arcanecrypto/teslacoil/ln"
	"gitlab.com/arcanecrypto/teslacoil/models/apikeys"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/lntestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/userstestutil"

	"gitlab.com/arcanecrypto/teslacoil/db"
)

var (
	SamplePreimage = func() []byte {
		encoded, _ := hex.DecodeString(SamplePreimageHex)
		return encoded
	}()
	SamplePreimageHex = "0123456789abcdef0123456789abcdef"
	SampleHash        = func() [32]byte {
		first := sha256.Sum256(SamplePreimage)
		return sha256.Sum256(first[:])
	}()

	SampleHashHex = hex.EncodeToString(SampleHash[:])
	firstMemo     = "HiMisterHey"
	description   = "My personal description"
	secondMemo    = "HelloWorld"
)

func TestNewOffchainTx(t *testing.T) {
	t.Parallel()
	user := userstestutil.CreateUserOrFail(t, testDB)

	amount1 := rand.Int63n(ln.MaxAmountSatPerInvoice)
	amount2 := rand.Int63n(ln.MaxAmountSatPerInvoice)
	amount3 := rand.Int63n(ln.MaxAmountSatPerInvoice)

	customerOrderId := "this is an order id"

	tests := []struct {
		memo        string
		description string
		amountSat   int64
		orderId     string

		lndInvoice lnrpc.Invoice
		want       Offchain
	}{
		{
			memo:        firstMemo,
			description: description,
			amountSat:   amount1,

			lndInvoice: lnrpc.Invoice{
				Value:          amount1,
				PaymentRequest: "SomePayRequest",
				RHash:          SampleHash[:],
				RPreimage:      SamplePreimage,
				Settled:        false,
			},
			want: Offchain{
				UserID:         user.ID,
				AmountSat:      amount1,
				AmountMSat:     amount1 * 1000,
				HashedPreimage: SampleHash[:],
				Memo:           &firstMemo,
				Description:    &description,
				Status:         Status("OPEN"),
				Direction:      Direction("INBOUND"),
			},
		},
		{
			memo:        firstMemo,
			description: description,
			amountSat:   amount2,

			lndInvoice: lnrpc.Invoice{
				Value:          amount2,
				PaymentRequest: "SomePayRequest",
				RHash:          SampleHash[:],
				RPreimage:      SamplePreimage,
				Settled:        false,
			},
			want: Offchain{
				UserID:         user.ID,
				AmountSat:      amount2,
				AmountMSat:     amount2 * 1000,
				HashedPreimage: SampleHash[:],
				Memo:           &firstMemo,
				Description:    &description,
				Status:         Status("OPEN"),
				Direction:      Direction("INBOUND"),
			},
		},
		{
			memo:        firstMemo,
			description: description,
			amountSat:   amount3,
			orderId:     customerOrderId,

			lndInvoice: lnrpc.Invoice{
				Value:          amount3,
				PaymentRequest: "SomePayRequest",
				RHash:          SampleHash[:],
				RPreimage:      SamplePreimage,
				Settled:        false,
			},
			want: Offchain{
				UserID:          user.ID,
				AmountSat:       amount3,
				AmountMSat:      amount3 * 1000,
				Memo:            &firstMemo,
				HashedPreimage:  SampleHash[:],
				Description:     &description,
				Status:          Status("OPEN"),
				Direction:       Direction("INBOUND"),
				CustomerOrderId: &customerOrderId,
			},
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("create invoice with amount %d memo %s and description %s",
			tt.amountSat, tt.memo, tt.description), func(t *testing.T) {

			// Create Mock LND client with preconfigured invoice response
			mockLNcli := lntestutil.LightningMockClient{
				InvoiceResponse: tt.lndInvoice,
			}

			offchainTx, err := NewOffchain(testDB, mockLNcli, NewOffchainOpts{
				UserID:      tt.want.UserID,
				AmountSat:   tt.amountSat,
				Memo:        tt.memo,
				Description: tt.description,
				OrderId:     tt.orderId,
			})
			if err != nil {
				testutil.FatalMsgf(t, "should be able to CreateInvoice %+v", err)
			}

			// Assertions
			got := offchainTx
			want := tt.want

			assertOffchainTxsAreEqual(t, got, want)
		})
	}
}

func TestPayInvoice(t *testing.T) {
	t.Parallel()
	// Setup the database
	user := CreateUserWithBalanceOrFail(t, testDB, ln.MaxAmountMsatPerInvoice*5)

	amount := int64(gofakeit.Number(1, ln.MaxAmountMsatPerInvoice))
	// Create Mock LND client with preconfigured invoice response
	mockLNcli := lntestutil.LightningMockClient{
		InvoiceResponse: lnrpc.Invoice{},
		SendPaymentSyncResponse: lnrpc.SendResponse{
			PaymentPreimage: SamplePreimage,
			PaymentHash:     SampleHash[:],
		},
		// define what lncli.DecodePayReq returns
		DecodePayReqResponse: lnrpc.PayReq{
			PaymentHash: SampleHashHex,
			NumSatoshis: amount,
		},
	}
	paymentRequest := "SomeOffchainRequest1"
	expectedOffchain := Offchain{
		UserID:         user.ID,
		AmountSat:      amount,
		AmountMSat:     amount * 1000,
		Preimage:       SamplePreimage,
		HashedPreimage: SampleHash[:],
		Direction:      OUTBOUND,
		Status:         SUCCEEDED,
	}

	t.Run("paying invoice decreases balance of user", func(t *testing.T) {

		_, err := PayInvoice(
			testDB, &mockLNcli, user.ID, paymentRequest)
		if err != nil {
			testutil.FatalMsgf(t, "could not pay invoice: %v", err)
		}

		_, err = users.GetByID(testDB, user.ID)
		if err != nil {
			testutil.FatalMsg(t, err)
		}

		// testutil.AssertEqual(t, updatedUser.Balance, user.Balance-amount)
	})

	t.Run("paying invoice greater than balance fails with 'violates check constraint user_balance_check'", func(t *testing.T) {
		mockLNcli.DecodePayReqResponse = lnrpc.PayReq{
			PaymentHash: SampleHashHex,
			NumSatoshis: ln.MaxAmountMsatPerInvoice * 6,
		}

		_, err := PayInvoice(
			testDB, &mockLNcli, user.ID, paymentRequest)
		if !errors.Is(err, ErrUserBalanceTooLow) {
			testutil.FailMsgf(t, "should not pay invoice greater than users balance")
		}

	})
	t.Run("paying invoice with 0 amount fails with Err0AmountInvoiceNotSupported", func(t *testing.T) {
		mockLNcli.DecodePayReqResponse = lnrpc.PayReq{
			PaymentHash: SampleHashHex,
			NumSatoshis: 0,
		}

		_, err := PayInvoice(
			testDB, &mockLNcli, user.ID, paymentRequest)
		if !errors.Is(err, Err0AmountInvoiceNotSupported) {
			testutil.FailMsgf(t, "expected Err0AmountInvoiceNotSupported but got error %v", err)
		}

	})
	t.Run("successfully paying invoice marks invoice as paid", func(t *testing.T) {
		mockLNcli.DecodePayReqResponse = lnrpc.PayReq{
			PaymentHash: SampleHashHex,
			NumSatoshis: amount,
		}

		got, err := PayInvoice(
			testDB, &mockLNcli, user.ID, paymentRequest)
		if err != nil {
			testutil.FatalMsgf(t, "could not pay invoice: %v", err)
		}

		expectedOffchain.SettledAt = got.SettledAt
		expectedOffchain.Status = SUCCEEDED
		expectedOffchain.Preimage = got.Preimage

		assertOffchainTxsAreEqual(t, got, expectedOffchain)
	})
	t.Run("successfully paying invoice marks invoice settledAt date", func(t *testing.T) {
		paymentRequest := "SomeOffchainRequest1"

		got, err := PayInvoice(
			testDB, &mockLNcli, user.ID, paymentRequest)
		if err != nil {
			testutil.FatalMsgf(t, "could not pay invoice: %v", err)
		}

		expectedOffchain.SettledAt = got.SettledAt
		expectedOffchain.Status = SUCCEEDED

		assertOffchainTxsAreEqual(t, got, expectedOffchain)

		updatedInvoice, err := GetTransactionByID(testDB, got.ID, user.ID)
		if err != nil {
			testutil.FatalMsg(t, err)
		}

		if updatedInvoice.SettledAt == nil {
			testutil.FailMsgf(t, "expected settledAt to be defined, but was <nil>")
		}
	})
	t.Run("successfully paying invoice marks invoice settledAt date", func(t *testing.T) {
		paymentRequest := "SomeOffchainRequest1"

		got, err := PayInvoice(
			testDB, &mockLNcli, user.ID, paymentRequest)
		if err != nil {
			testutil.FatalMsgf(t, "could not pay invoice: %v", err)
		}

		expectedOffchain.SettledAt = got.SettledAt
		expectedOffchain.Status = SUCCEEDED

		assertOffchainTxsAreEqual(t, got, expectedOffchain)

		updatedInvoice, err := GetTransactionByID(testDB, got.ID, user.ID)
		if err != nil {
			testutil.FatalMsgf(t, "could not getbyid: %v", err)
		}

		if updatedInvoice.SettledAt == nil {
			testutil.FailMsgf(t, "expected settledAt to be defined, but was <nil>")
		}
	})
}

// TODO: Add cases where the triggerInvoice .settled is false
// This case should return the exact same offchainTx and an empty User
func TestUpdateInvoiceStatus(t *testing.T) {
	t.Parallel()
	// Arrange
	u := userstestutil.CreateUserOrFail(t, testDB)

	t.Run("callback URL should be called", func(t *testing.T) {
		t.Parallel()
		testutil.DescribeTest(t)

		// for the callback to be executed, we need to create an API key for the
		// current user. this is because the callback body is hashed with the
		// users API key
		if _, _, err := apikeys.New(testDB, u); err != nil {
			testutil.FatalMsg(t, pkgErrors.Wrap(err, "Could not make API key"))
		}

		lnMock := lntestutil.GetRandomLightningMockClient()
		httpPoster := testutil.GetMockHttpPoster()
		mockInvoice, _ := ln.AddInvoice(lnMock, lnrpc.Invoice{})
		offchainTx := CreateNewOffchainTxOrFail(t, testDB, lnMock, NewOffchainOpts{
			UserID:      u.ID,
			AmountSat:   mockInvoice.Value,
			CallbackURL: "https://example.com",
		})

		testutil.AssertMsg(t, offchainTx.CallbackURL != nil,
			"Callback URL was nil! Offchain: "+offchainTx.String())
		invoice := lnrpc.Invoice{
			PaymentRequest: offchainTx.PaymentRequest,
			Settled:        true,
		}

		_, err := UpdateInvoiceStatus(invoice, testDB, httpPoster)
		if err != nil {
			testutil.FatalMsgf(t,
				"should be able to UpdateInvoiceStatus. Error:  %+v\n",
				err)
		}
		checkPostSent := func() bool {
			return httpPoster.GetSentPostRequests() == 1
		}

		// emails are sent in go-routine, so can't assume they're sent fast
		// enough for test to pick up
		if err := async.Await(8,
			time.Millisecond*20, checkPostSent); err != nil {
			testutil.FatalMsg(t, err)
		}
	})

	t.Run("callback URL should not be called with non-settled invoice", func(t *testing.T) {
		t.Parallel()
		lnMock := lntestutil.GetLightningMockClient()
		httpPoster := testutil.GetMockHttpPoster()
		mockInvoice, _ := ln.AddInvoice(lnMock, lnrpc.Invoice{})
		offchainTx := CreateNewOffchainTxOrFail(t, testDB, lnMock, NewOffchainOpts{
			UserID:      u.ID,
			AmountSat:   mockInvoice.Value,
			CallbackURL: "https://example.com",
		})

		invoice := lnrpc.Invoice{
			PaymentRequest: offchainTx.PaymentRequest,
			Settled:        false,
		}

		_, err := UpdateInvoiceStatus(invoice, testDB, httpPoster)
		if err != nil {
			testutil.FatalMsgf(t,
				"should be able to UpdateInvoiceStatus. Error:  %+v\n",
				err)
		}

		checkPostSent := func() bool {
			return httpPoster.GetSentPostRequests() > 0
		}

		// emails are sent in go-routing, so can't assume they're sent fast
		// enough for test to pick up
		if err := async.Await(4,
			time.Millisecond*20, checkPostSent); err == nil {
			testutil.FatalMsgf(t, "HTTP POSTer sent out callback for non-settled offchainTx")
		}
	})
}

func TestOffchain_WithAdditionalFields(t *testing.T) {
	t.Parallel()
	user := userstestutil.CreateUserOrFail(t, testDB)

	t.Run("expiry field", func(t *testing.T) {
		offchainTx := Offchain{
			UserID:         user.ID,
			HashedPreimage: []byte("f747dbf93249644a71749b6fff7c5a9eb7c1526c52ad3414717e222470940c57"),
			Expiry:         1,
			Direction:      Direction("INBOUND"),
			Status:         Status("OPEN"),
			AmountSat:      100,
			AmountMSat:     100000,
		}

		offchainTx, err := InsertOffchain(testDB, offchainTx)
		require.NoError(t, err)

		// Sleep for expiry to check if expired property is set
		// correctly. expired should be true
		time.Sleep(time.Second * time.Duration(offchainTx.Expiry))

		offchainTx = offchainTx.WithAdditionalFields()

		assert.True(t, offchainTx.Expired)
		assert.Equal(t, offchainTx.ExpiresAt, offchainTx.CreatedAt.Add(time.Second*time.Duration(offchainTx.Expiry)))
	})

	invoices := []Offchain{
		Offchain{
			UserID:         user.ID,
			HashedPreimage: []byte("f747dbf93249644a71749b6fff7c5a9eb7c1526c52ad3414717e222470940c57"),
			Expiry:         3600,
			Direction:      Direction("INBOUND"),
			Status:         Status("OPEN"),
			AmountSat:      100,
			AmountMSat:     100000,
		},
		Offchain{
			UserID:         user.ID,
			HashedPreimage: []byte("f747dbf93249644a71749b6fff7c5a9eb7c1526c52ad3414717e222470940c57"),
			Expiry:         2,
			Direction:      Direction("INBOUND"),
			Status:         Status("OPEN"),
			AmountSat:      100,
			AmountMSat:     100000,
		},
	}

	for _, invoice := range invoices {
		t.Run(fmt.Sprintf("payment with expiry %d should not be expired", invoice.Expiry),
			func(t *testing.T) {
				payment, err := InsertOffchain(testDB, invoice)
				require.NoError(t, err)
				assert.False(t, payment.Expired)
			})
	}
}

func assertOffchainTxsAreEqual(t *testing.T, got, want Offchain) {
	t.Helper()
	testutil.AssertEqual(t, got.UserID, want.UserID, "userID")
	testutil.AssertEqual(t, got.AmountSat, want.AmountSat, "amountSat")
	testutil.AssertEqual(t, got.AmountMSat, want.AmountMSat, "amountMSat")

	testutil.AssertMsg(t, (got.Preimage == nil) == (want.Preimage == nil), "Preimage was nil and not nil")
	if got.Preimage != nil {
		testutil.AssertEqual(t, got.Preimage, want.Preimage)
	}
	testutil.AssertEqual(t, got.HashedPreimage, want.HashedPreimage, "hashedPreimage")

	testutil.AssertMsg(t, (got.Memo == nil) == (want.Memo == nil), "Memo was nil and not nil")
	if got.Memo != nil {
		testutil.AssertEqual(t, *got.Memo, *want.Memo)
	}

	testutil.AssertMsg(t, (got.Description == nil) == (want.Description == nil), "Description was nil and not nil")
	if got.Description != nil {
		testutil.AssertEqual(t, *got.Description, *want.Description)
	}

	testutil.AssertEqual(t, got.Status, want.Status, "status")
	testutil.AssertEqual(t, got.Direction, want.Direction, "direction")

	testutil.AssertMsg(t, (got.CallbackURL == nil) == (want.CallbackURL == nil), "CallbackURL was nil and not nil")
	if got.CallbackURL != nil {
		testutil.AssertEqual(t, *got.CallbackURL, *want.CallbackURL)
	}

	testutil.AssertMsg(t, (got.CustomerOrderId == nil) == (want.CustomerOrderId == nil), "CustomerOrderId was nil and not nil")
	if got.CustomerOrderId != nil {
		testutil.AssertEqual(t, *got.CustomerOrderId, *want.CustomerOrderId)
	}
}

// CreateNewOffchainTxOrFail creates a new offchain or fail
func CreateNewOffchainTxOrFail(t *testing.T, db *db.DB, ln ln.AddLookupInvoiceClient,
	opts NewOffchainOpts) Offchain {
	payment, err := NewOffchain(db, ln, opts)
	if err != nil {
		testutil.FatalMsg(t,
			pkgErrors.Wrap(err, "wasn't able to create new payment"))
	}
	return payment
}
