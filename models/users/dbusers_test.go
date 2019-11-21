package users_test

import (
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"

	"gitlab.com/arcanecrypto/teslacoil/models/users"

	"github.com/brianvoe/gofakeit"
	"github.com/dchest/passwordreset"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
)

var (
	databaseConfig = testutil.GetDatabaseConfig("users")
	testDB         *db.DB
)

func TestMain(m *testing.M) {
	build.SetLogLevels(logrus.ErrorLevel)

	rand.Seed(time.Now().UnixNano())

	testDB = testutil.InitDatabase(databaseConfig)

	result := m.Run()

	if err := testDB.Close(); err != nil {
		panic(err.Error())
	}

	os.Exit(result)
}

func TestInsertUser(t *testing.T) {
	t.Run("can insert user", func(t *testing.T) {
		email := gofakeit.Email()
		hashedPassword := []byte("123")
		u := users.User{
			Email:          email,
			HashedPassword: hashedPassword,
		}

		insertedUser, err := users.InsertUser(testDB, u)
		require.NoError(t, err)

		assert.Equal(t, insertedUser.Email, email)
		assert.Equal(t, insertedUser.HashedPassword, hashedPassword)
	})
	t.Run("can not choose ID of user", func(t *testing.T) {
		u := users.User{
			ID:             999,
			Email:          gofakeit.Email(),
			HashedPassword: []byte("123"),
		}

		insertedUser, err := users.InsertUser(testDB, u)
		require.NoError(t, err)

		assert.NotEqual(t, 999, insertedUser.ID)
	})
	t.Run("can not insert without a hashed password", func(t *testing.T) {
		u := users.User{
			Email: gofakeit.Email(),
		}

		_, err := users.InsertUser(testDB, u)
		assert.Equal(t, users.ErrHashedPasswordMustBeDefined, err)
	})
	t.Run("can not insert without an email", func(t *testing.T) {
		u := users.User{
			ID:             999,
			HashedPassword: []byte("123"),
		}

		_, err := users.InsertUser(testDB, u)
		assert.Equal(t, users.ErrEmailMustBeDefined, err)
	})
}

func TestGetByID(t *testing.T) {
	t.Parallel()

	email := gofakeit.Email()
	tests := []struct {
		user           users.User
		expectedResult users.User
	}{
		{
			users.User{
				Email:          email,
				HashedPassword: []byte("SomePassword"),
			},
			users.User{
				Email: email,
			},
		},
	}

	for _, tt := range tests {
		u, err := users.InsertUser(testDB, tt.user)
		require.NoError(t, err)

		user, err := users.GetByID(testDB, u.ID)
		require.NoError(t, err)

		assert.Equal(t, user.Email, tt.expectedResult.Email)
	}
}

func TestGetByEmail(t *testing.T) {
	t.Parallel()

	email := gofakeit.Email()
	tests := []struct {
		user           users.User
		expectedResult users.User
	}{
		{
			users.User{
				Email:          email,
				HashedPassword: []byte("SomePassword"),
			},
			users.User{
				Email: email,
			},
		},
	}

	for _, tt := range tests {

		user, err := users.InsertUser(testDB, tt.user)
		require.NoError(t, err)

		user, err = users.GetByEmail(testDB, email)
		require.NoError(t, err)
		assert.Equal(t, user.Email, tt.expectedResult.Email)
	}
}

func TestGetByCredentials(t *testing.T) {
	t.Parallel()

	email := gofakeit.Email()
	tests := []struct {
		email          string
		password       string
		expectedResult users.User
	}{
		{
			email,
			"password",
			users.User{
				Email: email,
			},
		},
	}

	for _, tt := range tests {
		user, err := users.Create(testDB, users.CreateUserArgs{
			Email: tt.email, Password: tt.password,
		})
		require.NoError(t, err)

		user, err = users.GetByCredentials(testDB, tt.email, tt.password)
		require.NoError(t, err)

		assert.Equal(t, user.Email, tt.expectedResult.Email)
	}
}

func TestCreateUser(t *testing.T) {
	t.Parallel()

	t.Run("create user with just email", func(t *testing.T) {
		email := gofakeit.Email()
		user, err := users.Create(testDB,
			users.CreateUserArgs{
				Email:    email,
				Password: "password",
			})
		require.NoError(t, err)

		assert.Equal(t, user.Email, email)
	})
}

func TestGetEmailVerificationToken(t *testing.T) {
	t.Parallel()
	user := createUserOrFail(t)
	t.Run("get token for existing user", func(t *testing.T) {
		_, err := users.GetEmailVerificationToken(testDB, user.Email)
		require.NoError(t, err)
	})
	t.Run("don't get token for non existant user", func(t *testing.T) {
		_, err := users.GetEmailVerificationToken(testDB, gofakeit.Email())
		assert.Error(t, err)
	})
}

func TestVerifyEmailVerificationToken(t *testing.T) {
	t.Parallel()
	user := createUserOrFail(t)
	t.Run("verify valid token", func(t *testing.T) {
		token, err := users.GetEmailVerificationToken(testDB, user.Email)
		require.NoError(t, err)

		login, err := users.VerifyEmailVerificationToken(token)
		require.NoError(t, err)
		assert.Equal(t, user.Email, login)
	})

	t.Run("creating a token for a different user should yield a different login", func(t *testing.T) {
		otherUser := createUserOrFail(t)
		token, err := users.GetEmailVerificationToken(testDB, otherUser.Email)
		require.NoError(t, err)

		login, err := users.VerifyEmailVerificationToken(token)
		require.NoError(t, err)

		assert.NotEqual(t, user.Email, login)
	})

	t.Run("fail to verify token created with bad key", func(t *testing.T) {
		token, err := users.GetEmailVerificationTokenWithKey(testDB, user.Email, []byte("bad key"))
		require.NoError(t, err)

		_, err = users.VerifyEmailVerificationToken(token)
		require.Error(t, err)
	})
}

func TestVerifyEmail(t *testing.T) {
	t.Parallel()
	t.Run("verify email", func(t *testing.T) {
		t.Parallel()

		user := createUserOrFailNoEmailVerify(t)
		assert.False(t, user.HasVerifiedEmail, "User was created with verified email!")

		token, err := users.GetEmailVerificationToken(testDB, user.Email)
		require.NoError(t, err)

		verified, err := users.VerifyEmail(testDB, token)
		require.NoError(t, err)

		assert.True(t, verified.HasVerifiedEmail, "Email didn't get marked as verified!")
	})

	t.Run("don't verify email with bad key", func(t *testing.T) {
		t.Parallel()

		user := createUserOrFailNoEmailVerify(t)
		assert.False(t, user.HasVerifiedEmail, "User was created with verified email!")

		_, err := users.GetEmailVerificationTokenWithKey(testDB, user.Email, []byte("badddddd key"))
		require.NoError(t, err)

		sameUser, err := users.GetByEmail(testDB, user.Email)
		require.NoError(t, err)

		assert.False(t, sameUser.HasVerifiedEmail, "Users email got marked as verified")

	})
}

func TestGetPasswordResetToken(t *testing.T) {
	t.Parallel()

	t.Run("Get a token for an existing user", func(t *testing.T) {
		user := createUserOrFail(t)
		_, err := users.NewPasswordResetToken(testDB, user.Email)
		assert.NoError(t, err)
	})
	t.Run("Fail to get a token for an non-existing user", func(t *testing.T) {
		_, err := users.NewPasswordResetToken(testDB, gofakeit.Email())
		assert.Error(t, err)
	})
}

func TestVerifyPasswordResetToken(t *testing.T) {
	t.Parallel()

	user := createUserOrFail(t)

	t.Run("Verify a token we created", func(t *testing.T) {
		token, err := users.NewPasswordResetToken(testDB, user.Email)
		require.NoError(t, err)
		email, err := users.VerifyPasswordResetToken(testDB, token)
		require.NoError(t, err)
		assert.Equal(t, email, user.Email)
	})

	t.Run("Don't verify a token we didn't create", func(t *testing.T) {
		duration := 1 * time.Hour
		secretKey := []byte("this is a secret key")
		badToken := passwordreset.NewToken(user.Email, duration,
			user.HashedPassword, secretKey)
		_, err := users.VerifyPasswordResetToken(testDB, badToken)
		assert.Error(t, err)
	})
}

func TestChangePassword(t *testing.T) {
	t.Parallel()
	newPassword := gofakeit.Password(true, true, true, true, true, 32)
	user := createUserOrFail(t)

	err := bcrypt.CompareHashAndPassword(user.HashedPassword,
		[]byte(newPassword))
	assert.Error(t, err, "User has new password before reset occured")

	updated, err := user.ChangePassword(testDB, newPassword)
	require.NoError(t, err)
	err = bcrypt.CompareHashAndPassword(updated.HashedPassword, []byte(newPassword))
	assert.NoError(t, err, "User password did not change: %v")
}

func TestCreateConfirmAndDelete2FA(t *testing.T) {
	t.Parallel()
	user := createUserOrFail(t)

	key, err := user.Create2faCredentials(testDB)
	require.NoError(t, err)

	updated, err := users.GetByID(testDB, user.ID)
	require.NoError(t, err)
	assert.NotNil(t, updated.TotpSecret)
	assert.False(t, updated.ConfirmedTotpSecret, "User unexpectedly had confirmed TOTP secret")
	assert.Equal(t, key.Issuer(), users.TotpIssuer, key.Issuer())

	t.Run("not confirm with bad 2FA credentials", func(t *testing.T) {
		_, err := updated.Confirm2faCredentials(testDB, "123456")
		assert.Error(t, err)
	})

	t.Run("confirm 2FA credentials", func(t *testing.T) {
		totpCode, err := totp.GenerateCode(*updated.TotpSecret, time.Now())
		require.NoError(t, err)

		enabled, err := updated.Confirm2faCredentials(testDB, totpCode)
		require.NoError(t, err)

		assert.True(t, enabled.ConfirmedTotpSecret, "User hasn't confirmed TOTP secret")

		t.Run("fail to disable 2FA credentials with a bad passcode", func(t *testing.T) {
			_, err := enabled.Delete2faCredentials(testDB, "123456")
			assert.Error(t, err)
		})
		t.Run("disable 2FA credentials", func(t *testing.T) {
			totpCode, err := totp.GenerateCode(*updated.TotpSecret, time.Now())
			require.NoError(t, err)

			disabled, err := enabled.Delete2faCredentials(testDB, totpCode)
			require.NoError(t, err)

			assert.False(t, disabled.ConfirmedTotpSecret, "User has confirmed TOTP secret")
			assert.Nil(t, disabled.TotpSecret)
		})

	})
}

func TestUpdateUserFailWithBadOpts(t *testing.T) {
	t.Parallel()
	user := createUserOrFail(t)

	_, err := user.Update(testDB, users.UpdateOptions{})
	assert.Error(t, err)
}

func TestUpdateUserEmail(t *testing.T) {
	t.Parallel()

	user := createUserOrFail(t)

	newEmail := gofakeit.Email()
	updated, err := user.Update(testDB, users.UpdateOptions{NewEmail: &newEmail})
	require.NoError(t, err)
	assert.Equal(t, updated.Email, newEmail)

	empty := ""
	_, err = user.Update(testDB, users.UpdateOptions{NewEmail: &empty})
	assert.Error(t, err, "Was able to delete user email!")
}

func TestUpdateUserFirstName(t *testing.T) {
	t.Parallel()

	user := createUserOrFail(t)

	newName := "NewLastName"
	updated, err := user.Update(testDB, users.UpdateOptions{NewLastName: &newName})
	require.NoError(t, err)
	assert.Equal(t, updated.Lastname, &newName)

	empty := ""
	removed, err := user.Update(testDB, users.UpdateOptions{NewLastName: &empty})
	require.NoError(t, err)
	assert.Nil(t, removed.Lastname)
}

func TestUpdateUserLastName(t *testing.T) {
	t.Parallel()

	user := createUserOrFail(t)

	newName := "NewFirstName"
	updated, err := user.Update(testDB, users.UpdateOptions{NewFirstName: &newName})
	require.NoError(t, err)
	assert.Equal(t, updated.Firstname, &newName)
	empty := ""
	removed, err := user.Update(testDB, users.UpdateOptions{NewFirstName: &empty})
	require.NoError(t, err)
	assert.Nil(t, removed.Firstname)

}

func TestFailToUpdateNonExistingUser(t *testing.T) {
	t.Parallel()
	email := gofakeit.Email()
	user := users.User{ID: 99999}
	_, err := user.Update(testDB, users.UpdateOptions{NewEmail: &email})

	assert.Error(t, err)
}

// The following functions are copy paste replicated here as well as in
// the userstestutil package. This is to avoid a cyclical dependency (which
// is a compiler failure)

// createUserOrFailNoEmailVerify creates a user with a random email and password
func createUserOrFailNoEmailVerify(t *testing.T) users.User {
	passwordLen := gofakeit.Number(8, 32)
	password := gofakeit.Password(true, true, true, true, true, passwordLen)

	u, err := users.Create(testDB, users.CreateUserArgs{
		Email:    gofakeit.Email(),
		Password: password,
	})
	require.NoError(t, err)
	return u
}

// createUserOrFail creates a user and verifies their email
func createUserOrFail(t *testing.T) users.User {
	user := createUserOrFailNoEmailVerify(t)
	token, err := users.GetEmailVerificationToken(testDB, user.Email)
	require.NoError(t, err)

	verified, err := users.VerifyEmail(testDB, token)
	require.NoError(t, err)

	return verified
}
