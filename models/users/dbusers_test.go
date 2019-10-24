package users

import (
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit"
	"github.com/dchest/passwordreset"
	"github.com/pquerna/otp/totp"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/db"
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

func TestUser_CreateConfirmAndDelete2FA(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)
	user := CreateUserOrFail(t)

	key, err := user.Create2faCredentials(testDB)
	if err != nil {
		testutil.FatalMsg(t, err)
	}

	updated, err := GetByID(testDB, user.ID)
	if err != nil {
		testutil.FatalMsg(t, err)
	}
	testutil.AssertMsg(t, updated.TotpSecret != nil, "TOTP secret was nil")
	testutil.AssertMsg(t, !updated.ConfirmedTotpSecret, "User unexpectedly had confirmed TOTP secret")
	testutil.AssertMsgf(t, key.Issuer() == TotpIssuer, "Key had unexpected issuer: %s", key.Issuer())

	t.Run("not confirm with bad 2FA credentials", func(t *testing.T) {
		_, err := updated.Confirm2faCredentials(testDB, "123456")
		if err == nil {
			testutil.FatalMsg(t, "was able to enable 2FA with bad code")
		}
	})

	t.Run("confirm 2FA credentials", func(t *testing.T) {
		totpCode, err := totp.GenerateCode(*updated.TotpSecret, time.Now())
		if err != nil {
			testutil.FatalMsg(t, err)
		}
		enabled, err := updated.Confirm2faCredentials(testDB, totpCode)
		if err != nil {
			testutil.FatalMsg(t, err)
		}
		testutil.AssertMsg(t, enabled.ConfirmedTotpSecret, "User hasn't confirmed TOTP secret")

		t.Run("fail to disable 2FA credentials with a bad passcode", func(t *testing.T) {
			_, err := enabled.Delete2faCredentials(testDB, "123456")
			if err == nil {
				testutil.FatalMsg(t, "was able to delete 2FA credentials with bad code")
			}
		})
		t.Run("disable 2FA credentials", func(t *testing.T) {
			totpCode, err := totp.GenerateCode(*updated.TotpSecret, time.Now())
			if err != nil {
				testutil.FatalMsg(t, err)
			}
			disabled, err := enabled.Delete2faCredentials(testDB, totpCode)
			if err != nil {
				testutil.FatalMsg(t, err)
			}
			testutil.AssertMsg(t, !disabled.ConfirmedTotpSecret, "User has confirmed TOTP secret")
			testutil.AssertEqual(t, disabled.TotpSecret, nil)

		})

	})
}

func TestUser_ResetPassword(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)
	password := gofakeit.Password(true, true, true, true, true, 32)
	user := CreateUserOrFail(t)

	if err := bcrypt.CompareHashAndPassword(user.HashedPassword, []byte(password)); err == nil {
		testutil.FatalMsg(t, "User has new password before reset occured")
	}

	updated, err := user.ResetPassword(testDB, password)
	if err != nil {
		testutil.FatalMsg(t, err)
	}
	if err := bcrypt.CompareHashAndPassword(updated.HashedPassword, []byte(password)); err != nil {
		testutil.FatalMsgf(t, "User password did not change: %v", err)
	}
}

func TestGetPasswordResetToken(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	t.Run("Get a token for an existing user", func(t *testing.T) {
		user := CreateUserOrFail(t)
		_, err := GetPasswordResetToken(testDB, user.Email)
		if err != nil {
			testutil.FatalMsg(t, err)
		}
	})
	t.Run("Fail to get a token for an non-existing user", func(t *testing.T) {
		_, err := GetPasswordResetToken(testDB, gofakeit.Email())
		if err == nil {
			testutil.FatalMsg(t, "Was able to get a token from a non existing user!")
		}
	})
}

func TestVerifyPasswordResetToken(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	user := CreateUserOrFail(t)

	t.Run("Verify a token we created", func(t *testing.T) {
		token, err := GetPasswordResetToken(testDB, user.Email)
		if err != nil {
			testutil.FatalMsg(t, err)
		}
		email, err := VerifyPasswordResetToken(testDB, token)
		if err != nil {
			testutil.FatalMsgf(t, "Wasn't able to verify token: %+v", err)
		}
		testutil.AssertEqual(t, email, user.Email)
	})

	t.Run("Don't verify a token we didn't create", func(t *testing.T) {
		duration := 1 * time.Hour
		secretKey := []byte("this is a secret key")
		badToken := passwordreset.NewToken(user.Email, duration,
			user.HashedPassword, secretKey)
		if _, err := VerifyPasswordResetToken(testDB, badToken); err == nil {
			testutil.FatalMsg(t, "Was able to verify a bad token!")
		}
	})
}

func TestCanCreateUser(t *testing.T) {
	t.Parallel()

	t.Run("create user with just email", func(t *testing.T) {
		email := gofakeit.Email()
		user, err := Create(testDB,
			CreateUserArgs{
				Email:    email,
				Password: "password",
			})
		if err != nil {
			testutil.FatalMsg(t, err)
		}

		testutil.AssertEqual(t, user.Email, email)
	})

	t.Run("inserting user ID 0 should not result in that user ID being used", func(t *testing.T) {
		user := User{
			ID:             0,
			Email:          gofakeit.Email(),
			Firstname:      nil,
			Lastname:       nil,
			HashedPassword: []byte("this is a hashed password"),
		}
		inserted, err := insertUser(testDB.MustBegin(), user)
		if err != nil {
			testutil.FatalMsg(t, err)
		}
		testutil.AssertNotEqual(t, inserted.ID, user.ID)

	})
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
				Email: email,
			},
		},
	}

	for _, tt := range tests {

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
		testutil.AssertEqual(t, user.Email, tt.expectedResult.Email)
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
				Email: email,
			},
		},
	}

	for _, tt := range tests {
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

		testutil.AssertEqual(t, user.Email, tt.expectedResult.Email)
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
				Email: email,
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

		testutil.AssertEqual(t, user.Email, tt.expectedResult.Email)
	}
}

func TestGetEmailVerificationToken(t *testing.T) {
	t.Parallel()
	user := CreateUserOrFail(t)
	t.Run("get token for existing user", func(t *testing.T) {
		_, err := GetEmailVerificationToken(testDB, user.Email)
		if err != nil {
			testutil.FatalMsg(t, err)
		}
	})
	t.Run("don't get token for non existant user", func(t *testing.T) {
		_, err := GetEmailVerificationToken(testDB, gofakeit.Email())
		if err == nil {
			testutil.FatalMsg(t, "Got token for non existant user!")
		}
	})
}

func TestVerifyEmailVerificationToken(t *testing.T) {
	t.Parallel()
	user := CreateUserOrFail(t)
	t.Run("verify valid token", func(t *testing.T) {
		token, err := GetEmailVerificationToken(testDB, user.Email)
		if err != nil {
			testutil.FatalMsg(t, err)
		}

		login, err := verifyEmailVerificationToken(token)
		if err != nil {
			testutil.FatalMsg(t, err)
		}
		testutil.AssertEqual(t, user.Email, login)
	})

	t.Run("creating a token for a different user should yield a different login", func(t *testing.T) {
		otherUser := CreateUserOrFail(t)
		token, err := GetEmailVerificationToken(testDB, otherUser.Email)
		if err != nil {
			testutil.FatalMsg(t, err)
		}

		login, err := verifyEmailVerificationToken(token)
		if err != nil {
			testutil.FatalMsg(t, err)
		}
		testutil.AssertNotEqual(t, user.Email, login)
	})

	t.Run("fail to verify token created with bad key", func(t *testing.T) {
		token, err := getEmailVerificationTokenWithKey(testDB, user.Email, []byte("bad key"))
		if err != nil {
			testutil.FatalMsg(t, err)
		}

		if _, err := verifyEmailVerificationToken(token); err == nil {
			testutil.FatalMsg(t, "Was able to verify token created with bad key!")
		}
	})
}

func TestVerifyEmail(t *testing.T) {
	t.Parallel()
	t.Run("verify email", func(t *testing.T) {
		t.Parallel()

		user := CreateUserOrFailNoEmailVerify(t)
		testutil.AssertMsg(t, !user.HasVerifiedEmail, "User was created with verified email!")

		token, err := GetEmailVerificationToken(testDB, user.Email)
		if err != nil {
			testutil.FatalMsg(t, err)
		}
		verified, err := VerifyEmail(testDB, token)
		if err != nil {
			testutil.FatalMsg(t, err)
		}

		testutil.AssertMsg(t, verified.HasVerifiedEmail, "Email didn't get marked as verified!")
	})

	t.Run("don't verify email with bad key", func(t *testing.T) {
		t.Parallel()

		user := CreateUserOrFailNoEmailVerify(t)
		testutil.AssertMsg(t, !user.HasVerifiedEmail, "User was created with verified email!")

		token, err := getEmailVerificationTokenWithKey(testDB, user.Email, []byte("badddddd key"))
		if err != nil {
			testutil.FatalMsg(t, err)
		}
		if _, err = VerifyEmail(testDB, token); err == nil {
			testutil.FatalMsgf(t, "Was able to verify email with bad key!")
		}

		sameUser, err := GetByEmail(testDB, user.Email)
		if err != nil {
			testutil.FatalMsg(t, err)
		}
		testutil.AssertMsg(t, !sameUser.HasVerifiedEmail, "Users email got marked as verified")

	})
}

// The following functions are copy paste replicated here as well as in
// the userstestutil package. This is to avoid a cyclical dependency (which
// is a compiler failure)

// CreateUserOrFailNoEmailVerify creates a user with a random email and password
func CreateUserOrFailNoEmailVerify(t *testing.T) User {
	passwordLen := gofakeit.Number(8, 32)
	password := gofakeit.Password(true, true, true, true, true, passwordLen)

	u, err := Create(testDB, CreateUserArgs{
		Email:    gofakeit.Email(),
		Password: password,
	})
	if err != nil {
		testutil.FatalMsgf(t,
			"CreateUser(%s, db) -> should be able to CreateUser. Error:  %+v",
			t.Name(), err)
	}
	return u
}

// CreateUserOrFail creates a user and verifies their email
func CreateUserOrFail(t *testing.T) User {
	user := CreateUserOrFailNoEmailVerify(t)
	token, err := GetEmailVerificationToken(testDB, user.Email)
	if err != nil {
		testutil.FatalMsg(t, err)
	}

	verified, err := VerifyEmail(testDB, token)
	if err != nil {
		testutil.FatalMsg(t, err)
	}

	return verified
}
