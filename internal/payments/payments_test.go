package payments

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"

	"github.com/lightningnetwork/lnd/lnrpc"
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

func TestMain(m *testing.M) {
	build.SetLogLevel(logrus.ErrorLevel)

	testDB = testutil.InitDatabase(databaseConfig)

	flag.Parse()
	result := m.Run()

	os.Exit(result)
}

func TestCreateInvoice(t *testing.T) {
	t.Parallel()
	user, err := users.Create(testDB,
		"test_userCreateInvoice@example.com",
		"password",
	)
	if err != nil {
		log.Error("User result was empty")
		t.Fatalf("%+v\n", err)
	}

	amount1 := rand.Int63n(4294967)
	amount2 := rand.Int63n(4294967)

	tests := []struct {
		memo      string
		amountSat int64

		lndInvoice lnrpc.Invoice
		want       Payment
	}{
		{
			"HiMisterHey",
			amount1,

			lnrpc.Invoice{
				Value:          int64(amount1),
				PaymentRequest: "SomePayRequest",
				RHash:          testutil.SampleHash[:],
				RPreimage:      testutil.SamplePreimage,
				Settled:        false,
			},
			Payment{
				UserID:         user.ID,
				AmountSat:      amount1,
				AmountMSat:     amount1 * 1000,
				HashedPreimage: testutil.SampleHashHex,
				Memo:           "HiMisterHey",
				Description:    "My personal description",
				Status:         Status("OPEN"),
				Direction:      Direction("INBOUND"),
			},
		},
		{
			"HelloWorld",
			amount2,

			lnrpc.Invoice{
				Value:          int64(amount2),
				PaymentRequest: "SomePayRequest",
				RHash:          testutil.SampleHash[:],
				RPreimage:      testutil.SamplePreimage,
				Settled:        false,
			},
			Payment{
				UserID:         user.ID,
				AmountSat:      amount2,
				AmountMSat:     amount2 * 1000,
				HashedPreimage: testutil.SampleHashHex,
				Memo:           "HelloWorld",
				Description:    "My personal description",
				Status:         Status("OPEN"),
				Direction:      Direction("INBOUND"),
			},
		},
	}

	t.Log("Testing adding invoices to the DB")
	{
		for i, tt := range tests {
			t.Logf("\tTest: %d\tWhen creating invoice with amount %d and memo %s",
				i, tt.amountSat, tt.memo)
			{
				// Create Mock LND client with preconfigured invoice response
				mockLNcli := testutil.LightningMockClient{
					InvoiceResponse: tt.lndInvoice,
				}

				payment, err := CreateInvoice(testDB, mockLNcli, tt.want.UserID,
					tt.amountSat, "", tt.memo)
				if err != nil {
					t.Fatalf("\t%s\tShould be able to CreateInvoice %+v\n%s", fail, err, reset)
				}
				t.Logf("\t%s\tShould be able to CreateInvoice%s", succeed, reset)

				// Assertions
				{
					got := payment
					want := tt.want

					assertPaymentsAreEqual(t, got, want)
				}
			}
		}
	}

	// Fail tests after all assertions that will not interfere with eachother
	// for improved test result readability.
	if t.Failed() {
		t.FailNow()
	}
}

func TestGetByID(t *testing.T) {
	t.Parallel()

	const email1 = "email1@example.com"
	const password1 = "password1"
	const email2 = "email2@example.com"
	const password2 = "password2"
	amount1 := rand.Int63n(4294967)
	amount2 := rand.Int63n(4294967)

	user, err := users.Create(testDB,
		"test_userGetByID@example.com",
		"password",
	)
	if err != nil {
		t.Fatalf("Should be able to create user, got error %v", err)
	}

	tests := []struct {
		email    string
		password string
		want     Payment
	}{
		{

			email1,
			password1,
			Payment{
				UserID:         user.ID,
				AmountSat:      amount1,
				AmountMSat:     amount1 * 1000,
				HashedPreimage: testutil.SampleHashHex,
				Memo:           "HiMisterHey",
				Description:    "My personal description",
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
				Memo:           "HelloWorld",
				Description:    "My personal description",
				Status:         Status("OPEN"),
				Direction:      Direction("INBOUND"),
			},
		},
	}

	t.Log("testing getting payments by ID")
	{
		for i, tt := range tests {
			t.Logf("\ttest: %d\twhen inserting payment for user %d and amount %d",
				i, tt.want.UserID, tt.want.AmountSat)
			{
				tx := testDB.MustBegin()
				payment, err := insert(tx, tt.want)
				if tt.want.HashedPreimage != "" && tt.want.Preimage != nil {
					if !strings.Contains(err.Error(), "cant supply both a preimage and a hashed preimage") {
						t.Error("Is in there")
						t.Fatalf(
							"\t%s\tshould return error when preimage AND hashed preimage supplied. Error:  %+v\n%s",
							fail, err, reset)
					}
					t.Logf("\t%s\tshould return error when preimage AND hashed preimage supplied%s", succeed, reset)
					return
				}

				if err != nil {
					t.Fatalf(
						"\t%s\tShould be able to insertPayment. Error:  %+v\n%s",
						fail, err, reset)
				}
				_ = tx.Commit()
				t.Logf("\t%s\tShould be able to insertPayment%s", succeed, reset)

				// Act
				payment, err = GetByID(testDB, payment.ID, tt.want.UserID)
				if err != nil {
					t.Fatalf(
						"\t%s\tShould be able to GetByID. Error: %+v\n%s",
						fail, err, reset)
				}
				t.Logf("\t%s\tShould be able to GetByID%s", succeed, reset)

				{
					assertPaymentsAreEqual(t, payment, tt.want)
				}
			}
		}
	}

	// Fail tests after all assertions that will not interfere with eachother
	// for improved test result readability.
	if t.Failed() {
		t.FailNow()
	}
}

func TestPayInvoice(t *testing.T) {
	t.Parallel()
	// Setup the database
	u, err := users.Create(testDB,
		"test_userPayInvoice@example.com",
		"password",
	)
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
	tests := []struct {
		paymentRequest string
		memo           string

		decodePayReq lnrpc.PayReq
		want         UserPaymentResponse
	}{
		{
			"SomePaymentRequest1",
			"HelloPayment",

			lnrpc.PayReq{
				PaymentHash: testutil.SampleHashHex,
				NumSatoshis: int64(amount1),
				Description: "HelloPayment",
			},
			UserPaymentResponse{
				Payment: Payment{
					UserID:         u.ID,
					AmountSat:      amount1,
					AmountMSat:     amount1 * 1000,
					Preimage:       &testutil.SamplePreimageHex,
					HashedPreimage: testutil.SampleHashHex,
					Memo:           "HelloPayment",
					Description:    "My personal description",
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
			"SomePaymentRequest2",
			"HelloPayment",

			lnrpc.PayReq{
				PaymentHash: testutil.SampleHashHex,
				NumSatoshis: int64(amount2),
				Description: "HelloPayment",
			},
			UserPaymentResponse{
				Payment: Payment{
					UserID:         u.ID,
					AmountSat:      amount2,
					AmountMSat:     amount2 * 1000,
					Preimage:       &testutil.SamplePreimageHex,
					HashedPreimage: testutil.SampleHashHex,
					Memo:           "HelloPayment",
					Description:    "My personal description",
					Status:         Status("SUCCEEDED"),
					Direction:      Direction("OUTBOUND"),
				},
				User: users.User{},
			},
		},
	}

	t.Log("testing paying invoice")
	{
		for i, tt := range tests {
			t.Logf("\ttest: %d\twhen paying invoice %s for user %d",
				i, tt.want.Payment.PaymentRequest, tt.want.User.ID)
			{
				log.Info("preimage value is... ", tt.want.Payment.Preimage)
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
					DecodePayReqResponse: tt.decodePayReq,
					// We need to define what DecodePayReq returns
				}
				payment, err := PayInvoice(
					testDB, &mockLNcli, u.ID, tt.paymentRequest, "", tt.memo)
				log.Infof("invoice response is %+v", mockLNcli.InvoiceResponse)
				if user.Balance < tt.want.Payment.AmountSat {
					if payment.Payment.Status == succeeded || payment.Payment.Preimage != nil || payment.Payment.SettledAt != nil {
						t.Fatalf(
							"\t%s\tshould not pay invoice when the users balance is too low\n%s",
							fail, reset)
					}
					t.Logf(
						"\t%s\tshould not pay invoice when the users balance is too low%s",
						succeed, reset)

					if !strings.Contains(
						err.Error(),
						`could not construct user update: pq: new row for relation "users" violates check constraint "users_balance_check"`) {
						t.Fatalf(
							"\t%s\tshould fail when paying invoice greater than balance. Error:  %+v%s",
							fail, err, reset)
					}
					t.Logf(
						"\t%s\tshould fail when paying invoice greater than balance%s",
						succeed, reset)
					return
				}
				if err != nil {
					t.Fatalf(
						"\t%s\tshould be able to PayInvoice. Error:  %+v\n%s",
						fail, err, reset)
				}
				t.Logf("\t%s\tShould be able to PayInvoice%s", succeed, reset)

				{
					got := payment
					want := tt.want

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
		}
	}

	// Fail tests after all assertions that will not interfere with eachother
	// for improved test result readability.
	if t.Failed() {
		t.FailNow()
	}

}

// TODO: Add cases where the triggerInvoice .settled is false
// This case should return the exact same payment and an empty User
func TestUpdateInvoiceStatus(t *testing.T) {
	t.Parallel()
	// Arrange
	u, err := users.Create(testDB,
		"test_userUpdateInvoiceStatus@example.com",
		"password",
	)
	if err != nil {
		log.Error("User result was empty")
		t.Fatalf("%+v\n", err)
	}

	var amount1 int64 = 50000
	var amount2 int64 = 20000

	tests := []struct {
		triggerInvoice lnrpc.Invoice
		memo           string
		description    string
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
			"HelloWorld",
			"My description",
			amount1,

			UserPaymentResponse{
				Payment: Payment{
					UserID:         u.ID,
					AmountSat:      amount1,
					AmountMSat:     amount1 * 1000,
					HashedPreimage: testutil.SampleHashHex,
					Preimage:       &testutil.SamplePreimageHex,
					Memo:           "HelloWorld",
					Description:    "My description",
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
			"HelloWorld",
			"My description",
			amount2,

			UserPaymentResponse{
				Payment: Payment{
					UserID:         u.ID,
					AmountSat:      amount2,
					AmountMSat:     amount2 * 1000,
					HashedPreimage: testutil.SampleHashHex,
					Preimage:       &testutil.SamplePreimageHex,
					Memo:           "HelloWorld",
					Description:    "My description",
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
			"HelloWorld",
			"My description",
			amount1,

			UserPaymentResponse{
				Payment: Payment{
					UserID:         u.ID,
					AmountSat:      amount1,
					AmountMSat:     amount1 * 1000,
					HashedPreimage: testutil.SampleHashHex,
					Preimage:       &testutil.SamplePreimageHex,
					Memo:           "HelloWorld",
					Description:    "My description",
					Status:         Status("OPEN"),
					Direction:      Direction("INBOUND"),
				},
				User: users.User{},
			},
		},
	}

	t.Log("testing updating invoice status")
	{
		for i, tt := range tests {
			t.Logf("\ttest: %d\twhen updating invoice with amout %d where balance should be %d after execution",
				i, tt.amountSat, tt.want.User.Balance)
			{
				_, err := CreateInvoice(testDB,
					testutil.LightningMockClient{
						InvoiceResponse: tt.triggerInvoice,
					}, u.ID, tt.amountSat, tt.description, tt.memo)
				if err != nil {
					t.Fatalf(
						"\t%s\tshould be able to CreateInvoice. Error:  %+v\n%s",
						fail, err, reset)
				}
				t.Logf("\t%s\tshould be able to CreateInvoice%s", succeed, reset)

				payment, err := UpdateInvoiceStatus(tt.triggerInvoice, testDB)
				if err != nil {
					t.Fatalf(
						"\t%s\tshould be able to UpdateInvoiceStatus. Error:  %+v\n%s",
						fail, err, reset)
				}
				t.Logf("\t%s\tShould be able to UpdateInvoiceStatus%s", succeed, reset)

				{
					got := payment.Payment
					want := tt.want

					assertPaymentsAreEqual(t, got, want.Payment)

					if payment.User.ID != want.User.ID {
						t.Logf("\t%s\tID should be equal to expected ID. Expected \"%d\" got \"%d\"%s",
							fail,
							want.User.ID,
							payment.User.ID,
							reset,
						)
						t.Fail()
					}

					if payment.User.Balance != want.User.Balance {
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
		}
	}

	// Fail tests after all assertions that will not interfere with eachother
	// for improved test result readability.
	if t.Failed() {
		t.FailNow()
	}
}

func TestGetAll(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scenario string

		invoices []struct {
			Memo      string
			AmountSat int64
		}

		limit  int
		offset int

		expectedNumberOfInvoices int
	}{
		{
			"adding 3 invoices, and getting first 50",

			[]struct {
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
			},
			50,
			0,

			3,
		},
		{
			"adding 3 invoices, and only gets two",

			[]struct {
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
			},
			2,
			0,

			2,
		},
		{
			"adding 3 invoices, and skips first 2",

			[]struct {
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
			},
			50,
			2,

			1,
		},
	}

	t.Log("testing get all")
	{
		for i, tt := range tests {
			t.Logf("\ttest: %d\twhen testing scenario \"%s\", GetAll should return %d invoices",
				i, tt.scenario, tt.expectedNumberOfInvoices)
			{
				u, err := users.Create(testDB,
					fmt.Sprintf("test_user%d@example.com", i),
					"password",
				)
				if err != nil {
					t.Fatalf(
						"\t%s\tshould be able to CreateUser. Error:  %+v\n%s",
						fail, err, reset)
				}
				t.Logf("\t%s\tshould be able to CreateUser%s", succeed, reset)

				for i, invoice := range tt.invoices {
					// Create Mock LND client with preconfigured invoice response
					mockLNcli := testutil.LightningMockClient{
						InvoiceResponse: lnrpc.Invoice{
							Value: int64(invoice.AmountSat),
							PaymentRequest: fmt.Sprintf("PayRequest_%d_%d",
								u.ID, i),
							RHash:     testutil.SampleHash[:],
							RPreimage: testutil.SamplePreimage,
							Memo:      invoice.Memo,
							Settled:   false,
						},
					}

					_, err := CreateInvoice(
						testDB, mockLNcli, u.ID, invoice.AmountSat, "",
						invoice.Memo)

					if err != nil {
						t.Fatalf(
							"\t%s\tshould be able to CreateInvoice. Error:  %+v\n%s",
							fail, err, reset)
					}
				}
				t.Logf("\t%s\tshould be able to CreateInvoices%s", succeed, reset)

				invoices, err := GetAll(testDB, u.ID, tt.limit, tt.offset)
				if err != nil {
					t.Fatalf("\t%s\tshould be able to GetAll. Error: %+v%s",
						fail, err, reset)
				}
				t.Logf("\t%s\tshould be able to GetAll%s", succeed, reset)
				numberOfInvoices := len(invoices)

				{
					if tt.expectedNumberOfInvoices != numberOfInvoices {
						t.Logf("\t%s\texpectedNumberofInvoices should be equal to expected numberOfInvoices. Expected \"%d\" got \"%d\"%s",
							fail,
							tt.expectedNumberOfInvoices,
							numberOfInvoices, reset)

						t.Fail()
					}

					for i, invoice := range invoices {
						if i < tt.offset {
							if tt.invoices[i].Memo == invoice.Memo {
								t.Logf("\t%s\tMemo should not be equal to expected memo. Expected \"%s\" got \"%s\"%s",
									fail,
									tt.invoices[i].Memo,
									invoice.Memo,
									reset)
								t.Fail()
							}
							if tt.invoices[i].AmountSat == invoice.AmountSat {
								t.Logf("\t%s\tMemo should not be equal to expected memo. Expected \"%s\" got \"%s\"%s",
									fail,
									tt.invoices[i].Memo,
									invoice.Memo,
									reset)
								t.Fail()
							}
							if invoice.UserID != u.ID {
								t.Logf("\t%s\tUserID should not be equal to expected UserID. Expected \"%d\" got \"%d\"%s",
									fail,
									u.ID,
									invoice.UserID,
									reset)
								t.Fail()
							}
						} else {

							if tt.invoices[i].Memo != invoice.Memo {
								t.Logf("\t%s\tMemo should be equal to expected memo. Expected \"%s\" got \"%s\"%s",
									fail,
									tt.invoices[i].Memo,
									invoice.Memo, reset)
								t.Fail()
							}
							if tt.invoices[i].AmountSat != invoice.AmountSat {
								t.Logf("\t%s\tMemo should be equal to expected memo. Expected \"%s\" got \"%s\"%s",
									fail,
									tt.invoices[i].Memo,
									invoice.Memo,
									reset)
								t.Fail()
							}
							if invoice.UserID != u.ID {
								t.Logf("\t%s\tUserID should be equal to expected UserID. Expected \"%d\" got \"%d\"%s",
									fail,
									u.ID,
									invoice.UserID,
									reset)
								t.Fail()
							}
						}
					}
					if !t.Failed() {
						t.Logf("\t%s\tAll values should be equal to expected values%s", succeed, reset)
					}
				}
			}
		}
	}

	// Fail tests after all assertions that will not interfere with eachother
	// for improved test result readability.
	if t.Failed() {
		t.FailNow()
	}
}

func assertPaymentsAreEqual(t *testing.T, got, want Payment) {
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

	if got.Memo != want.Memo {
		t.Logf("\t%s\tMemo should be equal to expected Memo. Expected \"%s\" got \"%s\"%s",
			fail, want.Memo, got.Memo, reset)
		t.Fail()
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
		t.Logf("\t%s\tAll values should be equal to expected values%s", succeed, reset)
	}
}
