package users

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"testing"

	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"
)

const (
	succeed = "\u2713"
	fail    = "\u2717"
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

	const email = "test_userCanCreate@example.com"
	tests := []struct {
		email string
		out   UserResponse
	}{
		{
			email,
			UserResponse{
				Email:   email,
				Balance: 0,
			},
		},
	}

	t.Log("testing can create user")
	{
		for i, tt := range tests {
			t.Logf("\ttest %d\twhen creating user with email %s", i, tt.email)

			{
				user, err := Create(testDB,
					email,
					"password",
				)
				if user == nil {
					t.Fatalf(
						"\t%s\tshould be able to Create user. User response was nil",
						fail)
				}
				if err != nil {
					t.Fatalf(
						"\t%s\tshould be able to Create user. Error: %v",
						fail, err)
				}
				t.Logf("\t%s\tshould be able to Create user", succeed)

				{
					expectedResult := tt.out

					if user.Email != expectedResult.Email {
						t.Logf(
							"\t%s\tEmail should be equal to expected Email. Expected \"%s\" got \"%s\"",
							fail,
							expectedResult.Email,
							user.Email,
						)
						t.Fail()
					}
					if user.Balance != expectedResult.Balance {
						t.Logf(
							"\t%s\tBalance should be equal to expected Balance. Expected: %d, got: %d",
							fail,
							expectedResult.Balance,
							user.Balance,
						)
						t.Fail()
					}
					if !t.Failed() {
						t.Logf("\t%s\tall values should be equal to expected values", succeed)
					}
				}
			}
		}
	}
}

func TestCanGetUserByEmail(t *testing.T) {
	testDB, err := db.OpenTestDatabase()
	if err != nil {
		t.Fatalf("%+v\n", err)
	}

	const email = "test_userGetByEmail@example.com"
	tests := []struct {
		user User
		out  UserResponse
	}{
		{
			User{
				Email:          email,
				HashedPassword: []byte("SomePassword"),
			},
			UserResponse{
				Email:   email,
				Balance: 0,
			},
		},
	}

	t.Log("testing can get user by email")
	{
		for i, tt := range tests {
			t.Logf("\ttest %d\twhen getting user with email %s", i, tt.user.Email)

			{
				user, err := insertUser(testDB, User{
					Email:          tt.user.Email,
					HashedPassword: tt.user.HashedPassword,
				})
				if user == nil {
					t.Log("User result was empty")
				}
				if err != nil {
					t.Logf("%+v\n", err)
				}

				user, err = GetByEmail(testDB, email)
				if user == nil {
					t.Fatalf(
						"\t%s\tshould be able to get user. User response was nil",
						fail)
				}
				if err != nil {
					t.Fatalf("\t%s\tshould be able to get user. Error: %v", fail, err)
				}
				t.Logf("\t%s\tshould be able to get user", succeed)

				{
					expectedResult := tt.out

					if user.Email != expectedResult.Email {
						t.Logf(
							"\t%s\tEmail should be equal to expected Email. Expected \"%s\" got \"%s\"",
							fail,
							expectedResult.Email,
							user.Email,
						)
						t.Fail()
					}
					if user.Balance != expectedResult.Balance {
						t.Logf(
							"\t%s\tBalance should be equal to expected Balance. Expected: %d, got: %d",
							fail,
							expectedResult.Balance,
							user.Balance,
						)
						t.Fail()
					}
					if !t.Failed() {
						t.Logf("\t%s\tall values should be equal to expected values", succeed)
					}
				}
			}
		}
	}
}

func TestCanGetUserByCredentials(t *testing.T) {

	testDB, err := db.OpenTestDatabase()
	if err != nil {
		t.Fatalf("%+v\n", err)
	}

	const email = "test_userByCredentials@example.com"
	tests := []struct {
		email    string
		password string
		out      UserResponse
	}{
		{
			email,
			"password",
			UserResponse{
				Email:   email,
				Balance: 0,
			},
		},
	}

	t.Log("testing can get user by email")
	{
		for i, tt := range tests {
			t.Logf("\ttest %d\twhen getting user with email %s", i, tt.email)

			{
				user, err := Create(testDB, tt.email, tt.password)
				if user == nil {
					t.Fatalf("\t%s\tshould be able to Create user. User response was nil", fail)
				}
				if err != nil {
					t.Fatalf("\t%s\tshould be able to Create user. Error: %v", fail, err)
				}
				t.Logf("\t%s\tshould be able to Create user", succeed)

				user, err = GetByCredentials(testDB, tt.email, tt.password)
				if user == nil {
					t.Fatalf("\t%s\tshould be able to get user by credentials. User response was nil", fail)
				}
				if err != nil {
					t.Fatalf("\t%s\tshould be able to get user by credentials. Error: %v", fail, err)
				}
				t.Logf("\t%s\tshould be able to get user by credentials", succeed)

				{
					expectedResult := tt.out

					if user.Email != expectedResult.Email {
						t.Logf(
							"\t%s\tEmail should be equal to expected Email. Expected \"%s\" got \"%s\"",
							fail,
							expectedResult.Email,
							user.Email,
						)
						t.Fail()
					}
					if user.Balance != expectedResult.Balance {
						t.Logf(
							"\t%s\tBalance should be equal to expected Balance. Expected: %d, got: %d",
							fail,
							expectedResult.Balance,
							user.Balance,
						)
						t.Fail()
					}
					if !t.Failed() {
						t.Logf("\t%s\tall values should be equal to expected values", succeed)
					}
				}
			}
		}
	}
}
