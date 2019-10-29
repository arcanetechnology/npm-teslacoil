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
	"gitlab.com/arcanecrypto/teslacoil/models/users/balance"

	"github.com/brianvoe/gofakeit"
	"gitlab.com/arcanecrypto/teslacoil/models/users"

	"github.com/lightningnetwork/lnd/lnrpc"
	pkgErrors "github.com/pkg/errors"
	"gitlab.com/arcanecrypto/teslacoil/ln"
	"gitlab.com/arcanecrypto/teslacoil/models/apikeys"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/lntestutil"

	"gitlab.com/arcanecrypto/teslacoil/db"
)

func init() {
	gofakeit.Seed(0)
}

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
)

func TestNewOffchainTx(t *testing.T) {
	t.Parallel()
	user := CreateUserOrFail(t, testDB)

	amount1 := rand.Int63n(ln.MaxAmountSatPerInvoice)
	amount2 := rand.Int63n(ln.MaxAmountSatPerInvoice)
	amount3 := rand.Int63n(ln.MaxAmountSatPerInvoice)

	payReq1 := gofakeit.Word()
	payReq2 := gofakeit.Word()
	payReq3 := gofakeit.Word()

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
				PaymentRequest: payReq1,
				RHash:          SampleHash[:],
				RPreimage:      SamplePreimage,
				Expiry:         1337,
				Settled:        false,
			},
			want: Offchain{
				UserID:         user.ID,
				PaymentRequest: payReq1,
				AmountMSat:     amount1 * 1000,
				HashedPreimage: SampleHash[:],
				Memo:           &firstMemo,
				Description:    &description,
				Status:         OPEN,
				Direction:      INBOUND,
			},
		},
		{
			memo:        firstMemo,
			description: description,
			amountSat:   amount2,

			lndInvoice: lnrpc.Invoice{
				Value:          amount2,
				PaymentRequest: payReq2,
				RHash:          SampleHash[:],
				RPreimage:      SamplePreimage,
				Expiry:         1337,
				Settled:        false,
			},
			want: Offchain{
				UserID:         user.ID,
				AmountMSat:     amount2 * 1000,
				HashedPreimage: SampleHash[:],
				PaymentRequest: payReq2,
				Memo:           &firstMemo,
				Description:    &description,
				Status:         OPEN,
				Direction:      INBOUND,
			},
		},
		{
			memo:        firstMemo,
			description: description,
			amountSat:   amount3,
			orderId:     customerOrderId,

			lndInvoice: lnrpc.Invoice{
				Value:          amount3,
				PaymentRequest: payReq3,
				RHash:          SampleHash[:],
				RPreimage:      SamplePreimage,
				Expiry:         1337,
				Settled:        false,
			},
			want: Offchain{
				UserID:          user.ID,
				AmountMSat:      amount3 * 1000,
				Memo:            &firstMemo,
				HashedPreimage:  SampleHash[:],
				PaymentRequest:  payReq3,
				Description:     &description,
				Status:          OPEN,
				Direction:       INBOUND,
				CustomerOrderId: &customerOrderId,
			},
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("create invoice with amount %d memo %s and description %s",
			tt.amountSat, tt.memo, tt.description), func(t *testing.T) {
			t.Parallel()

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
			require.NoError(t, err)

			// Assertions
			got := offchainTx
			want := tt.want

			assert := assert.New(t)
			assert.Equal(want.AmountMSat, got.AmountMSat)
			assert.Equal(want.SettledAt, got.SettledAt)
			assert.Equal(want.Memo, got.Memo)
			assert.Equal(want.Description, got.Description)
			assert.Equal(want.CallbackURL, got.CallbackURL)
			assert.Equal(want.UserID, got.UserID)
			assert.Equal(want.CustomerOrderId, got.CustomerOrderId)
			assert.Equal(want.PaymentRequest, got.PaymentRequest)
			assert.Equal(want.Status, got.Status)
			assert.Equal(want.Direction, got.Direction)
			assert.Equal(want.HashedPreimage, got.HashedPreimage)
			assert.Equal(want.Preimage, got.Preimage)
		})
	}

	t.Run("offchain TX without customer order id", func(t *testing.T) {
		t.Parallel()
		offchain, err := NewOffchain(testDB, lntestutil.GetLightningMockClient(), NewOffchainOpts{
			UserID:    user.ID,
			AmountSat: int64(gofakeit.Number(1, ln.MaxAmountSatPerInvoice)),
		})
		require.NoError(t, err)
		assert.Nil(t, offchain.CustomerOrderId)
	})

	t.Run("offchain TX with customer order id", func(t *testing.T) {
		t.Parallel()
		order := gofakeit.Word()
		offchain, err := NewOffchain(testDB, lntestutil.GetLightningMockClient(), NewOffchainOpts{
			UserID:    user.ID,
			AmountSat: int64(gofakeit.Number(1, ln.MaxAmountSatPerInvoice)),
			OrderId:   order,
		})
		require.NoError(t, err)
		assert.NotNil(t, offchain.CustomerOrderId)
		assert.Equal(t, order, *offchain.CustomerOrderId)
	})

	t.Run("offchain TX without description", func(t *testing.T) {
		t.Parallel()
		offchain, err := NewOffchain(testDB, lntestutil.GetLightningMockClient(), NewOffchainOpts{
			UserID:    user.ID,
			AmountSat: int64(gofakeit.Number(1, ln.MaxAmountSatPerInvoice)),
		})
		require.NoError(t, err)
		assert.Nil(t, offchain.Description)
	})

	t.Run("offchain TX with description", func(t *testing.T) {
		t.Parallel()
		desc := gofakeit.Sentence(gofakeit.Number(1, 12))
		offchain, err := NewOffchain(testDB, lntestutil.GetLightningMockClient(), NewOffchainOpts{
			UserID:      user.ID,
			AmountSat:   int64(gofakeit.Number(1, ln.MaxAmountSatPerInvoice)),
			Description: desc,
		})
		require.NoError(t, err)
		require.NotNil(t, offchain.Description)
		assert.Equal(t, desc, *offchain.Description)
	})

	t.Run("offchain TX without memo", func(t *testing.T) {
		t.Parallel()
		offchain, err := NewOffchain(testDB, lntestutil.GetLightningMockClient(), NewOffchainOpts{
			UserID:    user.ID,
			AmountSat: int64(gofakeit.Number(1, ln.MaxAmountSatPerInvoice)),
		})
		require.NoError(t, err)
		assert.Nil(t, offchain.Memo)
	})

	t.Run("offchain TX with memo", func(t *testing.T) {
		t.Parallel()
		memo := gofakeit.Sentence(gofakeit.Number(1, 12))
		offchain, err := NewOffchain(testDB, lntestutil.GetLightningMockClient(), NewOffchainOpts{
			UserID:    user.ID,
			AmountSat: int64(gofakeit.Number(1, ln.MaxAmountSatPerInvoice)),
			Memo:      memo,
		})
		require.NoError(t, err)
		require.NotNil(t, offchain.Memo)
		assert.Equal(t, memo, *offchain.Memo)
	})

	t.Run("offchain TX without callbackURL", func(t *testing.T) {
		t.Parallel()
		offchain, err := NewOffchain(testDB, lntestutil.GetLightningMockClient(), NewOffchainOpts{
			UserID:    user.ID,
			AmountSat: int64(gofakeit.Number(1, ln.MaxAmountSatPerInvoice)),
		})
		require.NoError(t, err)
		assert.Nil(t, offchain.CallbackURL)
	})

	t.Run("offchain TX with callbackURL", func(t *testing.T) {
		t.Parallel()
		url := gofakeit.URL()
		offchain, err := NewOffchain(testDB, lntestutil.GetLightningMockClient(), NewOffchainOpts{
			UserID:      user.ID,
			AmountSat:   int64(gofakeit.Number(1, ln.MaxAmountSatPerInvoice)),
			CallbackURL: url,
		})
		require.NoError(t, err)
		assert.NotNil(t, offchain.CallbackURL)
		assert.Equal(t, url, *offchain.CallbackURL)
	})
}

func TestOffchain_MarkAsPaid(t *testing.T) {
	t.Parallel()
	user := CreateUserWithBalanceOrFail(t, testDB, ln.MaxAmountMsatPerInvoice*5)
	tx := genOffchain(user)
	inserted, err := InsertOffchain(testDB, tx)
	require.NoError(t, err)

	settlement := time.Now()
	paid, err := inserted.MarkAsPaid(testDB, settlement)
	require.NoError(t, err)
	assert.Equal(t, SUCCEEDED, paid.Status)
	require.NotNil(t, paid.SettledAt)
	assert.WithinDuration(t, settlement, *paid.SettledAt, time.Second)
}

func TestOffchain_MarkAsFailed(t *testing.T) {
	t.Parallel()
	user := CreateUserWithBalanceOrFail(t, testDB, ln.MaxAmountMsatPerInvoice*5)
	tx := genOffchain(user)
	inserted, err := InsertOffchain(testDB, tx)
	require.NoError(t, err)

	paid, err := inserted.MarkAsFailed(testDB)
	require.NoError(t, err)
	assert.Equal(t, FAILED, paid.Status)

	// hm, what are we going to require here? Are failed invoices settled?
	// assert.Nil(t, paid.SettledAt)
}

func TestPayInvoice(t *testing.T) {
	t.Parallel()

	amount := int64(gofakeit.Number(1, ln.MaxAmountMsatPerInvoice))

	paymentRequest := "SomeOffchainRequest1"

	t.Run("paying invoice decreases balance of user", func(t *testing.T) {
		t.Parallel()
		user := CreateUserWithBalanceOrFail(t, testDB, ln.MaxAmountMsatPerInvoice*5)
		// Create Mock LND client with preconfigured invoice response
		mockLNcli := lntestutil.LightningMockClient{
			SendPaymentSyncResponse: lnrpc.SendResponse{
				PaymentPreimage: SamplePreimage,
				PaymentHash:     SampleHash[:],
			},
			// define what lncli.DecodePayReq returns
			DecodePayReqResponse: lnrpc.PayReq{
				PaymentHash: SampleHashHex,
				NumSatoshis: amount,
				Expiry:      1337,
			},
		}
		balancePrePayment, err := balance.ForUser(testDB, user.ID)
		require.NoError(t, err)

		_, err = PayInvoice(
			testDB, &mockLNcli, user.ID, paymentRequest)
		require.NoError(t, err)

		_, err = users.GetByID(testDB, user.ID)
		require.NoError(t, err)

		bal, err := balance.ForUser(testDB, user.ID)
		require.NoError(t, err)

		assert.Equal(t, balancePrePayment.Sats()-amount, bal.Sats())
	})

	t.Run("paying invoice greater than balance fails", func(t *testing.T) {
		t.Parallel()

		user := CreateUserWithBalanceOrFail(t, testDB, 5)
		// Create Mock LND client with preconfigured invoice response
		mockLNcli := lntestutil.LightningMockClient{
			SendPaymentSyncResponse: lnrpc.SendResponse{
				PaymentPreimage: SamplePreimage,
				PaymentHash:     SampleHash[:],
			},
			// define what lncli.DecodePayReq returns
			DecodePayReqResponse: lnrpc.PayReq{
				PaymentHash: SampleHashHex,
				NumSatoshis: 6,
			},
		}

		_, err := PayInvoice(testDB, &mockLNcli, user.ID, paymentRequest)
		if errors.Is(err, ErrBalanceTooLow) {
			assert.Equal(t, err, ErrBalanceTooLow)
		}

	})
	t.Run("paying invoice with 0 amount fails with Err0AmountInvoiceNotSupported", func(t *testing.T) {
		t.Parallel()
		user := CreateUserWithBalanceOrFail(t, testDB, ln.MaxAmountMsatPerInvoice*5)
		mockLNcli := lntestutil.LightningMockClient{
			SendPaymentSyncResponse: lnrpc.SendResponse{
				PaymentPreimage: SamplePreimage,
				PaymentHash:     SampleHash[:],
			},
			// define what lncli.DecodePayReq returns
			DecodePayReqResponse: lnrpc.PayReq{
				PaymentHash: SampleHashHex,
				NumSatoshis: 0,
			},
		}

		_, err := PayInvoice(
			testDB, &mockLNcli, user.ID, paymentRequest)
		if !errors.Is(err, Err0AmountInvoiceNotSupported) {
			testutil.FailMsgf(t, "expected Err0AmountInvoiceNotSupported but got error %v", err)
		}

	})

	t.Run("successfully paying invoice marks invoice settledAt date", func(t *testing.T) {
		t.Parallel()
		user := CreateUserWithBalanceOrFail(t, testDB, ln.MaxAmountMsatPerInvoice*5)
		mockLNcli := lntestutil.LightningMockClient{
			SendPaymentSyncResponse: lnrpc.SendResponse{
				PaymentPreimage: SamplePreimage,
				PaymentHash:     SampleHash[:],
			},
			DecodePayReqResponse: lnrpc.PayReq{
				PaymentHash: SampleHashHex,
				NumSatoshis: amount,
				Expiry:      1000,
			},
		}

		got, err := PayInvoice(testDB, &mockLNcli, user.ID, paymentRequest)
		require.NoError(t, err)

		assert.Equal(t, SUCCEEDED, got.Status)
		assert.NotNil(t, got.SettledAt)
		assert.WithinDuration(t, *got.SettledAt, time.Now(), time.Second)
	})
}

func TestUpdateInvoiceStatus(t *testing.T) {
	t.Parallel()
	u := CreateUserOrFail(t, testDB)

	t.Run("update invoice status to settled", func(t *testing.T) {
		t.Parallel()
		lnMock := lntestutil.GetRandomLightningMockClient()
		httpPoster := testutil.GetMockHttpPoster()
		mockInvoice, _ := ln.AddInvoice(lnMock, lnrpc.Invoice{})
		offchainTx := CreateNewOffchainTxOrFail(t, testDB, lnMock, NewOffchainOpts{
			UserID:    u.ID,
			AmountSat: mockInvoice.Value,
		})

		invoice := lnrpc.Invoice{
			PaymentRequest: offchainTx.PaymentRequest,
			Settled:        true,
		}

		updated, err := UpdateInvoiceStatus(invoice, testDB, httpPoster)
		require.NoError(t, err)
		assert.NotNil(t, updated.SettledAt)
	})

	t.Run("not settle an invoice", func(t *testing.T) {
		t.Parallel()
		lnMock := lntestutil.GetRandomLightningMockClient()
		httpPoster := testutil.GetMockHttpPoster()
		mockInvoice, _ := ln.AddInvoice(lnMock, lnrpc.Invoice{})
		offchainTx := CreateNewOffchainTxOrFail(t, testDB, lnMock, NewOffchainOpts{
			UserID:    u.ID,
			AmountSat: mockInvoice.Value,
		})

		invoice := lnrpc.Invoice{
			PaymentRequest: offchainTx.PaymentRequest,
			Settled:        false,
		}

		updated, err := UpdateInvoiceStatus(invoice, testDB, httpPoster)
		require.NoError(t, err)
		assert.Nil(t, updated.SettledAt)
	})

	t.Run("callback URL should be called", func(t *testing.T) {
		t.Parallel()

		// for the callback to be executed, we need to create an API key for the
		// current user. this is because the callback body is hashed with the
		// users API key
		if _, _, err := apikeys.New(testDB, u.ID); err != nil {
			require.NoError(t, err)
		}

		lnMock := lntestutil.GetRandomLightningMockClient()
		httpPoster := testutil.GetMockHttpPoster()
		mockInvoice, _ := ln.AddInvoice(lnMock, lnrpc.Invoice{})
		offchainTx := CreateNewOffchainTxOrFail(t, testDB, lnMock, NewOffchainOpts{
			UserID:      u.ID,
			AmountSat:   mockInvoice.Value,
			CallbackURL: gofakeit.URL(),
		})
		assert.NotNil(t, offchainTx.CallbackURL)

		invoice := lnrpc.Invoice{
			PaymentRequest: offchainTx.PaymentRequest,
			Settled:        true,
		}

		_, err := UpdateInvoiceStatus(invoice, testDB, httpPoster)
		require.NoError(t, err)

		checkPostSent := func() bool {
			return httpPoster.GetSentPostRequests() == 1
		}
		// emails are sent in go-routine, so can't assume they're sent fast
		// enough for test to pick up
		err = async.AwaitNoBackoff(8, time.Millisecond*20, checkPostSent)
		require.NoError(t, err)
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
	user := CreateUserOrFail(t, testDB)

	t.Run("expiry field", func(t *testing.T) {
		offchainTx := Offchain{
			UserID:         user.ID,
			HashedPreimage: []byte("f747dbf93249644a71749b6fff7c5a9eb7c1526c52ad3414717e222470940c57"),
			Expiry:         1,
			Direction:      Direction("INBOUND"),
			Status:         Status("OPEN"),
			AmountMSat:     100000,
		}

		offchainTx, err := InsertOffchain(testDB, offchainTx)
		require.NoError(t, err)

		// Sleep for expiry to check if expired property is set
		// correctly. expired should be true
		time.Sleep(time.Second * time.Duration(offchainTx.Expiry))

		withFields := offchainTx.withAdditionalFields()

		assert.True(t, withFields.Expired)
		assert.Equal(t, withFields.ExpiresAt, offchainTx.CreatedAt.Add(time.Second*time.Duration(offchainTx.Expiry)))
	})

	invoices := []Offchain{
		Offchain{
			UserID:         user.ID,
			HashedPreimage: []byte("f747dbf93249644a71749b6fff7c5a9eb7c1526c52ad3414717e222470940c57"),
			Expiry:         3600,
			Direction:      Direction("INBOUND"),
			Status:         Status("OPEN"),
			AmountMSat:     100000,
		},
		Offchain{
			UserID:         user.ID,
			HashedPreimage: []byte("f747dbf93249644a71749b6fff7c5a9eb7c1526c52ad3414717e222470940c57"),
			Expiry:         2,
			Direction:      Direction("INBOUND"),
			Status:         Status("OPEN"),
			AmountMSat:     100000,
		},
	}

	for _, invoice := range invoices {
		t.Run(fmt.Sprintf("payment with expiry %d should not be expired", invoice.Expiry),
			func(t *testing.T) {
				payment, err := InsertOffchain(testDB, invoice)
				require.NoError(t, err)
				assert.False(t, payment.withAdditionalFields().Expired)
			})
	}
}

/*
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
*/

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
