package payments

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path"
	"testing"

	"github.com/lightningnetwork/lnd/lnrpc"
	"google.golang.org/grpc"

	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"
	"gitlab.com/arcanecrypto/teslacoil/internal/users"
)

var sampleRPreimage = []byte("SomePreimage")
var samplePreimage = hex.EncodeToString(sampleRPreimage)
var migrationsPath = path.Join("file://",
	os.Getenv("GOPATH"),
	"/src/gitlab.com/arcanecrypto/teslacoil/internal/platform/migrations")

const (
	succeed = "\u001b[32m\u2713"
	fail    = "\u001b[31m\u2717"
	reset   = "\u001b[0m"
)

type lightningMockClient struct {
	InvoiceResponse         lnrpc.Invoice
	SendPaymentSyncResponse lnrpc.SendResponse
	DecodePayReqRespons     lnrpc.PayReq
}

func (client lightningMockClient) AddInvoice(ctx context.Context,
	in *lnrpc.Invoice, opts ...grpc.CallOption) (
	*lnrpc.AddInvoiceResponse, error) {
	return &lnrpc.AddInvoiceResponse{}, nil
}

func (client lightningMockClient) LookupInvoice(ctx context.Context,
	in *lnrpc.PaymentHash, opts ...grpc.CallOption) (*lnrpc.Invoice, error) {
	return &client.InvoiceResponse, nil
}

func (client lightningMockClient) DecodePayReq(ctx context.Context,
	in *lnrpc.PayReqString, opts ...grpc.CallOption) (*lnrpc.PayReq, error) {
	return &client.DecodePayReqRespons, nil
}

func (client lightningMockClient) SendPaymentSync(ctx context.Context,
	in *lnrpc.SendRequest, opts ...grpc.CallOption) (
	*lnrpc.SendResponse, error) {
	return &client.SendPaymentSyncResponse, nil
}

func TestMain(m *testing.M) {
	testDB, err := db.OpenTestDatabase()
	if err != nil {
		fmt.Printf("%+v\n", err)
		return
	}

	if err = db.CreateTestDatabase(testDB); err != nil {
		fmt.Println(err)
		return
	}

	flag.Parse()
	result := m.Run()

	db.TeardownTestDB(testDB)
	os.Exit(result)
}

func TestCreateInvoice(t *testing.T) {
	// Setup the database
	testDB, err := db.OpenTestDatabase()
	if err != nil {
		t.Fatalf("%+v\n", err)
	}
	user, err := users.Create(testDB,
		"test_userCreateInvoice@example.com",
		"password",
	)
	if err != nil || user == nil {
		fmt.Println("User result was empty")
		t.Fatalf("%+v\n", err)
	}

	amount1 := rand.Int63n(4294967)
	amount2 := rand.Int63n(4294967)

	tests := []struct {
		createInvoiceData CreateInvoiceData
		lndInvoice        lnrpc.Invoice
		out               Payment
	}{
		{

			CreateInvoiceData{
				Memo:      "HiMisterHey",
				AmountSat: amount1,
			},
			lnrpc.Invoice{
				Value:          int64(amount1),
				PaymentRequest: "SomePayRequest",
				RHash:          []byte("SomeRHash"),
				RPreimage:      sampleRPreimage,
				Settled:        false,
			},
			Payment{
				UserID:         user.ID,
				AmountSat:      amount1,
				AmountMSat:     amount1 * 1000,
				Preimage:       &samplePreimage,
				HashedPreimage: hex.EncodeToString([]byte("SomeRHash")),
				Memo:           "HiMisterHey",
				Description:    "My personal description",
				Status:         Status("OPEN"),
				Direction:      Direction("INBOUND"),
			},
		},
		{

			CreateInvoiceData{
				Memo:      "HelloWorld",
				AmountSat: amount2,
			},
			lnrpc.Invoice{
				Value:          amount2,
				PaymentRequest: "SomePayRequest",
				RHash:          []byte("SomeRHash"),
				RPreimage:      sampleRPreimage,
				Settled:        false,
			},
			Payment{
				UserID:         user.ID,
				AmountSat:      amount2,
				AmountMSat:     amount2 * 1000,
				Preimage:       &samplePreimage,
				HashedPreimage: hex.EncodeToString([]byte("SomeRHash")),
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
				i, tt.createInvoiceData.AmountSat, tt.createInvoiceData.Memo)
			{
				// Create Mock LND client with preconfigured invoice response
				mockLNcli := lightningMockClient{
					InvoiceResponse: tt.lndInvoice,
				}

				payment, err := CreateInvoice(testDB, mockLNcli, tt.createInvoiceData, tt.out.UserID)
				if err != nil {
					t.Fatalf("\t%s\tShould be able to CreateInvoice %+v\n%s", fail, err, reset)
				}
				t.Logf("\t%s\tShould be able to CreateInvoice%s", succeed, reset)

				// Assertions
				{
					expectedResult := tt.out

					assertPaymentsAreEqual(t, payment, expectedResult)
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
	// Prepare
	testDB, err := db.OpenTestDatabase()
	if err != nil {
		t.Fatalf("%+v\n", err)
	}

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

	tests := []struct {
		email    string
		password string
		out      Payment
	}{
		{

			email1,
			password1,
			Payment{
				UserID:         user.ID,
				AmountSat:      amount1,
				AmountMSat:     amount1 * 1000,
				Preimage:       &samplePreimage,
				HashedPreimage: hex.EncodeToString([]byte("SomeRHash")),
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
				Preimage:       &samplePreimage,
				HashedPreimage: hex.EncodeToString([]byte("SomeRHash")),
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
				i, tt.out.UserID, tt.out.AmountSat)
			{
				tx := testDB.MustBegin()
				payment, err := insertPayment(tx, tt.out)
				if err != nil {
					t.Fatalf(
						"\t%s\tShould be able to insertPayment. Error:  %+v\n%s",
						fail, err, reset)
				}
				tx.Commit()
				t.Logf("\t%s\tShould be able to insertPayment%s", succeed, reset)

				// Act
				payment, err = GetByID(testDB, payment.ID, tt.out.UserID)
				if err != nil {
					t.Fatalf(
						"\t%s\tShould be able to GetByID. Error: %+v\n%s",
						fail, err, reset)
				}
				t.Logf("\t%s\tShould be able to GetByID%s", succeed, reset)

				{
					assertPaymentsAreEqual(t, payment, tt.out)
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

func TestUpdateUserBalance(t *testing.T) {
	// Prepare
	testDB, err := db.OpenTestDatabase()
	if err != nil {
		t.Fatalf("%+v\n", err)
	}
	u, err := users.Create(testDB,
		"test_userUpdateUserBalance@example.com",
		"password",
	)
	if err != nil || u == nil {
		fmt.Println("User result was empty")
		t.Fatalf("%+v\n", err)
	}

	tests := []struct {
		amount int64
		out    UserResponse
	}{
		{

			10000,
			UserResponse{
				ID:      u.ID,
				Balance: 10000,
			},
		},
	}

	t.Log("testing getting payments by ID")
	{
		for i, tt := range tests {
			t.Logf("\ttest: %d\twhen updating balance by %d for user %d",
				i, tt.out.Balance, tt.out.ID)
			{
				tx := testDB.MustBegin()
				user, err := updateUserBalance(tx, u.ID, tt.amount)
				if err != nil {
					t.Fatalf(
						"\t%s\tshould be able to updateUserBalance. Error:  %+v\n%s",
						fail, err, reset)
				}
				err = tx.Commit()
				if err != nil {
					t.Fatalf(
						"\t%s\tShould be able to commit db tx. Error:  %+v\n%s",
						fail, err, reset)
				}
				t.Logf("\t%s\tShould be able to updateUserBalance%s", succeed, reset)

				{
					expectedResult := tt.out
					if user.ID != expectedResult.ID {
						t.Logf("\t%s\tID should be equal to expected ID. Expected \"%d\" got \"%d\"%s",
							fail,
							expectedResult.ID,
							user.ID,
							reset,
						)
						t.Fail()
					}

					if user.Balance != expectedResult.Balance {
						t.Logf("\t%s\tStatus should be equal to expected Status. Expected \"%d\" got \"%d\"%s",
							fail,
							expectedResult.Balance,
							user.Balance,
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

func TestPayInvoice(t *testing.T) {
	// Setup the database
	testDB, err := db.OpenTestDatabase()
	if err != nil {
		t.Fatalf("%+v\n", err)
	}
	u, err := users.Create(testDB,
		"test_userPayInvoice@example.com",
		"password",
	)
	if err != nil || u == nil {
		fmt.Println("User result was empty")
		t.Fatalf("%+v\n", err)
	}

	tests := []struct {
		payInvoiceData PayInvoiceData

		out UserPaymentResponse
	}{
		{
			PayInvoiceData{
				PaymentRequest: "SomePaymentHash",
				Memo:           "HelloPayment",
			},
			UserPaymentResponse{
				Payment: Payment{
					UserID:         u.ID,
					AmountSat:      5000,
					AmountMSat:     5000000,
					Preimage:       &samplePreimage,
					HashedPreimage: "SomeHash",
					Memo:           "HelloPayment",
					Description:    "My personal description",
					Status:         Status("SUCCEEDED"),
					Direction:      Direction("OUTBOUND"),
				},
				User: UserResponse{
					ID:      u.ID,
					Balance: 5000, // Test user starts with 100k satoshi
				},
			},
		},
	}

	// Create Mock LND client with preconfigured invoice response
	mockLNcli := lightningMockClient{
		InvoiceResponse: lnrpc.Invoice{},
		SendPaymentSyncResponse: lnrpc.SendResponse{
			PaymentPreimage: []byte("SomePreimage"),
			PaymentHash:     []byte("SomeRHash"),
		},
		// We need to define what DecodePayReq returns
		DecodePayReqRespons: lnrpc.PayReq{
			PaymentHash: "SomeHash",
			NumSatoshis: 5000,
			Description: "HelloPayment",
		},
	}

	t.Log("testing paying invoice")
	{
		for i, tt := range tests {
			t.Logf("\ttest: %d\twhen paying invoice %s for user %d",
				i, tt.out.Payment.PaymentRequest, tt.out.User.ID)
			{
				payment, err := PayInvoice(
					testDB, mockLNcli, tt.payInvoiceData, u.ID)
				if err != nil {
					t.Fatalf(
						"\t%s\tshould be able to PayInvoice. Error:  %+v\n%s",
						fail, err, reset)
				}
				t.Logf("\t%s\tShould be able to PayInvoice%s", succeed, reset)

				{
					expectedResult := tt.out.User

					assertPaymentsAreEqual(t, payment.Payment, tt.out.Payment)

					if payment.User.ID != expectedResult.ID {
						t.Logf("\t%s\tID should be equal to expected ID. Expected \"%d\" got \"%d\"%s",
							fail,
							expectedResult.ID,
							payment.User.ID,
							reset,
						)
						t.Fail()
					}

					if payment.User.Balance != expectedResult.Balance {
						t.Logf("\t%s\tBalance should be equal to expected Balance. Expected \"%d\" got \"%d\"%s",
							fail,
							expectedResult.Balance,
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

func TestUpdateInvoiceStatus(t *testing.T) {
	// Arrange
	testDB, err := db.OpenTestDatabase()
	if err != nil {
		t.Fatalf("%+v\n", err)
	}
	u, err := users.Create(testDB,
		"test_userUpdateInvoiceStatus@example.com",
		"password",
	)
	if err != nil || u == nil {
		fmt.Println("User result was empty")
		t.Fatalf("%+v\n", err)
	}

	tests := []struct {
		triggerInvoice lnrpc.Invoice

		out UserPaymentResponse
	}{
		{

			lnrpc.Invoice{
				PaymentRequest: "SomePayRequest1",
				RHash:          []byte("SomeHash"),
				RPreimage:      []byte("SomePreimage"),
				Settled:        true,
				Value:          20000,
			},

			UserPaymentResponse{
				Payment: Payment{
					UserID:         u.ID,
					AmountSat:      20000,
					AmountMSat:     20000000,
					Preimage:       &samplePreimage,
					HashedPreimage: hex.EncodeToString([]byte("SomeHash")),
					Memo:           "HelloWorld",
					Description:    "My description",
					Status:         Status("SUCCEEDED"),
					Direction:      Direction("INBOUND"),
				},
				User: UserResponse{
					ID:      u.ID,
					Balance: 20000, // Test user starts with 100k satoshi
				},
			},
		},
	}

	t.Log("testing updating invoice status")
	{
		for i, tt := range tests {
			t.Logf("\ttest: %d\twhen updating invoice status %s for user %d",
				i, tt.out.Payment.PaymentRequest, tt.out.User.ID)
			{
				_, err := CreateInvoice(testDB,
					lightningMockClient{
						InvoiceResponse: tt.triggerInvoice,
					},
					CreateInvoiceData{
						Memo:        "HelloWorld",
						Description: "My description",
						AmountSat:   20000,
					}, u.ID)
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
					expectedResult := tt.out.User

					assertPaymentsAreEqual(t, payment.Payment, tt.out.Payment)

					if payment.User.ID != expectedResult.ID {
						t.Logf("\t%s\tID should be equal to expected ID. Expected \"%d\" got \"%d\"%s",
							fail,
							expectedResult.ID,
							payment.User.ID,
							reset,
						)
						t.Fail()
					}

					if payment.User.Balance != expectedResult.Balance {
						t.Logf("\t%s\tStatus should be equal to expected Status. Expected \"%d\" got \"%d\"%s",
							fail,
							expectedResult.Balance,
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
	// Arrange
	// Setup the database
	testDB, err := db.OpenTestDatabase()
	if err != nil {
		t.Fatalf("%+v\n", err)
	}

	const amount = 20000
	tests := []struct {
		scenario string

		invoices []CreateInvoiceData
		filter   GetAllInvoicesData

		expectedNumberOfInvoices int
	}{
		{
			"adding 3 invoices, and getting first 50",
			[]CreateInvoiceData{
				CreateInvoiceData{
					Memo:      "1",
					AmountSat: 20001,
				},
				CreateInvoiceData{
					Memo:      "2",
					AmountSat: 20002,
				},
				CreateInvoiceData{
					Memo:      "3",
					AmountSat: 20003,
				},
			},
			GetAllInvoicesData{
				SkipFirst: 0,
				Count:     50,
			},
			3,
		},
		{
			"adding 3 invoices, and only gets two",
			[]CreateInvoiceData{
				CreateInvoiceData{
					Memo:      "1",
					AmountSat: 20001,
				},
				CreateInvoiceData{
					Memo:      "2",
					AmountSat: 20002,
				},
				CreateInvoiceData{
					Memo:      "3",
					AmountSat: 20003,
				},
			},
			GetAllInvoicesData{
				SkipFirst: 0,
				Count:     2,
			},
			2,
		},
		{
			"adding 3 invoices, and skips first 2",
			[]CreateInvoiceData{
				CreateInvoiceData{
					Memo:      "1",
					AmountSat: 20001,
				},
				CreateInvoiceData{
					Memo:      "2",
					AmountSat: 20002,
				},
				CreateInvoiceData{
					Memo:      "3",
					AmountSat: 20003,
				},
			},
			GetAllInvoicesData{
				SkipFirst: 2,
				Count:     50,
			},
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
				if err != nil || u == nil {
					t.Fatalf(
						"\t%s\tshould be able to CreateUser. Error:  %+v\n%s",
						fail, err, reset)
				}
				t.Logf("\t%s\tshould be able to CreateUser%s", succeed, reset)

				for i, invoice := range tt.invoices {
					// Create Mock LND client with preconfigured invoice response
					mockLNcli := lightningMockClient{
						InvoiceResponse: lnrpc.Invoice{
							Value: invoice.AmountSat,
							PaymentRequest: fmt.Sprintf("PayRequest_%d_%d",
								u.ID, i),
							RHash:     []byte("SomeRHash"),
							RPreimage: []byte("SomePreimage"),
							Memo:      invoice.Memo,
							Settled:   false,
						},
					}

					_, err := CreateInvoice(
						testDB, mockLNcli, invoice, u.ID)

					if err != nil {
						t.Fatalf(
							"\t%s\tshould be able to CreateInvoice. Error:  %+v\n%s",
							fail, err, reset)
					}
				}
				t.Logf("\t%s\tshould be able to CreateInvoices%s", succeed, reset)

				// Act

				invoices, err := GetAll(testDB, u.ID, tt.filter)
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
						if i < tt.filter.SkipFirst {
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

func assertPaymentsAreEqual(t *testing.T, payment, expectedResult Payment) {
	if payment.UserID != expectedResult.UserID {
		t.Logf("\t%s\tUserID should be equal to expected UserID. Expected \"%d\" got \"%d\"%s",
			fail, expectedResult.UserID, payment.UserID, reset)
		t.Fail()
	}

	if payment.AmountSat != expectedResult.AmountSat {
		t.Logf("\t%s\tAmountSat should be equal to expected AmountSat. Expected \"%d\" got \"%d\"%s",
			fail, expectedResult.AmountSat, payment.AmountSat, reset)
		t.Fail()
	}

	if payment.AmountMSat != expectedResult.AmountMSat {
		t.Logf("\t%s\tAmountMSat should be equal to expected AmountMSat. Expected \"%d\" got \"%d\"%s",
			fail, expectedResult.AmountMSat, payment.AmountMSat, reset)
		t.Fail()
	}

	if *payment.Preimage != *expectedResult.Preimage {
		t.Logf("\t%s\tPreimage should be equal to expected Preimage. Expected \"%s\" got \"%s\"%s",
			fail, *expectedResult.Preimage, *payment.Preimage, reset)
		t.Fail()
	}

	if payment.HashedPreimage != expectedResult.HashedPreimage {
		t.Logf("\t%s\tHashedPreimage should be equal to expected HashedPreimage. Expected \"%s\" got \"%s\"%s",
			fail, expectedResult.HashedPreimage, payment.HashedPreimage, reset)
		t.Fail()
	}

	if payment.Memo != expectedResult.Memo {
		t.Logf("\t%s\tMemo should be equal to expected Memo. Expected \"%s\" got \"%s\"%s",
			fail, expectedResult.Memo, payment.Memo, reset)
		t.Fail()
	}

	if payment.Status != expectedResult.Status {
		t.Logf("\t%s\tStatus should be equal to expected Status. Expected \"%s\" got \"%s\"%s",
			fail, expectedResult.Status, payment.Status, reset)
		t.Fail()
	}

	if payment.Direction != expectedResult.Direction {
		t.Logf("\t%s\tDirection should be equal to expected Direction. Expected \"%s\" got \"%s\"%s",
			fail, expectedResult.Direction, payment.Direction, reset)
		t.Fail()
	}
	if !t.Failed() {
		t.Logf("\t%s\tAll values should be equal to expected values%s", succeed, reset)
	}
}
