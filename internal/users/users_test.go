package users

import (
	"flag"
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"gitlab.com/arcanecrypto/lpp/internal/platform/db"
)

var migrationsPath = path.Join(
	os.Getenv("GOPATH"),
	"/src/gitlab.com/arcanecrypto/lpp/internal/platform/migrations")

func createTestDatabase(testDB *sqlx.DB) error {
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

func TestMain(m *testing.M) {
	println("Configuring user test database")
	lpp.InitLogRotator("/home/bo/gocode/src/gitlab.com/arcanecrypto/lpp/logs/lpp.log", 10, 3)

	testDB, err := db.OpenTestDatabase()
	if err != nil {
		fmt.Printf("%+v\n", err)
		return
	}
	fmt.Println(testDB.Ping())

	err = createTestDatabase(testDB)
	if err != nil {
		fmt.Printf("%+v\n", err)
		return
	}

	flag.Parse()
	result := m.Run()
	os.Exit(result)
}

func TestCanCreateUser(t *testing.T) {

	testDB, err := db.OpenTestDatabase()
	if err != nil {
		t.Fatalf("%+v\n", err)
	}
	user, err := Create(testDB,
		"test_user@example.com",
		"password",
	)
	if user == nil {
		t.Log("User result was empty")
	}
	if err != nil {
		t.Logf("%+v\n", err)
	}
	if err != nil || user == nil {
		t.FailNow()
	}

	expectedResult := UserResponse{
		Email:   "test_user@example.com",
		Balance: 0,
		ID:      1,
	}
	if user.Email != expectedResult.Email {
		t.Fatalf(
			"Email incorrect. Expected \"%s\" got \"%s\"",
			expectedResult.Email,
			user.Email,
		)
	}
	if user.Balance != expectedResult.Balance {
		t.Fatalf(
			"Incorrect Balance. Expected: %d, got: %d",
			expectedResult.Balance,
			user.Balance,
		)
	}
}

func TestCanGetUserByEmail(t *testing.T) {

	testDB, err := db.OpenTestDatabase()
	if err != nil {
		t.Fatalf("%+v\n", err)
	}

	user, err := GetByEmail(testDB, "test_user@example.com")
	if user == nil {
		t.Log("User result was empty")
	}
	if err != nil {
		t.Logf("%+v\n", err)
	}
	if err != nil || user == nil {
		t.FailNow()
	}

	expectedResult := UserResponse{
		Email:   "test_user@example.com",
		Balance: 0,
		ID:      1,
	}
	if user.Email != expectedResult.Email {
		t.Fatalf(
			"Email incorrect. Expected \"%s\" got \"%s\"",
			expectedResult.Email,
			user.Email,
		)
	}
	if user.Balance != expectedResult.Balance {
		t.Fatalf(
			"Incorrect Balance. Expected: %d, got: %d",
			expectedResult.Balance,
			user.Balance,
		)
	}
}

func TestCanGetUserByCredentials(t *testing.T) {

	testDB, err := db.OpenTestDatabase()
	if err != nil {
		t.Fatalf("%+v\n", err)
	}

	// Get the user and fail if error or no user was returned
	user, err := GetByCredentials(testDB, "test_user@example.com", "password")
	if user == nil {
		t.Log("User result was empty")
	}
	if err != nil {
		t.Logf("%+v\n", err)
	}
	if err != nil || user == nil {
		t.FailNow()
	}

	// Check if the GetByCredentials returned the expected user object
	expectedResult := UserResponse{
		Email:   "test_user@example.com",
		Balance: 0,
		ID:      1,
	}
	if user.Email != expectedResult.Email {
		t.Fatalf(
			"Email incorrect. Expected \"%s\" got \"%s\"",
			expectedResult.Email,
			user.Email,
		)
	}
	if user.Balance != expectedResult.Balance {
		t.Fatalf(
			"Incorrect Balance. Expected: %d, got: %d",
			expectedResult.Balance,
			user.Balance,
		)
	}
}

func TestCanUpdateUserBalance(t *testing.T) {

	testDB, err := db.OpenTestDatabase()
	if err != nil {
		t.Fatalf("%+v\n", err)
	}

	// Update user
	user, err := UpdateUserBalance(testDB, 1, 1000)
	if user == nil {
		t.Log("User result was empty")
	}
	if err != nil {
		t.Logf("%+v\n", err)
	}
	if err != nil || user == nil {
		t.FailNow()
	}

	// Check that user balance was updated correctly.
	expectedResult := UserResponse{
		Email:   "test_user@example.com",
		Balance: 1000,
		ID:      1,
	}
	if user.Email != expectedResult.Email {
		t.Fatalf(
			"Email incorrect. Expected \"%s\" got \"%s\"",
			expectedResult.Email,
			user.Email,
		)
	}
	if user.Balance != expectedResult.Balance {
		t.Fatalf(
			"Incorrect Balance. Expected: %d, got: %d",
			expectedResult.Balance,
			user.Balance,
		)
	}
}
