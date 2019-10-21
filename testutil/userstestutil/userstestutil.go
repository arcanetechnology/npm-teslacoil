package userstestutil

import (
	"testing"

	"github.com/brianvoe/gofakeit"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/models/users"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
)

// CreateUserOrFail creates a user with a random email and password. The
// method also verifies the users email.
func CreateUserOrFail(t *testing.T, db *db.DB) users.User {
	passwordLen := gofakeit.Number(8, 32)
	password := gofakeit.Password(true, true, true, true, true, passwordLen)
	return CreateUserOrFailWithPassword(t, db, password)
}

// CreateUserOrFailWithPassword creates a user with a random email and the
// given password. The method also verifies the users email.
func CreateUserOrFailWithPassword(t *testing.T, db *db.DB, password string) users.User {
	u, err := users.Create(db, users.CreateUserArgs{
		Email:    gofakeit.Email(),
		Password: password,
	})
	if err != nil {
		testutil.FatalMsgf(t,
			"CreateUser(%s, db) -> should be able to CreateUser. Error:  %+v",
			t.Name(), err)
	}

	token, err := users.GetEmailVerificationToken(db, u.Email)
	if err != nil {
		testutil.FatalMsgf(t, "Could not get email verification token: %v", err)
	}

	verified, err := users.VerifyEmail(db, token)
	if err != nil {
		testutil.FatalMsgf(t, "Could not verify email: %v", verified)
	}

	return verified
}

// CreateUserWithBalanceOrFail creates a user with an initial balance
func CreateUserWithBalanceOrFail(t *testing.T, db *db.DB, balance int) users.User {
	u := CreateUserOrFail(t, db)

	return IncreaseBalanceOrFail(t, db, u, balance)
}

// IncreaseBalanceOrFail increases the balance of the given user
func IncreaseBalanceOrFail(t *testing.T, db *db.DB, user users.User, balance int) users.User {

	tx := db.MustBegin()
	user, err := users.IncreaseBalance(tx, users.ChangeBalance{
		UserID:    user.ID,
		AmountSat: int64(balance),
	})
	if err != nil {
		testutil.FatalMsgf(t,
			"[%s] could not increase balance by %d for user %d: %+v", t.Name(),
			balance, user.ID, err)
	}
	err = tx.Commit()
	if err != nil {
		testutil.FatalMsg(t, "could not commit tx")
	}

	testutil.AssertEqual(t, user.Balance, balance)
	return user
}
