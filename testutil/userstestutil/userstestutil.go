package userstestutil

import (
	"testing"

	"github.com/brianvoe/gofakeit"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"
	"gitlab.com/arcanecrypto/teslacoil/internal/users"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
)

// CreateUserOrFail creates a user with a random email and password
func CreateUserOrFail(t *testing.T, db *db.DB) users.User {
	passwordLen := gofakeit.Number(8, 32)
	password := gofakeit.Password(true, true, true, true, true, passwordLen)
	return CreateUserOrFailWithPassword(t, db, password)
}

// CreateUserOrFailWithPassword creates a user with a random email and the
// given password
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
	return u
}

// CreateUserWithBalanceOrFail creates a user with an initial balance
func CreateUserWithBalanceOrFail(t *testing.T, db *db.DB, balance int) users.User {
	u := CreateUserOrFail(t, db)

	tx := db.MustBegin()
	user, err := users.IncreaseBalance(tx, users.ChangeBalance{
		UserID:    u.ID,
		AmountSat: int64(balance),
	})
	if err != nil {
		testutil.FatalMsgf(t,
			"[%s] could not increase balance by %d for user %d: %+v", t.Name(),
			balance, u.ID, err)
	}
	err = tx.Commit()
	if err != nil {
		testutil.FatalMsg(t, "could not commit tx")
	}

	if user.Balance != int64(balance) {
		testutil.FatalMsgf(t, "wrong balance, expected [%d], got [%d]", balance,
			user.Balance)
	}

	return user
}
