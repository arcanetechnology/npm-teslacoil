package userstestutil

import (
	"testing"

	"github.com/brianvoe/gofakeit"
	"github.com/stretchr/testify/require"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/models/apikeys"
	"gitlab.com/arcanecrypto/teslacoil/models/transactions"
	"gitlab.com/arcanecrypto/teslacoil/models/users"
)

// CreateUserOrFail creates a user with a random email and password. The
// method also verifies the users email and adds an API key.
func CreateUserOrFail(t *testing.T, db *db.DB) users.User {
	passwordLen := gofakeit.Number(8, 32)
	password := gofakeit.Password(true, true, true, true, true, passwordLen)
	return CreateUserOrFailWithPassword(t, db, password)
}

// CreateUserOrFailWithPassword creates a user with a random email and the
// given password. The method also verifies the users email and adds an API key.
func CreateUserOrFailWithPassword(t *testing.T, db *db.DB, password string) users.User {
	u, err := users.Create(db, users.CreateUserArgs{
		Email:    gofakeit.Email(),
		Password: password,
	})
	require.NoError(t, err)

	token, err := users.GetEmailVerificationToken(db, u.Email)
	require.NoError(t, err)

	verified, err := users.VerifyEmail(db, token)
	require.NoError(t, err)

	_, _, err = apikeys.New(db, u.ID)
	require.NoError(t, err)

	return verified
}

// CreateUserWithBalanceOrFail creates a user with an initial balance
func CreateUserWithBalanceOrFail(t *testing.T, db *db.DB, balance int) users.User {
	u := CreateUserOrFail(t, db)

	settled := gofakeit.Date()
	offchain := transactions.Offchain{
		UserID:     u.ID,
		AmountMSat: int64(balance) * 1000,
		Direction:  transactions.INBOUND,
		Expiry:     1337,
		Status:     transactions.SUCCEEDED,
		SettledAt:  &settled,
	}
	_, err := transactions.InsertOffchain(db, offchain)
	require.NoError(t, err)

	return u
}
