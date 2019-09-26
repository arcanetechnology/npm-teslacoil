package payments

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/testutil"

	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"
	"gitlab.com/arcanecrypto/teslacoil/internal/users"
)

var (
	databaseConfig = testutil.GetDatabaseConfig("payments")
	testDB         *db.DB
)

const (
	succeed = "\u001b[32m\u2713"
	fail    = "\u001b[31m\u2717"
	reset   = "\u001b[0m"
)

var (
	firstMemo   = "HiMisterHey"
	description = "My personal description"
	secondMemo  = "HelloWorld"
	// address to testnet node running on lightningspin.com
	testnetAddress = "tb1q40gzxjcamcny49st7m8lyz9rtmssjgfefc33at"
	simnetAddress  = "sb1qnl462s336uu4n8xanhyvpega4zwjr9jrhc26x4"
)

func TestMain(m *testing.M) {
	build.SetLogLevel(logrus.ErrorLevel)

	testDB = testutil.InitDatabase(databaseConfig)

	flag.Parse()
	result := m.Run()

	os.Exit(result)
}

func TestNewPayment(t *testing.T) {
	t.Parallel()
	user := CreateUserOrFail(t)

	amount1 := rand.Int63n(4294967)
	amount2 := rand.Int63n(4294967)

	tests := []struct {
		memo        string
		description string
		amountSat   int64

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
				RHash:          testutil.SampleHash[:],
				RPreimage:      testutil.SamplePreimage,
				Settled:        false,
			},
			want: Payment{
				UserID:         user.ID,
				AmountSat:      amount1,
				AmountMSat:     amount1 * 1000,
				HashedPreimage: testutil.SampleHashHex,
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
				RHash:          testutil.SampleHash[:],
				RPreimage:      testutil.SamplePreimage,
				Settled:        false,
			},
			want: Payment{
				UserID:         user.ID,
				AmountSat:      amount2,
				AmountMSat:     amount2 * 1000,
				HashedPreimage: testutil.SampleHashHex,
				Memo:           &firstMemo,
				Description:    &description,
				Status:         Status("OPEN"),
				Direction:      Direction("INBOUND"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("create invoice with amount %d memo %s and description %s",
			tt.amountSat, tt.memo, tt.description), func(t *testing.T) {

			// Create Mock LND client with preconfigured invoice response
			mockLNcli := testutil.LightningMockClient{
				InvoiceResponse: tt.lndInvoice,
			}

			payment, err := NewPayment(testDB, mockLNcli, tt.want.UserID,
				tt.amountSat, tt.memo, tt.description)
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

	user := CreateUserOrFail(t)

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
				HashedPreimage: testutil.SampleHashHex,
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
				HashedPreimage: testutil.SampleHashHex,
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
	u, err := users.Create(testDB,
		users.CreateUserArgs{
			Email:    "test_userPayInvoice@example.com",
			Password: "password",
		})
	if err != nil {
		log.Error("User result was empty")
		t.Fatalf("%+v\n", err)
	}

	tx := testDB.MustBegin()
	_, err = users.IncreaseBalance(tx, users.ChangeBalance{
		UserID:    u.ID,
		AmountSat: 5000,
	})
	if err != nil {
		t.Fatalf("\t%s\tshould be able to PayInvoice. Error: %+v%s\n", fail, err, reset)
	}

	err = tx.Commit()
	if err != nil {
		t.Fatalf(
			"\t%s\tshould be able to PayInvoice. Error:  %+v\n%s",
			fail, err, reset)
	}

	var amount1 int64 = 5000
	var amount2 int64 = 2000
	testCases := []struct {
		paymentRequest string

		decodePayReq lnrpc.PayReq
		want         UserPaymentResponse
	}{
		{
			paymentRequest: "SomePaymentRequest1",

			decodePayReq: lnrpc.PayReq{
				PaymentHash: testutil.SampleHashHex,
				NumSatoshis: amount1,
			},
			want: UserPaymentResponse{
				Payment: Payment{
					UserID:         u.ID,
					AmountSat:      amount1,
					AmountMSat:     amount1 * 1000,
					Preimage:       &testutil.SamplePreimageHex,
					HashedPreimage: testutil.SampleHashHex,
					Status:         Status("SUCCEEDED"),
					Direction:      Direction("OUTBOUND"),
				},
				User: users.User{
					ID:      u.ID,
					Balance: 0,
				},
			},
		},
		{
			paymentRequest: "SomePaymentRequest2",

			decodePayReq: lnrpc.PayReq{
				PaymentHash: testutil.SampleHashHex,
				NumSatoshis: amount2,
			},
			want: UserPaymentResponse{
				Payment: Payment{
					UserID:         u.ID,
					AmountSat:      amount2,
					AmountMSat:     amount2 * 1000,
					Preimage:       &testutil.SamplePreimageHex,
					HashedPreimage: testutil.SampleHashHex,
					Status:         Status("SUCCEEDED"),
					Direction:      Direction("OUTBOUND"),
				},
				User: users.User{},
			},
		},
	}

	t.Log("testing paying invoice")
	for i, test := range testCases {
		t.Logf("\ttest: %d\twhen paying invoice %s for user %d",
			i, test.want.Payment.PaymentRequest, test.want.User.ID)
		user, err := users.GetByID(testDB, u.ID)
		if err != nil {
			t.Fatalf(
				"\t%s\tshould be able to GetByID. Error:  %+v\n%s",
				fail, err, reset)
		}

		// Create Mock LND client with preconfigured invoice response
		mockLNcli := testutil.LightningMockClient{
			InvoiceResponse: lnrpc.Invoice{},
			SendPaymentSyncResponse: lnrpc.SendResponse{
				PaymentPreimage: testutil.SamplePreimage,
				PaymentHash:     testutil.SampleHash[:],
			},
			DecodePayReqResponse: test.decodePayReq,
			// We need to define what DecodePayReq returns
		}
		payment, err := PayInvoice(
			testDB, &mockLNcli, u.ID, test.paymentRequest)
		if user.Balance < test.want.Payment.AmountSat {
			if payment.Payment.Status == SUCCEEDED || payment.Payment.Preimage != nil || payment.Payment.SettledAt != nil {
				testutil.FatalMsg(t, "should not pay invoice when the users balance is too low")
			}
			t.Logf(
				"\t%s\tshould not pay invoice when the users balance is too low%s",
				succeed, reset)

			if !strings.Contains(
				err.Error(),
				`new row for relation "users" violates check constraint "users_balance_check"`) {
				testutil.FatalMsgf(t,
					"should fail when paying invoice greater than balance. Error: %+v",
					err)
			}
			t.Logf(
				"\t%s\tshould fail when paying invoice greater than balance%s",
				succeed, reset)
			return
		}
		if err != nil {
			testutil.FatalMsgf(t,
				"should be able to PayInvoice. Error:  %+v",
				err)
		}
		t.Logf("\t%s\tShould be able to PayInvoice%s", succeed, reset)

		got := payment
		want := test.want

		assertPaymentsAreEqual(t, got.Payment, want.Payment)

		if got.User.ID != want.User.ID {
			t.Logf("\t%s\tID should be equal to expected ID. Expected \"%d\" got \"%d\"%s",
				fail,
				want.User.ID,
				payment.User.ID,
				reset,
			)
			t.Fail()
		}

		if got.User.Balance != want.User.Balance {
			t.Logf("\t%s\tBalance should be equal to expected Balance. Expected \"%d\" got \"%d\"%s",
				fail,
				want.User.Balance,
				payment.User.Balance,
				reset,
			)
			t.Fail()
		}
	}

}

// TODO: Add cases where the triggerInvoice .settled is false
// This case should return the exact same payment and an empty User
func TestUpdateInvoiceStatus(t *testing.T) {
	t.Parallel()
	// Arrange
	u := CreateUserOrFail(t)

	var amount1 int64 = 50000
	var amount2 int64 = 20000

	testCases := []struct {
		triggerInvoice lnrpc.Invoice
		memo           string
		amountSat      int64

		want UserPaymentResponse
	}{
		{
			lnrpc.Invoice{
				PaymentRequest: "SomePayRequest1",
				RHash:          testutil.SampleHash[:],
				RPreimage:      testutil.SamplePreimage,
				Settled:        true,
				Value:          int64(amount1),
			},
			firstMemo,
			amount1,

			UserPaymentResponse{
				Payment: Payment{
					UserID:         u.ID,
					AmountSat:      amount1,
					AmountMSat:     amount1 * 1000,
					HashedPreimage: testutil.SampleHashHex,
					Preimage:       &testutil.SamplePreimageHex,
					Memo:           &firstMemo,
					Status:         Status("SUCCEEDED"),
					Direction:      Direction("INBOUND"),
				},
				User: users.User{
					ID:      u.ID,
					Balance: amount1,
				},
			},
		},
		{
			lnrpc.Invoice{
				PaymentRequest: "SomePayRequest2",
				RHash:          testutil.SampleHash[:],
				RPreimage:      testutil.SamplePreimage,
				Settled:        true,
				Value:          int64(amount2),
			},
			secondMemo,
			amount2,

			UserPaymentResponse{
				Payment: Payment{
					UserID:         u.ID,
					AmountSat:      amount2,
					AmountMSat:     amount2 * 1000,
					HashedPreimage: testutil.SampleHashHex,
					Preimage:       &testutil.SamplePreimageHex,
					Memo:           &secondMemo,
					Status:         Status("SUCCEEDED"),
					Direction:      Direction("INBOUND"),
				},
				User: users.User{
					ID:      u.ID,
					Balance: 70000,
				},
			},
		},
		{
			lnrpc.Invoice{
				PaymentRequest: "SomePayRequest3",
				RHash:          testutil.SampleHash[:],
				RPreimage:      testutil.SamplePreimage,
				Settled:        false,
				Value:          int64(amount1),
			},
			firstMemo,
			amount1,

			UserPaymentResponse{
				Payment: Payment{
					UserID:         u.ID,
					AmountSat:      amount1,
					AmountMSat:     amount1 * 1000,
					HashedPreimage: testutil.SampleHashHex,
					Preimage:       &testutil.SamplePreimageHex,
					Memo:           &firstMemo,
					Status:         Status("OPEN"),
					Direction:      Direction("INBOUND"),
				},
				User: users.User{},
			},
		},
	}

	for _, test := range testCases {
		t.Run(fmt.Sprintf("when updating invoice with amount %d where balance should be %d after exectuion",
			test.amountSat, test.want.User.Balance),
			func(t *testing.T) {

				// Arrange
				_, err := NewPayment(testDB,
					testutil.LightningMockClient{
						InvoiceResponse: test.triggerInvoice,
					}, u.ID, test.amountSat, test.memo, "")
				if err != nil {
					testutil.FatalMsgf(t,
						"should be able to CreateInvoice. Error:  %+v\n",
						err)
				}

				// Act
				payment, err := UpdateInvoiceStatus(test.triggerInvoice, testDB)
				if err != nil {
					testutil.FatalMsgf(t,
						"should be able to UpdateInvoiceStatus. Error:  %+v\n",
						err)
				}

				// Assert
				got := payment.Payment
				want := test.want

				assertPaymentsAreEqual(t, got, want.Payment)

				if payment.User.ID != want.User.ID {
					testutil.FailMsgf(t,
						"ID should be equal to expected ID. Expected \"%d\" got \"%d\"",
						want.User.ID,
						payment.User.ID,
					)
				}

				if payment.User.Balance != want.User.Balance {
					testutil.FailMsgf(t,
						"balance should be equal to expected Balance. Expected \"%d\" got \"%d\"",
						want.User.Balance,
						payment.User.Balance,
					)
				}
			})
	}
}

func TestGetAllOffset(t *testing.T) {
	testutil.DescribeTest(t)

	// Arrange
	user := CreateUserOrFail(t)

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
		_, err := NewPayment(testDB,
			testutil.LightningMockClient{
				InvoiceResponse: lnrpc.Invoice{
					Value: int64(invoice.AmountSat),
					Memo:  invoice.Memo,
				},
			},
			user.ID,
			invoice.AmountSat,
			invoice.Memo,
			"")
		if err != nil {
			testutil.FatalMsg(t, errors.Wrap(err, "could not create invoice"))
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

	user := CreateUserOrFail(t)

	for _, invoice := range testInvoices {
		_, err := NewPayment(testDB,
			testutil.LightningMockClient{
				InvoiceResponse: lnrpc.Invoice{
					Value: int64(invoice.AmountSat),
					Memo:  invoice.Memo,
				},
			},
			user.ID,
			invoice.AmountSat,
			invoice.Memo,
			"")
		if err != nil {
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
			t.Fatalf("\t%s\tshould be able to GetAll. Error: %+v%s",
				fail, err, reset)
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

	user := CreateUserOrFail(t)

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

	user := CreateUserOrFail(t)

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
					testutil.FailMsg(t, "could not insert payment")
				}
				_ = tx.Commit()

				if payment.Expired {
					testutil.FailMsg(t, "payment should not be expired")
				}
			})
	}
}

func TestWithdrawOnChainBadOpts(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		scenario  string
		balance   int64
		amountSat int64
	}{
		{
			scenario:  "withdraw more than balance",
			balance:   1000,
			amountSat: 2000,
		},
		{
			scenario:  "withdraw negative amount",
			balance:   1000,
			amountSat: -5000,
		},
		{
			scenario:  "withdraw 0 amount",
			balance:   2000,
			amountSat: 0,
		},
	}
	mockLNcli := testutil.LightningMockClient{
		SendCoinsResponse: lnrpc.SendCoinsResponse{
			Txid: "owrgkpoaerkgpok",
		},
	}

	for _, test := range testCases {
		user := CreateUserWithBalanceOrFail(t, test.balance)

		t.Run(test.scenario, func(t *testing.T) {
			txid, err := WithdrawOnChain(testDB, mockLNcli, WithdrawOnChainArgs{
				UserID:    user.ID,
				AmountSat: test.amountSat,
				Address:   simnetAddress,
			})
			if err == nil || txid != nil {
				testutil.FatalMsgf(t, "should not send transaction, bad opts")
			}
		})
	}

}

func TestWithdrawOnChainSendAll(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		balance int64
		// We specify amountSat to make sure it is ignored when sendAll is true
		amountSat int64
	}{
		{
			balance:   10000,
			amountSat: 500000,
		},
		{
			balance:   20000,
			amountSat: -500000,
		},
		{
			balance:   500, // 20 000
			amountSat: 0,
		},
	}

	for _, test := range testCases {

		user := CreateUserWithBalanceOrFail(t, test.balance)

		t.Run("can withdraw on-chain", func(t *testing.T) {

			mockLNcli := testutil.LightningMockClient{
				SendCoinsResponse: lnrpc.SendCoinsResponse{
					Txid: "owrgkpoaerkgpok",
				},
			}

			_, err := WithdrawOnChain(testDB, mockLNcli, WithdrawOnChainArgs{
				UserID:    user.ID,
				AmountSat: test.amountSat,
				Address:   testnetAddress,
				SendAll:   true,
			})
			if err != nil {
				testutil.FatalMsgf(t, "could not WithdrawOnChain %+v", err)
			}

			// TODO: Test this creates transactions for the right amount
			// t.Run("withdrew the right amount", func(t *testing.T) {
			// Look up the txid on-chain, and check the amount
			// fmt.Println("txid: ", txid)
			// })
		})

		// Assert
		t.Run("users balance is 0", func(t *testing.T) {
			user, err := users.GetByID(testDB, user.ID)
			if err != nil {
				testutil.FatalMsgf(t, "could not get user %+v", err)
			}
			if user.Balance != 0 {
				testutil.FatalMsgf(t, "users balance was not 0 %+v", err)
			}
		})
	}
}

func assertPaymentsAreEqual(t *testing.T, got, want Payment) {
	t.Helper()
	if got.UserID != want.UserID {
		t.Logf("\t%s\tUserID should be equal to expected UserID. Expected \"%d\" got \"%d\"%s",
			fail, want.UserID, got.UserID, reset)
		t.Fail()
	}

	if got.AmountSat != want.AmountSat {
		t.Logf("\t%s\tAmountSat should be equal to expected AmountSat. Expected \"%d\" got \"%d\"%s",
			fail, want.AmountSat, got.AmountSat, reset)
		t.Fail()
	}

	if got.AmountMSat != want.AmountMSat {
		t.Logf("\t%s\tAmountMSat should be equal to expected AmountMSat. Expected \"%d\" got \"%d\"%s",
			fail, want.AmountMSat, got.AmountMSat, reset)
		t.Fail()
	}

	if got.Preimage != nil && want.Preimage != nil && *got.Preimage != *want.Preimage {
		t.Logf("\t%s\tPreimage should be equal to expected Preimage. Expected \"%v\" got \"%v\"%s",
			fail, want.Preimage, got.Preimage, reset)
		t.Fail()
	}

	if got.HashedPreimage != want.HashedPreimage {
		t.Logf("\t%s\tHashedPreimage should be equal to expected HashedPreimage. Expected \"%s\" got \"%s\"%s",
			fail, want.HashedPreimage, got.HashedPreimage, reset)
		t.Fail()
	}

	if (got.Memo != nil && want.Memo == nil) ||
		(got.Memo == nil && want.Memo != nil) {
		testutil.FatalMsgf(t, "Memos arent equal. Expected: %v, got: %v",
			*got.Memo, want.Memo)
	}

	if got.Memo != nil && want.Memo != nil && *got.Memo != *want.Memo {
		testutil.FatalMsgf(t, "Memo should be equal to expected Memo. Expected \"%s\" got \"%s\"",
			*want.Memo, *got.Memo)
	}

	if (got.Description != nil && want.Description == nil) ||
		(got.Description == nil && want.Description != nil) {
		testutil.FatalMsgf(t, "Descriptions arent equal. Expected: %v, got: %v",
			got.Description, want.Description)
	}

	if got.Description != nil && want.Description != nil && *got.Description != *want.Description {
		testutil.FatalMsgf(t, "Descriptions should be equal to expected Memo. Expected \"%s\" got \"%s\"",
			*want.Description, *got.Description)
	}

	if got.Status != want.Status {
		t.Logf("\t%s\tStatus should be equal to expected Status. Expected \"%s\" got \"%s\"%s",
			fail, want.Status, got.Status, reset)
		t.Fail()
	}

	if got.Direction != want.Direction {
		t.Logf("\t%s\tDirection should be equal to expected Direction. Expected \"%s\" got \"%s\"%s",
			fail, want.Direction, got.Direction, reset)
		t.Fail()
	}
	if !t.Failed() {
		testutil.Succeed(t, "all values should be equal to expected values")
	}
}

// CreateUserOrFail is a util function for creating a user
func CreateUserOrFail(t *testing.T) users.User {
	u, err := users.Create(testDB, users.CreateUserArgs{
		Email:    testutil.GetTestEmail(t),
		Password: "password",
	})
	if err != nil {
		testutil.FatalMsgf(t,
			"CreateUser(%s, db) -> should be able to CreateUser. Error:  %+v",
			t.Name(), err)
	}

	return u
}

func CreateUserWithBalanceOrFail(t *testing.T, balance int64) users.User {
	u := CreateUserOrFail(t)

	tx := testDB.MustBegin()
	user, err := users.IncreaseBalance(tx, users.ChangeBalance{
		UserID:    u.ID,
		AmountSat: balance,
	})
	if err != nil {
		testutil.FatalMsgf(t,
			"[%s] could not increase balance by %d for user %d: %+v", t.Name(),
			balance, u.ID, err)
	}
	err = tx.Commit()
	if err != nil {
		testutil.FatalMsg(t, "could not commit tx")
	}

	if user.Balance != balance {
		testutil.FatalMsgf(t, "wrong balance, expected [%d], got [%d]", balance,
			user.Balance)
	}

	return user
}
