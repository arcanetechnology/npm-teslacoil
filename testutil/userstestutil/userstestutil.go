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
