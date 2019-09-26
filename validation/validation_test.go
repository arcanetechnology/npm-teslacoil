package validation

import (
	"os"
	"testing"

	"github.com/brianvoe/gofakeit"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gopkg.in/go-playground/validator.v8"
)

var validate *validator.Validate

func TestMain(m *testing.M) {
	build.SetLogLevel(logrus.InfoLevel)
	gofakeit.Seed(0)

	config := validator.Config{TagName: "binding"}
	validate = validator.New(&config)

	os.Exit(m.Run())
}

func TestIsValidPassword(t *testing.T) {
	type MyStruct struct {
		Password string `binding:"password"`
	}

	if err := RegisterValidator(validate, "password", IsValidPassword); err != nil {
		log.Fatal(err)
	}

	badStruct := MyStruct{Password: "bad_password"}
	goodStruct := MyStruct{Password: gofakeit.Password(true, true, true, true, true, 32)}

	t.Run("validate a good password", func(t *testing.T) {
		if err := validate.Struct(goodStruct); err != nil {
			testutil.FatalMsgf(t, "Struct %+v didn't pass validation: %v", goodStruct, err)
		}
	})

	t.Run("invalidate a bad password", func(t *testing.T) {
		if validate.Struct(badStruct) == nil {
			testutil.FatalMsgf(t, "Bad struct %+v passed validation!", badStruct)
		}
	})
}
