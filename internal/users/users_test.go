package users

import (
	"encoding/hex"
	"flag"
	"os"
	"testing"

	"github.com/sirupsen/logrus"

	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"
)

const (
	succeed = "\u001b[32m\u2713"
	fail    = "\u001b[31m\u2717"
	reset   = "\u001b[0m"
)

var (
	samplePreimage = hex.EncodeToString([]byte("SomePreimage"))
	databaseConfig = db.DatabaseConfig{
		User:     "lpp_test",
		Password: "password",
		Host:     "localhost",
		Port:     5434,
		Name:     "lpp_users",
	}
)

func TestMain(m *testing.M) {
	build.SetLogLevel(logrus.ErrorLevel)

	log.Info("Configuring user test database")
	testDB, err := db.OpenDatabase(databaseConfig)
	if err != nil {
		log.Fatalf("Could not open test database: %+v\n", err)
	}

	if err = db.TeardownTestDB(testDB, databaseConfig); err != nil {
		log.Fatalf("Could not tear down test database: %v", err)
	}

	if err = db.CreateTestDatabase(testDB, databaseConfig); err != nil {
		log.Fatalf("Could not create test database: %v", err)
		return
	}

	flag.Parse()
	result := m.Run()

	os.Exit(result)
}

func TestCanCreateUser(t *testing.T) {
	t.Parallel()
	testDB, err := db.OpenDatabase(databaseConfig)
	if err != nil {
		t.Fatalf("%+v\n", err)
	}

	const email = "test_userCanCreate@example.com"
	tests := []struct {
		email          string
		expectedResult UserResponse
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
					tt.email,
					"password",
				)
				if err != nil {
					t.Fatalf(
						"\t%s\tshould be able to Create user. Error: %v%s",
						fail, err, reset)
				}
				t.Logf("\t%s\tshould be able to Create user%s", succeed, reset)

				{
					expectedResult := tt.expectedResult

					if user.Email != expectedResult.Email {
						t.Logf(
							"\t%s\tEmail should be equal to expected Email. Expected \"%s\" got \"%s\"%s",
							fail,
							expectedResult.Email,
							user.Email,
							reset,
						)
						t.Fail()
					}
					if user.Balance != expectedResult.Balance {
						t.Logf(
							"\t%s\tBalance should be equal to expected Balance. Expected: %d, got: %d%s",
							fail,
							expectedResult.Balance,
							user.Balance,
							reset,
						)
						t.Fail()
					}
					if !t.Failed() {
						t.Logf("\t%s\tall values should be equal to expected values%s", succeed, reset)
					}
				}
			}
		}
	}
}

func TestCanGetUserByEmail(t *testing.T) {
	t.Parallel()
	testDB, err := db.OpenDatabase(databaseConfig)
	if err != nil {
		t.Fatalf("%+v\n", err)
	}

	const email = "test_userGetByEmail@example.com"
	tests := []struct {
		user           User
		expectedResult UserResponse
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
				tx := testDB.MustBegin()
				user, err := insertUser(tx, User{
					Email:          tt.user.Email,
					HashedPassword: tt.user.HashedPassword,
				})
				err = tx.Commit()
				if err != nil {
					t.Logf("%+v\n", err)
				}

				user, err = GetByEmail(testDB, email)
				if err != nil {
					t.Fatalf("\t%s\tshould be able to get user. Error: %v%s", fail, err, reset)
				}
				t.Logf("\t%s\tshould be able to get user%s", succeed, reset)

				{
					expectedResult := tt.expectedResult

					if user.Email != expectedResult.Email {
						t.Logf(
							"\t%s\tEmail should be equal to expected Email. Expected \"%s\" got \"%s\"%s",
							fail,
							expectedResult.Email,
							user.Email,
							reset,
						)
						t.Fail()
					}
					if user.Balance != expectedResult.Balance {
						t.Logf(
							"\t%s\tBalance should be equal to expected Balance. Expected: %d, got: %d%s",
							fail,
							expectedResult.Balance,
							user.Balance,
							reset,
						)
						t.Fail()
					}
					if !t.Failed() {
						t.Logf("\t%s\tall values should be equal to expected values%s", succeed, reset)
					}
				}
			}
		}
	}
}

func TestCanGetUserByCredentials(t *testing.T) {
	t.Parallel()

	testDB, err := db.OpenDatabase(databaseConfig)
	if err != nil {
		t.Fatalf("%+v\n", err)
	}

	const email = "test_userByCredentials@example.com"
	tests := []struct {
		email          string
		password       string
		expectedResult UserResponse
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
				if err != nil {
					t.Fatalf("\t%s\tshould be able to Create user. Error: %v%s", fail, err, reset)
				}
				t.Logf("\t%s\tshould be able to Create user%s", succeed, reset)

				user, err = GetByCredentials(testDB, tt.email, tt.password)
				if err != nil {
					t.Fatalf("\t%s\tshould be able to get user by credentials. Error: %v%s", fail, err, reset)
				}
				t.Logf("\t%s\tshould be able to get user by credentials%s", succeed, reset)

				{
					expectedResult := tt.expectedResult

					if user.Email != expectedResult.Email {
						t.Logf(
							"\t%s\tEmail should be equal to expected Email. Expected \"%s\" got \"%s\"%s",
							fail,
							expectedResult.Email,
							user.Email,
							reset,
						)
						t.Fail()
					}
					if user.Balance != expectedResult.Balance {
						t.Logf(
							"\t%s\tBalance should be equal to expected Balance. Expected: %d, got: %d%s",
							fail,
							expectedResult.Balance,
							user.Balance,
							reset,
						)
						t.Fail()
					}
					if !t.Failed() {
						t.Logf("\t%s\tall values should be equal to expected values%s", succeed, reset)
					}
				}
			}
		}
	}
}

func TestCanGetUserByID(t *testing.T) {
	t.Parallel()
	testDB, err := db.OpenDatabase(databaseConfig)
	if err != nil {
		t.Fatalf("%+v\n", err)
	}

	const email = "test_userCanGetByID@example.com"
	tests := []struct {
		user           User
		expectedResult UserResponse
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

	t.Log("testing can get user by ID")
	{
		for i, tt := range tests {
			t.Logf("\ttest %d\twhen getting user with email %s", i, tt.user.Email)

			{
				tx := testDB.MustBegin()
				u, err := insertUser(tx, User{
					Email:          tt.user.Email,
					HashedPassword: tt.user.HashedPassword,
				})
				err = tx.Commit()
				if err != nil {
					t.Logf("%+v\n", err)
				}

				user, err := GetByID(testDB, u.ID)
				if err != nil {
					t.Fatalf("\t%s\tshould be able to get user. Error: %v%s", fail, err, reset)
				}
				t.Logf("\t%s\tshould be able to get user%s", succeed, reset)

				{
					expectedResult := tt.expectedResult

					if user.Email != expectedResult.Email {
						t.Logf(
							"\t%s\tEmail should be equal to expected Email. Expected \"%s\" got \"%s\"%s",
							fail,
							expectedResult.Email,
							user.Email,
							reset,
						)
						t.Fail()
					}
					if user.Balance != expectedResult.Balance {
						t.Logf(
							"\t%s\tBalance should be equal to expected Balance. Expected: %d, got: %d%s",
							fail,
							expectedResult.Balance,
							user.Balance,
							reset,
						)
						t.Fail()
					}
					if !t.Failed() {
						t.Logf("\t%s\tall values should be equal to expected values%s", succeed, reset)
					}
				}
			}
		}
	}
}

func TestDecreaseBalance(t *testing.T) {
	t.Parallel()
	// Arrange
	testDB, err := db.OpenDatabase(databaseConfig)
	if err != nil {
		t.Fatalf("%+v\n", err)
	}
	u, err := Create(testDB,
		"test_userDecreaseBalance@example.com",
		"password",
	)
	// Give initial balance of 100 000
	tx := testDB.MustBegin()
	u, err = IncreaseBalance(tx, ChangeBalance{
		UserID:    u.ID,
		AmountSat: 100000,
	})
	if err != nil {
		t.Fatalf(
			"\t%s\tShould be able to give user iniital balance by using IncreaseBalance. Error: %+v\n%s",
			fail, err, reset)
	}
	tx.Commit()
	t.Logf("\t%s\tShould be able to give user iniital balance by using IncreaseBalance%s", succeed, reset)

	tests := []struct {
		dec ChangeBalance

		expectedResult User
	}{
		{
			ChangeBalance{
				AmountSat: 20000,
				UserID:    u.ID,
			},

			User{
				ID:      u.ID,
				Email:   u.Email,
				Balance: 80000,
			},
		},
		{
			ChangeBalance{
				AmountSat: 20000,
				UserID:    u.ID,
			},

			User{
				ID:      u.ID,
				Email:   u.Email,
				Balance: 60000,
			},
		},
		{
			ChangeBalance{
				AmountSat: 60000,
				UserID:    u.ID,
			},

			User{
				ID:      u.ID,
				Email:   u.Email,
				Balance: 0,
			},
		},
		{
			// This should fail because the users balance should already be 0
			ChangeBalance{
				AmountSat: 10,
				UserID:    u.ID,
			},

			User{
				ID: u.ID,
			},
		},
		{
			// This should fail because it is illegal to increase balance by a negative amount
			ChangeBalance{
				AmountSat: -30000,
				UserID:    u.ID,
			},

			User{
				ID: u.ID,
			},
		},
	}

	t.Log("testing decreasing user balance")
	{
		for i, tt := range tests {
			t.Logf("\ttest: %d\twhen decreasing balance by %d for user %d",
				i, tt.dec.AmountSat, tt.dec.UserID)
			{
				user, err := GetByID(testDB, tt.expectedResult.ID)
				if err != nil {
					t.Fatalf("Should be able to GetByID")
				}
				tx := testDB.MustBegin()
				u, err = DecreaseBalance(tx, tt.dec)
				if user.Balance < tt.dec.AmountSat {
					log.Info("should be in here")
					if err == nil {
						t.Logf(
							"\t%s\tDecreasing balance greater than balance should result in error. Expected user <nil> got \"%v\". Expected error != <nil>, got %v%s",
							fail,
							user,
							err,
							reset,
						)
						t.Fail()
						return
					}
					t.Logf(
						"\t%s\tDecreasing balance greater than balance should result in error \"UpdateUserBalance(): could not construct user update: pw: new row for relation \"users\" violates check constraint \"users_balance_check\"\"\n						                                   got \"%v\"%s",
						succeed,
						err,
						reset)
					return
				}

				if tt.dec.AmountSat <= 0 {
					if err.Error() != "amount cant be less than or equal to 0" {
						t.Logf(
							"\t%s\tDecreasing balance by a negative amount should result in error. Expected user <nil> got \"%v\". Expected error \"amount cant be less than or equal to 0\", got %v%s",
							fail,
							user,
							err,
							reset,
						)
						t.Fail()
						return
					}
					t.Logf(
						"\t%s\tDecreasing balance by a negative amount should result in error.%s",
						succeed,
						reset)
					return
				}
				if err != nil {
					t.Fatalf(
						"\t%s\tshould be able to DecreaseBalance. Error:  %+v\n%s",
						fail, err, reset)
				}
				tx.Commit()
				t.Logf("\t%s\tShould be able to DecreaseBalance%s", succeed, reset)

				{
					expectedResult := tt.expectedResult

					if u.ID != expectedResult.ID {
						t.Logf("\t%s\tID should be equal to expected ID. Expected \"%d\" got \"%d\"%s",
							fail,
							expectedResult.ID,
							u.ID,
							reset,
						)
						t.Fail()
					}

					if u.Email != expectedResult.Email {
						t.Logf("\t%s\tEmail should be equal to expected Email. Expected \"%s\" got \"%s\"%s",
							fail,
							expectedResult.Email,
							u.Email,
							reset,
						)
						t.Fail()
					}

					if u.Balance != expectedResult.Balance {
						t.Logf("\t%s\tBalance should be equal to expected Balance. Expected \"%d\" got \"%d\"%s",
							fail,
							expectedResult.Balance,
							u.Balance,
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

func TestIncreaseBalance(t *testing.T) {
	t.Parallel()
	// Arrange
	testDB, err := db.OpenDatabase(databaseConfig)
	if err != nil {
		t.Fatalf("%+v\n", err)
	}
	u, err := Create(testDB,
		"test_userIncreaseBalance@example.com",
		"password",
	)
	log.Infof("created user %v", u)

	tests := []struct {
		userID    int   `db:"user_id"`
		amountSat int64 `db:"amount_sat"`

		expectedResult User
	}{
		{
			u.ID,
			20000,

			User{
				ID:      u.ID,
				Email:   u.Email,
				Balance: 20000,
			},
		},
		{
			u.ID,
			20000,

			User{
				ID:      u.ID,
				Email:   u.Email,
				Balance: 40000,
			},
		},
		{
			u.ID,
			60000,

			User{
				ID:      u.ID,
				Email:   u.Email,
				Balance: 100000,
			},
		},
		{
			// This should fail because it is illegal to increase balance by a negative amount
			u.ID,
			-30000,

			User{
				ID: u.ID,
			},
		},
	}

	t.Log("testing increasing user balance")
	{
		for i, tt := range tests {
			t.Logf("\ttest: %d\twhen increasing balance by %d for user %d",
				i, tt.amountSat, tt.userID)
			{
				tx := testDB.MustBegin()
				user, err := IncreaseBalance(tx, ChangeBalance{UserID: tt.userID, AmountSat: tt.amountSat})
				if tt.amountSat <= 0 {
					if err.Error() != "amount cant be less than or equal to 0" {
						t.Logf(
							"\t%s\tIncreasing balance by a negative amount should result in error. Expected user <nil> got \"%v\". Expected error \"amount cant be less than or equal to 0\", got %v%s",
							fail,
							user,
							err,
							reset,
						)
						t.Fail()
						return
					}
					t.Logf(
						"\t%s\tIncreasing balance by a negative amount should result in error.%s",
						succeed,
						reset)
					return
				}

				if err != nil {
					t.Fatalf(
						"\t%s\tshould be able to IncreaseBalance. Error:  %+v\n%s",
						fail, err, reset)
				}
				tx.Commit()
				t.Logf("\t%s\tShould be able to IncreaseBalance%s", succeed, reset)

				{
					expectedResult := tt.expectedResult

					if user.ID != expectedResult.ID {
						t.Logf("\t%s\tID should be equal to expected ID. Expected \"%d\" got \"%d\"%s",
							fail,
							expectedResult.ID,
							user.ID,
							reset,
						)
						t.Fail()
					}

					if user.Email != expectedResult.Email {
						t.Logf("\t%s\tEmail should be equal to expected Email. Expected \"%s\" got \"%s\"%s",
							fail,
							expectedResult.Email,
							user.Email,
							reset,
						)
						t.Fail()
					}

					if user.Balance != expectedResult.Balance {
						t.Logf("\t%s\tBalance should be equal to expected Balance. Expected \"%d\" got \"%d\"%s",
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
