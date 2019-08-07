package users

import (
	"flag"
	"fmt"
	"os"
	"path"
	"runtime"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"gitlab.com/arcanecrypto/lpp/internal/platform/db"
)

func createTestDatabase(testDB *sqlx.DB) error {
	_, filename, _, ok := runtime.Caller(1)
	if ok == false {
		return nil
	}
	migrationFiles := path.Join("file://", path.Dir(filename), "../platform/migrations")
	err := db.DropDatabase(migrationFiles, testDB)
	if err != nil {
		return errors.Wrapf(err,
			"Cannot connect to database %s with user %s",
			os.Getenv("DATABASE_TEST_NAME"),
			os.Getenv("DATABASE_TEST_USER"),
		)
	}
	err = db.MigrateUp(migrationFiles, testDB)
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

	flag.Parse()
	result := m.Run()
	os.Exit(result)
}

func TestCanCreateUser(t *testing.T) {

	testDB, err := db.OpenTestDatabase()
	if err != nil {
		t.Fatal(err)
	}
	user, err := Create(testDB,
		"test_user@example.com",
		"password",
	)
	if err != nil {
		t.Fatal(err)
	}
	if user == nil {
		t.Fatal("user result was empty")
	}

	expectedResult := UserResponse{
		Email:   "test_user@example.com",
		Balance: 0,
		ID:      1,
	}
	if user.Email != expectedResult.Email {
		t.Fatalf(
			"Email incorrect: %s does not equal %s",
			expectedResult.Email,
			user.Email,
		)
	}
	if user.Balance != expectedResult.Balance {
		t.Fatalf(
			"Incorrect Balance. Expected: %d, was: %d",
			expectedResult.Balance,
			user.Balance,
		)
	}
}

func TestCanGetUserByEmail(t *testing.T) {

	testDB, err := db.OpenTestDatabase()
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	user, err := GetByEmail(testDB, "test_user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if user == nil {
		t.Fatal("user result was empty")
	}

	expectedResult := UserResponse{
		Email:   "test_user@example.com",
		Balance: 0,
		ID:      1,
	}
	if user.Email != expectedResult.Email {
		t.Fatalf(
			"Email incorrect: %s does not equal %s",
			expectedResult.Email,
			user.Email,
		)
	}
	if user.Balance != expectedResult.Balance {
		t.Fatalf(
			"Incorrect Balance. Expected: %d, was: %d",
			expectedResult.Balance,
			user.Balance,
		)
	}
}

func TestCanGetUserByCredentials(t *testing.T) {

	testDB, err := db.OpenTestDatabase()
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	user, err := GetByCredentials(testDB, "test_user@example.com", "password")
	if err != nil {
		t.Fatal(err)
	}
	if user == nil {
		t.Fatal("user result was empty")
	}
	expectedResult := UserResponse{
		Email:   "test_user@example.com",
		Balance: 0,
		ID:      1,
	}
	if user.Email != expectedResult.Email {
		t.Fatalf(
			"Email incorrect: %s does not equal %s",
			expectedResult.Email,
			user.Email,
		)
	}
	if user.Balance != expectedResult.Balance {
		t.Fatalf(
			"Incorrect Balance. Expected: %d, was: %d",
			expectedResult.Balance,
			user.Balance,
		)
	}
}

func TestCanUpdateUserBalance(t *testing.T) {

	testDB, err := db.OpenTestDatabase()
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	user, err := UpdateUserBalance(testDB, 1, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if user == nil {
		t.Fatal("user result was empty")
	}
	expectedResult := UserResponse{
		Email:   "test_user@example.com",
		Balance: 1000,
		ID:      1,
	}
	if user.Email != expectedResult.Email {
		t.Fatalf(
			"Email incorrect: %s does not equal %s",
			expectedResult.Email,
			user.Email,
		)
	}
	if user.Balance != expectedResult.Balance {
		t.Fatalf(
			"Incorrect Balance. Expected: %d, was: %d",
			expectedResult.Balance,
			user.Balance,
		)
	}
}
