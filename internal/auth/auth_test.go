package auth

import (
	"os"
	"testing"

	"github.com/brianvoe/gofakeit"
	"github.com/dgrijalva/jwt-go"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
)

func TestMain(m *testing.M) {
	gofakeit.Seed(0)
	os.Exit(m.Run())
}

func TestCreateJWT(t *testing.T) {
	email := gofakeit.Email()
	id := gofakeit.Number(
		0,
		1000000,
	)
	jwt, err := CreateJwt(email, id)
	if err != nil {
		testutil.FatalMsg(t, err)
	}

	token, claims, err := ParseBearerJwt(jwt)
	if err != nil {
		testutil.FatalMsg(t, err)
	}
	testutil.AssertMsg(t, token.Valid, "Token was invalid")
	testutil.AssertEqual(t, claims.UserID, id)
	testutil.AssertEqual(t, claims.Email, email)
}

func TestParseBearerJwt(t *testing.T) {
	email := gofakeit.Email()
	id := gofakeit.Number(
		0,
		1000000,
	)

	t.Run("creating a JWT with a bad key should not parse", func(t *testing.T) {
		token, err := createJwtWithKey(email, id, []byte("bad key"))
		if err != nil {
			testutil.FatalMsg(t, err)
		}
		_, _, err = ParseBearerJwt(token)
		testutil.AssertEqual(t, err, jwt.ErrSignatureInvalid)
	})

	t.Run("Parsing a JWT with a bad key should not work", func(t *testing.T) {
		token, err := CreateJwt(email, id)
		if err != nil {
			testutil.FatalMsg(t, err)
		}

		_, _, err = parseBearerJwtWithKey(token, []byte("another bad key"))
		testutil.AssertEqual(t, err, jwt.ErrSignatureInvalid)
	})
}
