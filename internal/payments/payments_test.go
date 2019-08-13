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
	InvoiceResponse lnrpc.Invoice
}

func (client lightningMockClient) AddInvoice(ctx context.Context, in *lnrpc.Invoice, opts ...grpc.CallOption) (*lnrpc.AddInvoiceResponse, error) {
	return &lnrpc.AddInvoiceResponse{}, nil
}

func (client lightningMockClient) LookupInvoice(ctx context.Context, in *lnrpc.PaymentHash, opts ...grpc.CallOption) (*lnrpc.Invoice, error) {
	return &client.InvoiceResponse, nil
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

	// Expected test results are defined here.
	expectedResult := Payment{
		ID:             1,
		UserID:         1,
		AmountSat:      20000,
		AmountMSat:     20000000,
		Preimage:       hex.EncodeToString([]byte("SomePreimage")),
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

	if payment.Preimage != expectedResult.Preimage {
		t.Logf(
			"Invoice preimage incorrect. Expected \"%s\" got \"%s\"",
			expectedResult.Preimage,
			payment.Preimage,
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
