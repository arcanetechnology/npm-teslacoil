package payments

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path"
	"runtime"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/pkg/errors"
	"google.golang.org/grpc"

	"gitlab.com/arcanecrypto/lpp/internal/platform/db"
	// "gitlab.com/arcanecrypto/lpp/internal/platform/ln"
	"gitlab.com/arcanecrypto/lpp/internal/users"
)

func createTestDatabase(testDB *sqlx.DB) error {
	_, filename, _, ok := runtime.Caller(1)
	if ok == false {
		return errors.New("Could not find path to migrations files.")
	}

	migrationsPath := path.Join("file://", path.Dir(filename), "../platform/migrations")
	err := db.DropDatabase(migrationsPath, testDB)
	if err != nil {
		return errors.Wrapf(err,
			"Cannot connect to database %s with user %s",
			os.Getenv("DATABASE_TEST_NAME"),
			os.Getenv("DATABASE_TEST_USER"),
		)
	}
	err = db.MigrateUp(migrationsPath, testDB)
	if err != nil {
		return errors.Wrapf(err,
			"Cannot connect to database %s with user %s",
			os.Getenv("DATABASE_TEST_NAME"),
			os.Getenv("DATABASE_TEST_USER"),
		)
	}
	return nil
}

var testDB *sqlx.DB

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
	println("Configuring payments test database")

	testDB, err := db.OpenTestDatabase()
	if err != nil {
		fmt.Printf("%+v\n", err)
		return
	}

	err = createTestDatabase(testDB)
	if err != nil {
		fmt.Printf("%+v\n", err)
		return
	}

	// Create a user to bind payments to
	user, err := users.Create(testDB,
		"test_user@example.com",
		"password",
	)
	if err != nil || user == nil {
		fmt.Println("User result was empty")
		fmt.Printf("%+v\n", err)
		return
	}

	flag.Parse()
	result := m.Run()
	os.Exit(result)
}

func TestCreateInvoice(t *testing.T) {

	// Setup the database
	testDB, err := db.OpenTestDatabase()
	if err != nil {
		t.Fatalf("%+v\n", err)
	}
	incoiceData := CreateInvoiceData{
		Memo:      "HelloWorld",
		AmountSat: 20000,
	}
	userId := uint(1)

	// Create Mock LND client with preconfigured invoice response
	mockLNcli := lightningMockClient{
		InvoiceResponse: lnrpc.Invoice{
			Value:     20000,
			RHash:     []byte("SomeRHash"),
			RPreimage: []byte("SomePreimage"),
			Settled:   true,
		},
	}

	payment, err := CreateInvoice(testDB, mockLNcli, incoiceData, userId)
	if err != nil {
		t.Fatalf("%+v\n", err)
	}

	preImg := hex.EncodeToString([]byte("SomePreimage"))
	// Expected test results are defined here.
	expectedResult := Payment{
		ID:             1,
		UserID:         1,
		AmountSat:      20000,
		AmountMSat:     20000000,
		Preimage:       &preImg,
		HashedPreimage: hex.EncodeToString([]byte("SomeRHash")),
		Description:    "HelloWorld",
		Status:         Status("OPEN"),
		Direction:      Direction("INBOUND"),
	}

	// Assertions
	if payment.ID != expectedResult.ID {
		t.Logf(
			"ID incorrect. Expected \"%d\" got \"%d\"",
			expectedResult.ID,
			payment.ID,
		)
		t.Fail()
	}

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

	// Act
	tx := testDB.MustBegin()
	user, err := updateUserBalance(tx, 1, 100000)
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
		ID:      1,
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

}

func TestPayInvoice(t *testing.T) {

	// Setup the database
	testDB, err := db.OpenTestDatabase()
	if err != nil {
		t.Fatalf("%+v\n", err)
	}
	payInvoiceData := PayInvoiceData{
		PaymentRequest: "SomePaymentHash",
		Description:    "HelloPayment",
		AmountSat:      5000,
	}
	userId := uint(1)

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

	payment, err := PayInvoice(testDB, mockLNcli, payInvoiceData, userId)
	if err != nil {
		t.Fatalf("%+v\n", err)
	}

	preImg := hex.EncodeToString([]byte("SomePreimage"))
	// Expected test results are defined here.
	expectedResult := UserPaymentResponse{
		Payment: Payment{
			ID:             2,
			UserID:         1,
			AmountSat:      5000,
			AmountMSat:     5000000,
			Preimage:       &preImg,
			HashedPreimage: "SomeHash",
			Description:    "HelloPayment",
			Status:         Status("SUCCEEDED"),
			Direction:      Direction("OUTBOUND"),
		},
		User: UserResponse{
			ID:      1,
			Balance: 95000, // Test user starts with 100k satoshi
		},
	}

	// Asserting Payment results
	if payment.Payment.ID != expectedResult.Payment.ID {
		t.Logf(
			"ID incorrect. Expected \"%d\" got \"%d\"",
			expectedResult.Payment.ID,
			payment.Payment.ID,
		)
		t.Fail()
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

	if payment.Payment.Description != expectedResult.Payment.Description {
		t.Logf(
			"Description incorrect. Expected \"%s\" got \"%s\"",
			expectedResult.Payment.Description,
			payment.Payment.Description,
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
