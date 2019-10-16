package payments

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit"
	"gitlab.com/arcanecrypto/teslacoil/internal/users"

	"github.com/lightningnetwork/lnd/lnrpc"
	pkgErrors "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/asyncutil"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/apikeys"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/ln"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/lntestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/userstestutil"

	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"
)

var (
	databaseConfig = testutil.GetDatabaseConfig("payments")
	testDB         *db.DB
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
	// address to testnet node running on lightningspin.com
)

func TestMain(m *testing.M) {
	build.SetLogLevel(logrus.DebugLevel)

	testDB = testutil.InitDatabase(databaseConfig)

	flag.Parse()
	result := m.Run()

	os.Exit(result)
}

func TestNewPayment(t *testing.T) {
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
		want       Payment
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
			want: Payment{
				UserID:         user.ID,
				AmountSat:      amount1,
				AmountMSat:     amount1 * 1000,
				HashedPreimage: SampleHashHex,
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
			want: Payment{
				UserID:         user.ID,
				AmountSat:      amount2,
				AmountMSat:     amount2 * 1000,
				HashedPreimage: SampleHashHex,
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
			want: Payment{
				UserID:          user.ID,
				AmountSat:       amount3,
				AmountMSat:      amount3 * 1000,
				Memo:            &firstMemo,
				HashedPreimage:  SampleHashHex,
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

			payment, err := NewPayment(testDB, mockLNcli, NewPaymentOpts{
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
			got := payment
			want := tt.want

			assertPaymentsAreEqual(t, got, want)
		})
	}
}

func TestGetByID(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	const email1 = "email1@example.com"
	const password1 = "password1"
	const email2 = "email2@example.com"
	const password2 = "password2"
	amount1 := rand.Int63n(4294967)
	amount2 := rand.Int63n(4294967)

	user := userstestutil.CreateUserOrFail(t, testDB)

	testCases := []struct {
		email          string
		password       string
		expectedResult Payment
	}{
		{

			email1,
			password1,
			Payment{
				UserID:         user.ID,
				AmountSat:      amount1,
				AmountMSat:     amount1 * 1000,
				HashedPreimage: SampleHashHex,
				Memo:           &firstMemo,
				Description:    &description,
				Status:         Status("OPEN"),
				Direction:      Direction("INBOUND"),
			},
		},
		{

			email2,
			password2,
			Payment{
				UserID:         user.ID,
				AmountSat:      amount2,
				AmountMSat:     amount2 * 1000,
				HashedPreimage: SampleHashHex,
				Memo:           &secondMemo,
				Description:    &description,
				Status:         Status("OPEN"),
				Direction:      Direction("INBOUND"),
			},
		},
	}

	for _, test := range testCases {
		t.Run(fmt.Sprintf("GetByID() for payment with amount %d", test.expectedResult.AmountSat),
			func(t *testing.T) {

				tx := testDB.MustBegin()

				payment, err := insert(tx, test.expectedResult)
				/* TODO: Move these assertions to it's own test: `TestInsertWithBadOpts`
				 * Right now, there are no inputs causing assertion to be made,
				 * therefore it is commented out
				if test.expectedResult.HashedPreimage != "" && test.expectedResult.Preimage != nil {
					if !strings.Contains(err.Error(), "cant supply both a preimage and a hashed preimage") {
						testutil.FatalMsgf(t,
							"should return error when preimage AND hashed preimage supplied. Error:  %+v",
							err)
					}
					testutil.Succeed(t, "should return error when preimage AND hashed preimage supplied")
					return
				}
				*/

				if err != nil {
					testutil.FatalMsgf(t, "should be able to insertPayment. Error:  %+v",
						err)
				}
				_ = tx.Commit()

				// Act
				payment, err = GetByID(testDB, payment.ID, test.expectedResult.UserID)
				if err != nil {
					testutil.FatalMsgf(t, "should be able to GetByID. Error: %+v", err)
				}

				assertPaymentsAreEqual(t, payment, test.expectedResult)
			})
	}
}

func TestPayInvoice(t *testing.T) {
	t.Parallel()
	// Setup the database
	user := userstestutil.CreateUserWithBalanceOrFail(t, testDB, ln.MaxAmountMsatPerInvoice*5)

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
	paymentRequest := "SomePaymentRequest1"
	expectedPayment := Payment{
		UserID:         user.ID,
		AmountSat:      amount,
		AmountMSat:     amount * 1000,
		Preimage:       &SamplePreimageHex,
		HashedPreimage: SampleHashHex,
		Direction:      OUTBOUND,
		Status:         SUCCEEDED,
	}

	t.Run("paying invoice decreases balance of user", func(t *testing.T) {

		_, err := PayInvoice(
			testDB, &mockLNcli, user.ID, paymentRequest)
		if err != nil {
			testutil.FatalMsgf(t, "could not pay invoice: %v", err)
		}

		updatedUser, err := users.GetByID(testDB, user.ID)
		if err != nil {
			testutil.FatalMsg(t, err)
		}

		testutil.AssertEqual(t, updatedUser.Balance, user.Balance-amount)
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

		expectedPayment.SettledAt = got.SettledAt
		expectedPayment.Status = SUCCEEDED
		expectedPayment.Preimage = got.Preimage

		assertPaymentsAreEqual(t, *got, expectedPayment)
	})
	t.Run("successfully paying invoice marks invoice settledAt date", func(t *testing.T) {
		paymentRequest := "SomePaymentRequest1"

		got, err := PayInvoice(
			testDB, &mockLNcli, user.ID, paymentRequest)
		if err != nil {
			testutil.FatalMsgf(t, "could not pay invoice: %v", err)
		}

		expectedPayment.SettledAt = got.SettledAt
		expectedPayment.Status = SUCCEEDED

		assertPaymentsAreEqual(t, *got, expectedPayment)

		updatedInvoice, _ := GetByID(testDB, got.ID, user.ID)

		if updatedInvoice.SettledAt == nil {
			testutil.FailMsgf(t, "expected settledAt to be defined, but was <nil>")
		}
	})
	t.Run("successfully paying invoice marks invoice settledAt date", func(t *testing.T) {
		paymentRequest := "SomePaymentRequest1"

		got, err := PayInvoice(
			testDB, &mockLNcli, user.ID, paymentRequest)
		if err != nil {
			testutil.FatalMsgf(t, "could not pay invoice: %v", err)
		}

		expectedPayment.SettledAt = got.SettledAt
		expectedPayment.Status = SUCCEEDED

		assertPaymentsAreEqual(t, *got, expectedPayment)

		updatedInvoice, err := GetByID(testDB, got.ID, user.ID)
		if err != nil {
			testutil.FatalMsgf(t, "could not getbyid: %v", err)
		}

		if updatedInvoice.SettledAt == nil {
			testutil.FailMsgf(t, "expected settledAt to be defined, but was <nil>")
		}
	})
}

// TODO: Add cases where the triggerInvoice .settled is false
// This case should return the exact same payment and an empty User
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
		payment := CreateNewPaymentOrFail(t, testDB, lnMock, NewPaymentOpts{
			UserID:      u.ID,
			AmountSat:   mockInvoice.Value,
			CallbackURL: "https://example.com",
		})

		testutil.AssertMsg(t, payment.CallbackURL != nil,
			"Callback URL was nil! Payment: "+payment.String())
		invoice := lnrpc.Invoice{
			PaymentRequest: payment.PaymentRequest,
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
		if err := asyncutil.Await(8,
			time.Millisecond*20, checkPostSent); err != nil {
			testutil.FatalMsg(t, err)
		}
	})

	t.Run("callback URL should not be called with non-settled invoice", func(t *testing.T) {
		t.Parallel()
		lnMock := lntestutil.GetLightningMockClient()
		httpPoster := testutil.GetMockHttpPoster()
		mockInvoice, _ := ln.AddInvoice(lnMock, lnrpc.Invoice{})
		payment := CreateNewPaymentOrFail(t, testDB, lnMock, NewPaymentOpts{
			UserID:      u.ID,
			AmountSat:   mockInvoice.Value,
			CallbackURL: "https://example.com",
		})

		invoice := lnrpc.Invoice{
			PaymentRequest: payment.PaymentRequest,
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
		if err := asyncutil.Await(4,
			time.Millisecond*20, checkPostSent); err == nil {
			testutil.FatalMsgf(t, "HTTP POSTer sent out callback for non-settled payment")
		}
	})
}

func TestGetAllOffset(t *testing.T) {
	testutil.DescribeTest(t)

	// Arrange
	user := userstestutil.CreateUserOrFail(t, testDB)

	testInvoices := []struct {
		Memo      string
		AmountSat int64
	}{
		{
			Memo:      "1",
			AmountSat: 20001,
		},
		{

			Memo:      "2",
			AmountSat: 20002,
		},
		{
			Memo:      "3",
			AmountSat: 20003,
		},
	}

	for _, invoice := range testInvoices {
		if _, err := NewPayment(testDB,
			lntestutil.LightningMockClient{
				InvoiceResponse: lnrpc.Invoice{
					Value: int64(invoice.AmountSat),
					Memo:  invoice.Memo,
				},
			},
			NewPaymentOpts{
				UserID:    user.ID,
				AmountSat: invoice.AmountSat,
				Memo:      invoice.Memo,
			}); err != nil {
			testutil.FatalMsg(t, pkgErrors.Wrap(err, "could not create invoice"))
		}
	}

	testCases := []struct {
		offset                   int
		expectedNumberOfInvoices int
	}{
		{
			offset:                   0,
			expectedNumberOfInvoices: 3,
		},
		{
			offset:                   1,
			expectedNumberOfInvoices: 2,
		},
		{
			offset:                   3,
			expectedNumberOfInvoices: 0,
		},
		{
			offset:                   5000,
			expectedNumberOfInvoices: 0,
		},
	}

	for _, test := range testCases {
		t.Run(fmt.Sprintf("GetAll() with offset %d expects %d invoices",
			test.offset, test.expectedNumberOfInvoices),
			func(t *testing.T) {

				invoices, err := GetAll(testDB, user.ID, 10, test.offset)
				if err != nil {
					testutil.FatalMsgf(t, "should be able to GetAll. Error: %+v", err)
				}
				numberOfInvoices := len(invoices)

				if test.expectedNumberOfInvoices != numberOfInvoices {
					testutil.FatalMsgf(t,
						"expectedNumberofInvoices should be equal to expected numberOfInvoices. Expected %q got %q",
						test.expectedNumberOfInvoices,
						numberOfInvoices)
				}

				for i, invoice := range invoices {

					if test.offset > len(testInvoices) {
						testutil.FatalMsg(t, "offset was greater than number of testinvoices, aborting test")
					}

					// We add test.offset to i to skip 'test.offset' invoices
					expectedInvoice := testInvoices[i+test.offset]

					if invoice.Memo != nil && expectedInvoice.Memo != *invoice.Memo {
						testutil.FailMsgf(t, "Memo should be equal to expected memo. Expected %q got %q",
							expectedInvoice.Memo,
							*invoice.Memo)
					}
					if invoice.AmountSat != expectedInvoice.AmountSat {
						testutil.FailMsgf(t, "AmountSat should be equal to expected AmountSat. Expected %q got %q",
							expectedInvoice.AmountSat,
							invoice.AmountSat)
					}
					if invoice.UserID != user.ID {
						testutil.FailMsgf(t, "UserID should be equal to expected UserID. Expected %q got %q",
							user.ID,
							invoice.UserID)
					}
				}
			})
	}
}

func TestGetAllLimit(t *testing.T) {
	// Arrange
	testInvoices := []struct {
		Memo      string
		AmountSat int64
	}{
		{
			Memo:      "1",
			AmountSat: 20001,
		},
		{

			Memo:      "2",
			AmountSat: 20002,
		},
		{
			Memo:      "3",
			AmountSat: 20003,
		},
	}

	user := userstestutil.CreateUserOrFail(t, testDB)

	for _, invoice := range testInvoices {
		if _, err := NewPayment(testDB,
			lntestutil.LightningMockClient{
				InvoiceResponse: lnrpc.Invoice{
					Value: int64(invoice.AmountSat),
					Memo:  invoice.Memo,
				},
			},
			NewPaymentOpts{
				UserID:    user.ID,
				AmountSat: invoice.AmountSat,
				Memo:      invoice.Memo,
			}); err != nil {
			testutil.FatalMsg(t, "could not create invoice")
		}
	}

	testCases := []struct {
		limit                    int
		expectedNumberOfInvoices int
	}{
		{
			limit:                    50,
			expectedNumberOfInvoices: 3,
		},
		{
			limit:                    3,
			expectedNumberOfInvoices: 3,
		},
		{
			limit:                    1,
			expectedNumberOfInvoices: 1,
		},
		{
			limit:                    0,
			expectedNumberOfInvoices: 0,
		},
	}

	for _, test := range testCases {

		invoices, err := GetAll(testDB, user.ID, test.limit, 0)
		if err != nil {
			testutil.FatalMsg(t, err)
		}
		numberOfInvoices := len(invoices)

		if test.expectedNumberOfInvoices != numberOfInvoices {
			testutil.FailMsgf(t, "expectedNumberofInvoices should be equal to expected numberOfInvoices. Expected %q got %q",
				test.expectedNumberOfInvoices,
				numberOfInvoices)
		}

		for i, invoice := range invoices {

			expectedInvoice := testInvoices[i]

			if invoice.Memo != nil && expectedInvoice.Memo != *invoice.Memo {
				testutil.FailMsgf(t, "Memo should be equal to expected memo. Expected %q got %q",
					expectedInvoice.Memo,
					*invoice.Memo)
			}
			if invoice.AmountSat != expectedInvoice.AmountSat {
				testutil.FailMsgf(t, "AmountSat should be equal to expected AmountSat. Expected %q got %q",
					expectedInvoice.AmountSat,
					invoice.AmountSat)
			}
			if invoice.UserID != user.ID {
				testutil.FailMsgf(t, "UserID should be equal to expected UserID. Expected %q got %q",
					user.ID,
					invoice.UserID)
			}
		}
	}
}

func TestWithAdditionalFieldsShouldBeExpired(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	user := userstestutil.CreateUserOrFail(t, testDB)

	payment := Payment{
		UserID:         user.ID,
		HashedPreimage: "f747dbf93249644a71749b6fff7c5a9eb7c1526c52ad3414717e222470940c57",
		Expiry:         1,
		Direction:      Direction("INBOUND"),
		Status:         Status("OPEN"),
		AmountSat:      100,
		AmountMSat:     100000,
	}

	tx := testDB.MustBegin()
	payment, err := insert(tx, payment)
	if err != nil {
		testutil.FailMsg(t, "could not insert payment")
	}
	_ = tx.Commit()

	// Sleep for expiry to check if expired property is set
	// correctly. expired should be true
	time.Sleep(time.Second * time.Duration(payment.Expiry))

	payment = payment.WithAdditionalFields()

	if !payment.Expired {
		testutil.FailMsg(t, "payment should be expired")
	}

	if payment.ExpiresAt != payment.CreatedAt.Add(time.Second*time.Duration(payment.Expiry)) {
		testutil.FailMsg(t, "expiresAt should equal createdAt + expiry")
	}
}

func TestWithAdditionalFields(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	user := userstestutil.CreateUserOrFail(t, testDB)

	invoices := []Payment{
		Payment{
			UserID:         user.ID,
			HashedPreimage: "f747dbf93249644a71749b6fff7c5a9eb7c1526c52ad3414717e222470940c57",
			Expiry:         3600,
			Direction:      Direction("INBOUND"),
			Status:         Status("OPEN"),
			AmountSat:      100,
			AmountMSat:     100000,
		},
		Payment{
			UserID:         user.ID,
			HashedPreimage: "f747dbf93249644a71749b6fff7c5a9eb7c1526c52ad3414717e222470940c57",
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

				tx := testDB.MustBegin()
				payment, err := insert(tx, invoice)
				if err != nil {
					testutil.FatalMsg(t, "could not insert payment")
				}
				_ = tx.Commit()

				if payment.Expired {
					testutil.FailMsg(t, "payment should not be expired")
				}
			})
	}
}

func assertPaymentsAreEqual(t *testing.T, got, want Payment) {
	t.Helper()
	testutil.AssertEqual(t, got.UserID, want.UserID, "userID")
	testutil.AssertEqual(t, got.AmountSat, want.AmountSat, "amountSat")
	testutil.AssertEqual(t, got.AmountMSat, want.AmountMSat, "amountMSat")

	testutil.AssertMsg(t, (got.Preimage == nil) == (want.Preimage == nil), "Preimage was nil and not nil")
	if got.Preimage != nil {
		testutil.AssertEqual(t, *got.Preimage, *want.Preimage)
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
