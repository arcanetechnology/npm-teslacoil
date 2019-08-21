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
	succeed = "\u2713"
	fail    = "\u2717"
)

type lightningMockClient struct {
	InvoiceResponse         lnrpc.Invoice
	SendPaymentSyncResponse lnrpc.SendResponse
	DecodePayReqRespons     lnrpc.PayReq
}

func (client lightningMockClient) AddInvoice(ctx context.Context, in *lnrpc.Invoice, opts ...grpc.CallOption) (*lnrpc.AddInvoiceResponse, error) {
	return &lnrpc.AddInvoiceResponse{}, nil
}

func (client lightningMockClient) LookupInvoice(ctx context.Context, in *lnrpc.PaymentHash, opts ...grpc.CallOption) (*lnrpc.Invoice, error) {
	return &client.InvoiceResponse, nil
}

func (client lightningMockClient) DecodePayReq(ctx context.Context, in *lnrpc.PayReqString, opts ...grpc.CallOption) (*lnrpc.PayReq, error) {
	return &client.DecodePayReqRespons, nil
}

func (client lightningMockClient) SendPaymentSync(ctx context.Context, in *lnrpc.SendRequest, opts ...grpc.CallOption) (*lnrpc.SendResponse, error) {
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
					t.Fatalf("\t%s\tShould be able to CreateInvoice %+v\n", fail, err)
				}
				t.Logf("\t%s\tShould be able to CreateInvoice", succeed)

				// Assertions
				t.Logf("\tTest: %d\tWhen checking invoice with amount %d and memo %s",
					i, tt.createInvoiceData.AmountSat, tt.createInvoiceData.Memo)
				{
					expectedResult := tt.out
					if payment.UserID != expectedResult.UserID {
						t.Logf("\t%s\tUserID should be equal to expected UserID. Expected \"%d\" got \"%d\"",
							fail, expectedResult.UserID, payment.UserID)
						t.Fail()
					}

					if payment.AmountSat != expectedResult.AmountSat {
						t.Logf("\t%s\tAmountSat should be equal to expected AmountSat. Expected \"%d\" got \"%d\"",
							fail, expectedResult.AmountSat, payment.AmountSat)
						t.Fail()
					}

					if payment.AmountMSat != expectedResult.AmountMSat {
						t.Logf("\t%s\tAmountMSat should be equal to expected AmountMSat. Expected \"%d\" got \"%d\"",
							fail, expectedResult.AmountMSat, payment.AmountMSat)
						t.Fail()
					}

					if *payment.Preimage != *expectedResult.Preimage {
						t.Logf("\t%s\tPreimage should be equal to expected Preimage. Expected \"%s\" got \"%s\"",
							fail, *expectedResult.Preimage, *payment.Preimage)
						t.Fail()
					}

					if payment.HashedPreimage != expectedResult.HashedPreimage {
						t.Logf("\t%s\tHashedPreimage should be equal to expected HashedPreimage. Expected \"%s\" got \"%s\"",
							fail, expectedResult.HashedPreimage, payment.HashedPreimage)
						t.Fail()
					}

					if payment.Memo != expectedResult.Memo {
						t.Logf("\t%s\tMemo should be equal to expected Memo. Expected \"%s\" got \"%s\"",
							fail, expectedResult.Memo, payment.Memo)
						t.Fail()
					}

					if payment.Status != expectedResult.Status {
						t.Logf("\t%s\tStatus should be equal to expected Status. Expected \"%s\" got \"%s\"",
							fail, expectedResult.Status, payment.Status)
						t.Fail()
					}

					if payment.Direction != expectedResult.Direction {
						t.Logf("\t%s\tDirection should be equal to expected Direction. Expected \"%s\" got \"%s\"",
							fail, expectedResult.Direction, payment.Direction)
						t.Fail()
					}
					if !t.Failed() {
						t.Logf("\t%s\tAll values should be equal to expected values", succeed)
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

func TestGetByID(t *testing.T) {
	// Prepare
	testDB, err := db.OpenTestDatabase()
	if err != nil {
		t.Fatalf("%+v\n", err)
	}

	user, err := users.Create(testDB,
		"test_userGetByID@example.com",
		"password",
	)
	if err != nil || user == nil {
		fmt.Println("User result was empty")
		t.Fatalf("%+v\n", err)
	}
	expectedResult := Payment{
		UserID:         user.ID,
		AmountSat:      20000,
		AmountMSat:     20000000,
		Preimage:       &samplePreimage,
		HashedPreimage: hex.EncodeToString([]byte("SomeRHash")),
		Memo:           "HelloWorld",
		Description:    "My personal description",
		Status:         Status("OPEN"),
		Direction:      Direction("INBOUND"),
	}

	tx := testDB.MustBegin()
	payment, err := insertPayment(tx, expectedResult)
	tx.Commit()
	// Act
	payment, err = GetByID(testDB, payment.ID, user.ID)
	if err != nil {
		t.Log("Could not retrieve payment")
		t.Fatalf("%+v\n", err)
	}

	// Assert
	if payment.UserID != expectedResult.UserID {
		t.Logf(
			"UserID incorrect. Expected \"%d\" got \"%d\"",
			expectedResult.UserID,
			payment.UserID,
		)
		t.Fail()
	}

	if payment.AmountSat != expectedResult.AmountSat {
		t.Logf(
			"Invoice amount incorrect. Expected \"%d\" got \"%d\"",
			expectedResult.AmountSat,
			payment.AmountSat,
		)
		t.Fail()
	}

	if payment.AmountMSat != expectedResult.AmountMSat {
		t.Logf(
			"Invoice milli amount incorrect. Expected \"%d\" got \"%d\"",
			expectedResult.AmountMSat,
			payment.AmountMSat,
		)
		t.Fail()
	}

	if *payment.Preimage != *expectedResult.Preimage {
		t.Logf(
			"Invoice preimage incorrect. Expected \"%s\" got \"%s\"",
			*expectedResult.Preimage,
			*payment.Preimage,
		)
		t.Fail()
	}

	if payment.HashedPreimage != expectedResult.HashedPreimage {
		t.Logf(
			"Invoice hashed preimage incorrect. Expected \"%s\" got \"%s\"",
			expectedResult.HashedPreimage,
			payment.HashedPreimage,
		)
		t.Fail()
	}

	if payment.Description != expectedResult.Description {
		t.Logf(
			"Invoice description incorrect. Expected \"%s\" got \"%s\"",
			expectedResult.Description,
			payment.Description,
		)
		t.Fail()
	}

	if payment.Memo != expectedResult.Memo {
		t.Logf(
			"Invoice description incorrect. Expected \"%s\" got \"%s\"",
			expectedResult.Memo,
			payment.Memo,
		)
		t.Fail()
	}

	if payment.Status != expectedResult.Status {
		t.Logf(
			"Invoice status incorrect. Expected \"%s\" got \"%s\"",
			expectedResult.Status,
			payment.Status,
		)
		t.Fail()
	}

	if payment.Status != expectedResult.Status {
		t.Logf(
			"Invoice status incorrect. Expected \"%s\" got \"%s\"",
			expectedResult.Status,
			payment.Status,
		)
		t.Fail()
	}

	if payment.Direction != expectedResult.Direction {
		t.Logf(
			"Invoice direction incorrect. Expected \"%s\" got \"%s\"",
			expectedResult.Direction,
			payment.Direction,
		)
		t.Fail()
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

	// Act
	tx := testDB.MustBegin()
	user, err := updateUserBalance(tx, u.ID, 100000)
	if err != nil {
		fmt.Printf("%+v\n", err)
		return
	}
	err = tx.Commit()
	if err != nil {
		fmt.Printf("%+v\n", err)
		return
	}

	// Assert
	expectedResult := UserResponse{
		ID:      u.ID,
		Balance: 100000, // Test user starts with 100k satoshi
	}

	if user.ID != expectedResult.ID {
		t.Logf(
			"UserResponse.ID incorrect. Expected \"%d\" got \"%d\"",
			expectedResult.ID,
			user.ID,
		)
		t.Fail()
	}

	if user.Balance != expectedResult.Balance {
		t.Logf(
			"UserResponse.ID incorrect. Expected \"%d\" got \"%d\"",
			expectedResult.Balance,
			user.Balance,
		)
		t.Fail()
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

	payInvoiceData := PayInvoiceData{
		PaymentRequest: "SomePaymentHash",
		Memo:           "HelloPayment",
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

	payment, err := PayInvoice(testDB, mockLNcli, payInvoiceData, u.ID)
	if err != nil {
		t.Fatalf("%+v\n", err)
	}

	// Expected test results are defined here.
	expectedResult := UserPaymentResponse{
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
	}

	// Asserting Payment results
	if payment.Payment.UserID != expectedResult.Payment.UserID {
		t.Logf(
			"UserID incorrect. Expected \"%d\" got \"%d\"",
			expectedResult.Payment.UserID,
			payment.Payment.UserID,
		)
		t.Fail()
	}

	if payment.Payment.AmountSat != expectedResult.Payment.AmountSat {
		t.Logf(
			"AmountSat incorrect. Expected \"%d\" got \"%d\"",
			expectedResult.Payment.AmountSat,
			payment.Payment.AmountSat,
		)
		t.Fail()
	}

	if payment.Payment.AmountMSat != expectedResult.Payment.AmountMSat {
		t.Logf(
			"AmountMSat incorrect. Expected \"%d\" got \"%d\"",
			expectedResult.Payment.AmountMSat,
			payment.Payment.AmountMSat,
		)
		t.Fail()
	}

	if *payment.Payment.Preimage != *expectedResult.Payment.Preimage {
		t.Logf(
			"Preimage incorrect. Expected \"%s\" got \"%s\"",
			*expectedResult.Payment.Preimage,
			*payment.Payment.Preimage,
		)
		t.Fail()
	}

	if payment.Payment.HashedPreimage != expectedResult.Payment.HashedPreimage {
		t.Logf(
			"HashedPreimage incorrect. Expected \"%s\" got \"%s\"",
			expectedResult.Payment.HashedPreimage,
			payment.Payment.HashedPreimage,
		)
		t.Fail()
	}

	if payment.Payment.Memo != expectedResult.Payment.Memo {
		t.Logf(
			"Memo incorrect. Expected \"%s\" got \"%s\"",
			expectedResult.Payment.Memo,
			payment.Payment.Memo,
		)
		t.Fail()
	}

	if payment.Payment.Status != expectedResult.Payment.Status {
		t.Logf(
			"Status incorrect. Expected \"%s\" got \"%s\"",
			expectedResult.Payment.Status,
			payment.Payment.Status,
		)
		t.Fail()
	}

	if payment.Payment.Direction != expectedResult.Payment.Direction {
		t.Logf(
			"Direction incorrect. Expected \"%s\" got \"%s\"",
			expectedResult.Payment.Direction,
			payment.Payment.Direction,
		)
		t.Fail()
	}

	// Asserting user result
	if payment.User.ID != expectedResult.User.ID {
		t.Logf(
			"UserResponse.ID incorrect. Expected \"%d\" got \"%d\"",
			expectedResult.User.ID,
			payment.User.ID,
		)
		t.Fail()
	}

	if payment.User.Balance != expectedResult.User.Balance {
		t.Logf(
			"UserResponse.ID incorrect. Expected \"%d\" got \"%d\"",
			expectedResult.User.Balance,
			payment.User.Balance,
		)
		t.Fail()
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

	triggerInvoice := lnrpc.Invoice{
		PaymentRequest: "SomePayRequest1",
		RHash:          []byte("SomeHash"),
		RPreimage:      []byte("SomePreimage"),
		Settled:        true,
	}

	CreateInvoice(testDB,
		lightningMockClient{
			InvoiceResponse: lnrpc.Invoice{
				Value:          20000,
				PaymentRequest: "SomePayRequest1",
				RHash:          []byte("SomeHash"),
				RPreimage:      []byte("SomePreimage"),
				Settled:        true,
			},
		},
		CreateInvoiceData{
			Memo:        "HelloWorld",
			Description: "My description",
			AmountSat:   20000,
		}, u.ID)

	// Act
	payment, err := UpdateInvoiceStatus(triggerInvoice, testDB)
	if err != nil {
		t.Fatalf("%+v\n", err)
	}

	// Assert
	// Expected test results are defined here.
	expectedResult := UserPaymentResponse{
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
	}

	if payment.Payment.UserID != expectedResult.Payment.UserID {
		t.Logf(
			"UserID incorrect. Expected \"%d\" got \"%d\"",
			expectedResult.Payment.UserID,
			payment.Payment.UserID,
		)
		t.Fail()
	}

	if payment.Payment.AmountSat != expectedResult.Payment.AmountSat {
		t.Logf(
			"AmountSat incorrect. Expected \"%d\" got \"%d\"",
			expectedResult.Payment.AmountSat,
			payment.Payment.AmountSat,
		)
		t.Fail()
	}

	if payment.Payment.AmountMSat != expectedResult.Payment.AmountMSat {
		t.Logf(
			"AmountMSat incorrect. Expected \"%d\" got \"%d\"",
			expectedResult.Payment.AmountMSat,
			payment.Payment.AmountMSat,
		)
		t.Fail()
	}

	if *payment.Payment.Preimage != *expectedResult.Payment.Preimage {
		t.Logf(
			"Preimage incorrect. Expected \"%s\" got \"%s\"",
			*expectedResult.Payment.Preimage,
			*payment.Payment.Preimage,
		)
		t.Fail()
	}

	if payment.Payment.HashedPreimage != expectedResult.Payment.HashedPreimage {
		t.Logf(
			"HashedPreimage incorrect. Expected \"%s\" got \"%s\"",
			expectedResult.Payment.HashedPreimage,
			payment.Payment.HashedPreimage,
		)
		t.Fail()
	}

	if payment.Payment.Memo != expectedResult.Payment.Memo {
		t.Logf(
			"Memo incorrect. Expected \"%s\" got \"%s\"",
			expectedResult.Payment.Memo,
			payment.Payment.Memo,
		)
		t.Fail()
	}

	if payment.Payment.Status != expectedResult.Payment.Status {
		t.Logf(
			"Status incorrect. Expected \"%s\" got \"%s\"",
			expectedResult.Payment.Status,
			payment.Payment.Status,
		)
		t.Fail()
	}

	if payment.Payment.Direction != expectedResult.Payment.Direction {
		t.Logf(
			"Direction incorrect. Expected \"%s\" got \"%s\"",
			expectedResult.Payment.Direction,
			payment.Payment.Direction,
		)
		t.Fail()
	}

	if payment.Payment.Memo != expectedResult.Payment.Memo {
		t.Logf(
			"Memo incorrect. Expected \"%s\" got \"%s\"",
			expectedResult.Payment.Memo,
			payment.Payment.Memo,
		)
		t.Fail()
	}

	// Asserting user result
	if payment.User.ID != expectedResult.User.ID {
		t.Logf(
			"UserResponse.ID incorrect. Expected \"%d\" got \"%d\"",
			expectedResult.User.ID,
			payment.User.ID,
		)
		t.Fail()
	}

	if payment.User.Balance != expectedResult.User.Balance {
		t.Logf(
			"UserResponse.ID incorrect. Expected \"%d\" got \"%d\"",
			expectedResult.User.Balance,
			payment.User.Balance,
		)
		t.Fail()
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
	invoiceData := CreateInvoiceData{
		Memo:      "HelloWorld",
		AmountSat: 20000,
	}

	var userIDs [3]uint
	// Create 2 more users and add 3 more invoices to each of them
	for i := 0; i < 3; i++ {
		u, err := users.Create(testDB,
			fmt.Sprintf("test_user%d@example.com", i),
			"password",
		)
		if err != nil || u == nil {
			fmt.Println("User result was empty")
			t.Fatalf("%+v\n", err)
		}
		userIDs[i] = u.ID
		for invoiceIndex := 1; invoiceIndex <= 3; invoiceIndex++ {
			// Create Mock LND client with preconfigured invoice response
			mockLNcli := lightningMockClient{
				InvoiceResponse: lnrpc.Invoice{
					Value:          20000,
					PaymentRequest: "PayRequest_" + string(u.ID) + "_" + string(invoiceIndex),
					RHash:          []byte("SomeRHash"),
					RPreimage:      []byte("SomePreimage"),
					Settled:        false,
				},
			}

			_, err := CreateInvoice(testDB, mockLNcli, invoiceData, u.ID)
			if err != nil {
				t.Fatalf("%+v\n", err)
			}
		}
	}

	for i := 0; i < 3; i++ {
		userID := userIDs[i]
		// Act
		invoices, err := GetAll(testDB, userID)
		if err != nil {
			t.Fatalf("%+v\n", err)
		}

		// Assert
		expectedNumberOfInvoices := 3
		numberOfInvoices := len(invoices)

		if expectedNumberOfInvoices != numberOfInvoices {
			t.Logf(
				"Unexpected invoice count for user %d. Expected \"%d\" got \"%d\"",
				2,
				expectedNumberOfInvoices,
				numberOfInvoices,
			)
			t.Fail()
		}

		for _, invoice := range invoices {
			if invoice.UserID != userID {
				t.Logf(
					"user ID was incorrect. Expected \"%d\" got \"%d\"",
					2,
					invoice.UserID,
				)
				t.Fail()
			}
		}
	}

	// Fail tests after all assertions that will not interfere with eachother
	// for improved test result readability.
	if t.Failed() {
		t.FailNow()
	}
}
