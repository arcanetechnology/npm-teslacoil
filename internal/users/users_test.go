package users

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"

	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
)

var (
	databaseConfig = testutil.GetDatabaseConfig("users")
	testDB         *db.DB
)

func TestMain(m *testing.M) {
	build.SetLogLevel(logrus.ErrorLevel)

	testDB = testutil.InitDatabase(databaseConfig)

	log.Info("Configuring user test database")

	flag.Parse()
	result := m.Run()

	os.Exit(result)
}

func getTestEmail(t *testing.T) string {
	return fmt.Sprintf("%s@example.com", t.Name())
}

func TestUpdateEmail(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)
	email := getTestEmail(t)

	user, err := Create(testDB, email, "password")
	if err != nil {
		testutil.FatalMsg(t, err)
	}

	newEmail := "new-" + email
	updated, err := user.UpdateEmail(testDB, newEmail)
	if err != nil {
		testutil.FatalMsg(t, err)
	}

	if updated.Email == email {
		testutil.FatalMsg(t, "UpdateEmail did not change emails!")
	}

	if updated.Email != newEmail {
		testutil.FatalMsgf(t,
			"UpdateEmail did not change to expected result! Expected %s, got %s",
			newEmail, updated.Email)
	}
}

func TestFailToUpdateNonExistingUser(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)
	email := getTestEmail(t)
	user := UserResponse{ID: 99999}
	_, err := user.UpdateEmail(testDB, email)

	if err == nil {
		testutil.FatalMsg(t, "Was able to update email of non existant user!")
	}
}

func TestCanCreateUser(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	email := getTestEmail(t)
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
			testutil.FatalMsg(t, err)
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

	email := getTestEmail(t)
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
		user, err := insertUser(tx, tt.user)
		if err != nil {
			testutil.FatalMsg(t, err)
		}

		if err = tx.Commit(); err != nil {
			testutil.FatalMsg(t, err)
		}

		user, err = GetByEmail(testDB, email)
		if err != nil {
			testutil.FatalMsg(t, err)
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

	email := getTestEmail(t)
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
			testutil.FatalMsg(t, err)
		}

		user, err = GetByCredentials(testDB, tt.email, tt.password)
		if err != nil {
			testutil.FatalMsg(t, err)
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

	email := getTestEmail(t)
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
		u, err := insertUser(tx, tt.user)
		if err != nil {
			testutil.FatalMsg(t, err)
		}

		err = tx.Commit()
		if err != nil {
			testutil.FatalMsg(t, err)
		}

		user, err := GetByID(testDB, u.ID)
		if err != nil {
			testutil.FatalMsg(t, err)
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
	u, err := Create(testDB,
		getTestEmail(t),
		"password",
	)
	if err != nil {
		testutil.FatalMsg(t, err)
	}

	// Give initial balance of 100 000
	tx := testDB.MustBegin()
	u, err = IncreaseBalance(tx, ChangeBalance{
		UserID:    u.ID,
		AmountSat: 100000,
	})
	if err != nil {
		testutil.FatalMsg(t, err)
	}

	err = tx.Commit()
	if err != nil {
		testutil.FatalMsg(t, err)
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
	u, err := Create(testDB,
		getTestEmail(t),
		"password",
	)
	if err != nil {
		testutil.FatalMsg(t, err)
	}

	// Give initial balance of 100 000
	tx := testDB.MustBegin()
	u, err = IncreaseBalance(tx, ChangeBalance{
		UserID:    u.ID,
		AmountSat: 100000,
	})
	if err != nil {
		testutil.FatalMsg(t, err)
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
		getTestEmail(t),
		"password",
	)
	if err != nil {
		testutil.FatalMsg(t, err)
	}

	// Give initial balance of 100 000
	tx := testDB.MustBegin()
	u, err = IncreaseBalance(tx, ChangeBalance{
		UserID:    u.ID,
		AmountSat: 100000,
	})
	if err != nil {
		testutil.FatalMsg(t, err)
	}

	err = tx.Commit()
	if err != nil {
		testutil.FatalMsg(t, err)
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
			testutil.FatalMsg(t, err)
		}

		tx := testDB.MustBegin()
		u, err = DecreaseBalance(tx, tt.dec)
		if err != nil {
			testutil.FatalMsg(t, err)
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
	u, err := Create(testDB,
		getTestEmail(t),
		"password",
	)
	if err != nil {
		testutil.FatalMsgf(t,
			"Could not create user: %v", err)
	}

	tx := testDB.MustBegin()
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
	u, err := Create(testDB,
		getTestEmail(t),
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
