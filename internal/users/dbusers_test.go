package users

import (
	"flag"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"golang.org/x/crypto/bcrypt"
)

var (
	databaseConfig = testutil.GetDatabaseConfig("users")
	testDB         *db.DB
)

func TestMain(m *testing.M) {
	build.SetLogLevel(logrus.ErrorLevel)

	rand.Seed(time.Now().UnixNano())

	testDB = testutil.InitDatabase(databaseConfig)

	log.Info("Configuring user test database")

	flag.Parse()
	result := m.Run()

	if err := testDB.Close(); err != nil {
		panic(err.Error())
	}

	os.Exit(result)
}

func TestUpdateUserFailWithBadOpts(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)
	user := CreateUserOrFail(t)

	if _, err := user.Update(testDB, UpdateOptions{}); err == nil {
		testutil.FatalMsg(t, "Was able to give non-meaningful options and get a result")
	}
}

func TestUpdateUserEmail(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	user := CreateUserOrFail(t)

	newEmail := testutil.GetTestEmail(t)
	updated, err := user.Update(testDB, UpdateOptions{NewEmail: &newEmail})
	if err != nil {
		testutil.FatalMsgf(t, "Was not able to set email %s: %+v", newEmail, err)
	}
	if updated.Email != newEmail {
		testutil.FatalMsgf(t, "Got unexpected result after updating email: %+v", user)
	}

	empty := ""
	if _, err := user.Update(testDB, UpdateOptions{NewEmail: &empty}); err == nil {
		testutil.FatalMsg(t, "Was able to delete user email!")
	}
}

func TestUpdateUserFirstName(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	user := CreateUserOrFail(t)

	newName := "NewLastName"
	updated, err := user.Update(testDB, UpdateOptions{NewLastName: &newName})
	if err != nil {
		testutil.FatalMsgf(t, "Was not able to set last name: %+v", err)
	}
	if updated.Lastname == nil || *updated.Lastname != newName {
		testutil.FatalMsgf(t, "Got unexpected result after updating last name: %+v", updated)
	}
	empty := ""
	removed, err := user.Update(testDB, UpdateOptions{NewLastName: &empty})
	if err != nil {
		testutil.FatalMsgf(t, "Was not able to remove last name: %+v", err)
	}
	if removed.Lastname != nil {
		testutil.FatalMsgf(t, "Didn't unset last name: %+v", removed)
	}
}

func TestUpdateUserLastName(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	user := CreateUserOrFail(t)

	newName := "NewFirstName"
	updated, err := user.Update(testDB, UpdateOptions{NewFirstName: &newName})
	if err != nil {
		testutil.FatalMsgf(t, "Was not able to set first name: %+v", err)
	}
	if updated.Firstname == nil || *updated.Firstname != newName {
		testutil.FatalMsgf(t, "Got unexpected result after updating first name: %+v", user)
	}
	empty := ""
	removed, err := user.Update(testDB, UpdateOptions{NewFirstName: &empty})
	if err != nil {
		testutil.FatalMsgf(t, "Was not able to remove first name: %+v", err)
	}
	if removed.Firstname != nil {
		testutil.FatalMsgf(t, "Didn't unset first name: %+v", removed)
	}

}

func TestFailToUpdateNonExistingUser(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)
	email := testutil.GetTestEmail(t)
	user := User{ID: 99999}
	_, err := user.Update(testDB, UpdateOptions{NewEmail: &email})

	if err == nil || !strings.Contains(err.Error(), "given rows did not have any elements") {
		testutil.FatalMsgf(t,
			"Was able to update email of non existant user: %v", err)
	}
}

func TestUser_ChangePassword(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)
	oldPassword := gofakeit.Password(true, true, true, true, true, 32)
	newPassword := gofakeit.Password(true, true, true, true, true, 32)
	yetAnotherNewpassword := gofakeit.Password(
		true, true, true, true, true, 32)

	user := CreateUserOrFailWithPassword(t, oldPassword)

	t.Run("Must not be able to change password by providing a mismatched old password", func(t *testing.T) {
		if _, err := user.ChangePassword(testDB, newPassword, yetAnotherNewpassword); err == nil {
			testutil.FatalMsg(t, "Was able to change user password by giving a bad old password!")
		}
	})

	t.Run("Must be able to change the user password", func(t *testing.T) {
		updated, err := user.ChangePassword(testDB, oldPassword, newPassword)
		if err != nil {
			testutil.FatalMsg(t, errors.Wrapf(err, "wasn't able to update user password"))
		}

		if err = bcrypt.CompareHashAndPassword(updated.HashedPassword, []byte(newPassword)); err != nil {
			testutil.FatalMsg(t, err)
		}
	})

}

func TestCanCreateUser(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	email := testutil.GetTestEmail(t)
	tests := []struct {
		email          string
		expectedResult User
	}{
		{
			email,
			User{
				Email:   email,
				Balance: 0,
			},
		},
	}

	for i, tt := range tests {
		t.Logf("\ttest %d\twhen creating user with email %s", i, tt.email)

		user, err := Create(testDB,
			CreateUserArgs{
				Email:    tt.email,
				Password: "password",
			})
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

	email := testutil.GetTestEmail(t)
	tests := []struct {
		user           User
		expectedResult User
	}{
		{
			User{
				Email:          email,
				HashedPassword: []byte("SomePassword"),
			},
			User{
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

	email := testutil.GetTestEmail(t)
	tests := []struct {
		email          string
		password       string
		expectedResult User
	}{
		{
			email,
			"password",
			User{
				Email:   email,
				Balance: 0,
			},
		},
	}

	t.Log("testing can get user by email")

	for i, tt := range tests {
		t.Logf("\ttest %d\twhen getting user with email %s", i, tt.email)

		user, err := Create(testDB, CreateUserArgs{
			Email: tt.email, Password: tt.password,
		})
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

	email := testutil.GetTestEmail(t)
	tests := []struct {
		user           User
		expectedResult User
	}{
		{
			User{
				Email:          email,
				HashedPassword: []byte("SomePassword"),
			},
			User{
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
	user := CreateUserOrFail(t)

	// Give initial balance of 100 000
	tx := testDB.MustBegin()
	user, err := IncreaseBalance(tx, ChangeBalance{
		UserID:    user.ID,
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
			UserID:    user.ID,
		},

		User{
			ID: user.ID,
		},
	}

	decreased, err := DecreaseBalance(tx, test.dec)
	if err != nil && !strings.Contains(err.Error(), "less than or equal to 0") {
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
	user := CreateUserOrFail(t)

	// Give initial balance of 100 000
	tx := testDB.MustBegin()
	user, err := IncreaseBalance(tx, ChangeBalance{
		UserID:    user.ID,
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
			AmountSat: user.Balance + 1,
			UserID:    user.ID,
		},

		User{
			ID: user.ID,
		},
	}

	user, err = DecreaseBalance(tx, test.change)
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
	u := CreateUserOrFail(t)

	// Give initial balance of 100 000
	tx := testDB.MustBegin()
	u, err := IncreaseBalance(tx, ChangeBalance{
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
}

func TestNotIncreaseBalanceNegativeSats(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	// Arrange
	u := CreateUserOrFail(t)

	tx := testDB.MustBegin()
	user, err := IncreaseBalance(tx, ChangeBalance{UserID: u.ID, AmountSat: -300})
	if err != nil && !strings.Contains(err.Error(), "less than or equal to 0") {
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
	u := CreateUserOrFail(t)

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
}

// CreateUserOrFail creates a user with a random email and password
func CreateUserOrFail(t *testing.T) User {
	passwordLen := gofakeit.Number(8, 32)
	password := gofakeit.Password(true, true, true, true, true, passwordLen)
	return CreateUserOrFailWithPassword(t, password)
}

// CreateuserOrFailWithPassword creates a user with a random email and the
// given password
func CreateUserOrFailWithPassword(t *testing.T, password string) User {
	u, err := Create(testDB, CreateUserArgs{
		Email:    testutil.GetTestEmail(t),
		Password: password,
	})
	if err != nil {
		testutil.FatalMsgf(t,
			"CreateUser(%s, db) -> should be able to CreateUser. Error:  %+v",
			t.Name(), err)
	}
	return u
}