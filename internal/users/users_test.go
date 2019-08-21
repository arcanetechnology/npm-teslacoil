package users

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"testing"

	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"
)

var (
	samplePreimage = hex.EncodeToString([]byte("SomePreimage"))
)

func TestMain(m *testing.M) {
	println("Configuring user test database")

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

func TestCanCreateUser(t *testing.T) {
	testDB, err := db.OpenTestDatabase()
	if err != nil {
		t.Fatalf("%+v\n", err)
	}
	user, err := Create(testDB,
		"test_userCanCreate@example.com",
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
		Email:   "test_userCanCreate@example.com",
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
	user, err := Create(testDB,
		"test_userGetByEmail@example.com",
		"password",
	)
	if user == nil {
		t.Log("User result was empty")
	}
	if err != nil {
		t.Logf("%+v\n", err)
	}

	user, err = GetByEmail(testDB, "test_userGetByEmail@example.com")
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
		Email:   "test_userGetByEmail@example.com",
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
	user, err := Create(testDB,
		"test_userByCredentials@example.com",
		"password",
	)
	if user == nil {
		t.Log("User result was empty")
	}
	if err != nil {
		t.Logf("%+v\n", err)
	}

	// Get the user and fail if error or no user was returned
	user, err = GetByCredentials(testDB, "test_userByCredentials@example.com", "password")
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
		Email:   "test_userByCredentials@example.com",
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
