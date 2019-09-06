package users

import (
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"

	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/util"
)

var (
	databaseConfig = db.DatabaseConfig{
		User:     "lpp_test",
		Password: "password",
		Host:     util.GetEnvOrElse("DATABASE_HOST", "localhost"),
		Port:     util.GetDatabasePort(),
		Name:     "lpp_users",
	}
	testDB *db.DB
)

func TestMain(m *testing.M) {
	build.SetLogLevel(logrus.ErrorLevel)
	var err error

	log.Info("Configuring user test database")
	testDB, err = db.Open(databaseConfig)
	if err != nil {
		log.Fatalf("Could not open test database: %+v\n", err)
	}

	if err = testDB.Teardown(databaseConfig); err != nil {
		log.Fatalf("Could not tear down test DB: %v", err)
	}

	if err = testDB.Create(databaseConfig); err != nil {
		log.Fatalf("Could not create test database: %v", err)
	}

	flag.Parse()
	result := m.Run()

	os.Exit(result)
}

func TestCanCreateUser(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	testDB, err := db.Open(databaseConfig)
	if err != nil {
		testutil.FatalErr(t, err)
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

	for i, tt := range tests {
		t.Logf("\ttest %d\twhen creating user with email %s", i, tt.email)

		user, err := Create(testDB,
			tt.email,
			"password",
		)
		if err != nil {
			testutil.FatalErr(t, err)
		}

		expectedResult := tt.expectedResult

		if user.Email != expectedResult.Email {
			testutil.FatalMsgf(t,
				"Email should be equal to expected Email. Expected \"%s\" got \"%s\"",
				expectedResult.Email,
				user.Email,
			)
		}
		if user.Balance != expectedResult.Balance {
			testutil.FatalMsgf(t,
				"Balance should be equal to expected Balance. Expected: %d, got: %d",
				expectedResult.Balance,
				user.Balance,
			)
		}
	}
}

func TestCanGetUserByEmail(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	testDB, err := db.Open(databaseConfig)
	if err != nil {
		testutil.FatalErr(t, err)
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

	for i, tt := range tests {

		t.Logf("\ttest %d\twhen getting user with email %s", i, tt.user.Email)

		tx := testDB.MustBegin()
		user, err := insertUser(tx, User{
			Email:          tt.user.Email,
			HashedPassword: tt.user.HashedPassword,
		})
		if err != nil {
			testutil.FatalErr(t, err)
		}

		if err = tx.Commit(); err != nil {
			testutil.FatalErr(t, err)
		}

		user, err = GetByEmail(testDB, email)
		if err != nil {
			testutil.FatalErr(t, err)
		}

		expectedResult := tt.expectedResult

		if user.Email != expectedResult.Email {
			testutil.FatalMsgf(
				t,
				"Email should be equal to expected Email. Expected \"%s\" got \"%s\"",
				expectedResult.Email,
				user.Email,
			)
		}
		if user.Balance != expectedResult.Balance {
			testutil.FatalMsgf(

				t,
				"Balance should be equal to expected Balance. Expected: %d, got: %d",
				expectedResult.Balance,
				user.Balance,
			)
		}
	}
}

func TestCanGetUserByCredentials(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	testDB, err := db.Open(databaseConfig)
	if err != nil {
		testutil.FatalErr(t, err)
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

	for i, tt := range tests {
		t.Logf("\ttest %d\twhen getting user with email %s", i, tt.email)

		user, err := Create(testDB, tt.email, tt.password)
		if err != nil {
			testutil.FatalErr(t, err)
		}

		user, err = GetByCredentials(testDB, tt.email, tt.password)
		if err != nil {
			testutil.FatalErr(t, err)
		}

		expectedResult := tt.expectedResult

		if user.Email != expectedResult.Email {
			testutil.FatalMsgf(
				t,
				"Email should be equal to expected Email. Expected \"%s\" got \"%s\"",
				expectedResult.Email,
				user.Email,
			)
		}
		if user.Balance != expectedResult.Balance {
			testutil.FatalMsgf(
				t,
				"Balance should be equal to expected Balance. Expected: %d, got: %d",
				expectedResult.Balance,
				user.Balance,
			)
		}
	}
}

func TestCanGetUserByID(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)
	testDB, err := db.Open(databaseConfig)
	if err != nil {
		testutil.FatalErr(t, err)
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

	for i, tt := range tests {
		t.Logf("\ttest %d\twhen getting user with email %s", i, tt.user.Email)

		tx := testDB.MustBegin()
		u, err := insertUser(tx, User{
			Email:          tt.user.Email,
			HashedPassword: tt.user.HashedPassword,
		})
		if err != nil {
			testutil.FatalErr(t, err)
		}

		err = tx.Commit()
		if err != nil {
			testutil.FatalErr(t, err)
		}

		user, err := GetByID(testDB, u.ID)
		if err != nil {
			testutil.FatalErr(t, err)
		}

		expectedResult := tt.expectedResult

		if user.Email != expectedResult.Email {
			testutil.FatalMsgf(
				t,
				"Email should be equal to expected Email. Expected \"%s\" got \"%s\"",
				expectedResult.Email,
				user.Email,
			)
		}

		if user.Balance != expectedResult.Balance {
			testutil.FatalMsgf(
				t,
				"Balance should be equal to expected Balance. Expected: %d, got: %d",
				expectedResult.Balance,
				user.Balance,
			)
		}

	}
}

func TestNotDecreaseBalanceNegativeSats(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	// Arrange
	testDB, err := db.Open(databaseConfig)
	if err != nil {
		t.Fatalf("%+v\n", err)
	}
	u, err := Create(testDB,
		"test_userDecreaseBalanceNegativeSats@example.com",
		"password",
	)
	if err != nil {
		testutil.FatalErr(t, err)
	}

	// Give initial balance of 100 000
	tx := testDB.MustBegin()
	u, err = IncreaseBalance(tx, ChangeBalance{
		UserID:    u.ID,
		AmountSat: 100000,
	})
	if err != nil {
		testutil.FatalErr(t, err)
	}

	err = tx.Commit()
	if err != nil {
		testutil.FatalErr(t, err)
	}

	test := struct {
		dec ChangeBalance

		expectedResult User
	}{
		// This should fail because it is illegal to increase balance by a negative amount
		ChangeBalance{
			AmountSat: -30000,
			UserID:    u.ID,
		},

		User{
			ID: u.ID,
		},
	}

	decreased, err := DecreaseBalance(tx, test.dec)
	if !strings.Contains(err.Error(), "less than or equal to 0") {
		testutil.FatalMsgf(
			t,
			"Decreasing balance by a negative amount should result in error. Expected user <nil> got \"%v\". Expected error \"amount cant be less than or equal to 0\", got %v",
			decreased,
			err,
		)
	}
}

func TestNotDecreaseBalanceBelowZero(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	// Arrange
	testDB, err := db.Open(databaseConfig)
	if err != nil {
		t.Fatalf("%+v\n", err)
	}
	u, err := Create(testDB,
		"test_userDecreaseBalanceBelowZero@example.com",
		"password",
	)
	if err != nil {
		testutil.FatalErr(t, err)
	}

	// Give initial balance of 100 000
	tx := testDB.MustBegin()
	u, err = IncreaseBalance(tx, ChangeBalance{
		UserID:    u.ID,
		AmountSat: 100000,
	})
	if err != nil {
		testutil.FatalErr(t, err)
	}

	test := struct {
		change ChangeBalance
		user   User
	}{
		ChangeBalance{
			AmountSat: u.Balance + 1,
			UserID:    u.ID,
		},

		User{
			ID: u.ID,
		},
	}

	user, err := DecreaseBalance(tx, test.change)

	if err == nil {
		testutil.FatalMsgf(t,
			"Decreasing balance greater than balance should result in error. Expected user <nil> got \"%v\". Expected error != <nil>, got %v",
			user,
			err,
		)
	}
}

func TestDecreaseBalance(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	// Arrange
	u, err := Create(testDB,
		"test_userDecreaseBalance@example.com",
		"password",
	)
	if err != nil {
		testutil.FatalErr(t, err)
	}

	// Give initial balance of 100 000
	tx := testDB.MustBegin()
	u, err = IncreaseBalance(tx, ChangeBalance{
		UserID:    u.ID,
		AmountSat: 100000,
	})
	if err != nil {
		testutil.FatalErr(t, err)
	}

	err = tx.Commit()
	if err != nil {
		testutil.FatalErr(t, err)
	}

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
	}

	for i, tt := range tests {
		t.Logf("\ttest: %d\twhen decreasing balance by %d for user %d",
			i, tt.dec.AmountSat, tt.dec.UserID)

		_, err := GetByID(testDB, tt.expectedResult.ID)
		if err != nil {
			testutil.FatalErr(t, err)
		}

		tx := testDB.MustBegin()
		u, err = DecreaseBalance(tx, tt.dec)
		if err != nil {
			testutil.FatalErr(t, err)
		}

		err = tx.Commit()
		if err != nil {
			testutil.FailMsgf(t,
				"Could not commit balance decrease: %v", err)
		}

		expectedResult := tt.expectedResult

		if u.ID != expectedResult.ID {
			testutil.FailMsgf(
				t,
				"ID should be equal to expected ID. Expected \"%d\" got \"%d\"",
				expectedResult.ID,
				u.ID,
			)
		}

		if u.Email != expectedResult.Email {
			testutil.FailMsgf(
				t,
				"Email should be equal to expected Email. Expected \"%s\" got \"%s\"",
				expectedResult.Email,
				u.Email,
			)
		}

		if u.Balance != expectedResult.Balance {
			testutil.FailMsgf(
				t,
				"Balance should be equal to expected Balance. Expected \"%d\" got \"%d\"",
				expectedResult.Balance,
				u.Balance,
			)
		}

	}

	// Fail tests after all assertions that will not interfere with eachother
	// for improved test result readability.
	if t.Failed() {
		t.FailNow()
	}
}

func TestNotIncreaseBalanceNegativeSats(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	// Arrange
	testDB, err := db.Open(databaseConfig)
	if err != nil {
		t.Fatalf("%+v\n", err)
	}
	tx := testDB.MustBegin()

	u, err := Create(testDB,
		"test_userIncreaseBalanceNegativeSats@example.com",
		"password",
	)
	if err != nil {
		testutil.FatalMsgf(t,
			"Could not create user: %v", err)
	}

	user, err := IncreaseBalance(tx, ChangeBalance{UserID: u.ID, AmountSat: -300})
	if !strings.Contains(err.Error(), "less than or equal to 0") {
		testutil.FatalMsgf(
			t,
			"Increasing balance by a negative amount should result in error. Expected user <nil> got \"%v\". Expected error \"amount cant be less than or equal to 0\", got %v",
			user,
			err,
		)
	}
}

func TestIncreaseBalance(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	// Arrange
	testDB, err := db.Open(databaseConfig)
	if err != nil {
		testutil.FatalErr(t, err)
	}
	u, err := Create(testDB,
		"test_userIncreaseBalance@example.com",
		"password",
	)
	if err != nil {
		testutil.FatalMsgf(t,
			"Could not create user: %v", err)
	}

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
	}

	for i, tt := range tests {
		t.Logf("\ttest: %d\twhen increasing balance by %d for user %d",
			i, tt.amountSat, tt.userID)

		if err != nil {
			testutil.FatalMsgf(t,
				"should be able to IncreaseBalance. Error:  %+v",
				err)
		}

		tx := testDB.MustBegin()

		user, err := IncreaseBalance(tx, ChangeBalance{UserID: tt.userID, AmountSat: tt.amountSat})

		if err != nil {
			testutil.FailMsgf(
				t,
				"should be able to IncreaseBalance. Error:  %+v",
				err)
		}

		err = tx.Commit()
		if err != nil {
			testutil.FatalMsgf(t, "Could not commit: %v", err)
		}

		expectedResult := tt.expectedResult
		if user.ID != expectedResult.ID {
			testutil.FailMsgf(t,
				"tID should be equal to expected ID. Expected \"%d\" got \"%d\"",
				expectedResult.ID,
				user.ID,
			)
		}

		if user.Email != expectedResult.Email {
			testutil.FailMsgf(
				t,
				"Email should be equal to expected Email. Expected \"%s\" got \"%s\"",
				expectedResult.Email,
				user.Email,
			)
		}

		if user.Balance != expectedResult.Balance {
			testutil.FailMsgf(
				t,
				"Balance should be equal to expected Balance. Expected \"%d\" got \"%d\"",
				expectedResult.Balance,
				user.Balance,
			)
		}

	}

	// Fail tests after all assertions that will not interfere with eachother
	// for improved test result readability.
	if t.Failed() {
		t.FailNow()
	}
}
