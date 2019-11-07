package validation

import (
	"encoding/base64"
	"os"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/arcanecrypto/teslacoil/testutil/txtest"

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

func TestIsValidUrlBase64(t *testing.T) {
	require.NoError(t, registerValidator(validate, urlbase64, isValidUrlBase64))

	type Struct struct {
		Base64 string `binding:"urlbase64"`
	}

	goodStruct := Struct{Base64: base64.URLEncoding.EncodeToString(txtest.MockPreimage())}
	assert.NoError(t, validate.Struct(goodStruct))

	badStruct := Struct{Base64: "this should not validate"}
	assert.Error(t, validate.Struct(badStruct))
}

func TestIsValidPassword(t *testing.T) {

	if err := registerValidator(validate, password, isValidPassword); err != nil {
		log.Fatal(err)
	}

	type Struct struct {
		Password string `binding:"password"`
	}

	goodStruct := Struct{Password: gofakeit.Password(true, true, true, true, true, 32)}
	t.Run("validate a good password", func(t *testing.T) {
		if err := validate.Struct(goodStruct); err != nil {
			testutil.FatalMsgf(t, "struct %+v didn't pass validation: %v", goodStruct, err)
		}
	})

	badStruct := Struct{Password: "bad_password"}
	t.Run("invalidate a bad password", func(t *testing.T) {
		if validate.Struct(badStruct) == nil {
			testutil.FatalMsgf(t, "bad struct %+v passed validation", badStruct)
		}
	})

}

func TestIsValidPaymentRequest(t *testing.T) {

	if err := registerValidator(validate, paymentrequest, isValidPaymentRequest(&chaincfg.RegressionNetParams)); err != nil {
		log.Fatal(err)
	}
	type Struct struct {
		PaymentRequest string `binding:"paymentrequest"`
	}

	goodPaymentRequest := Struct{PaymentRequest: "lnbcrt500u1pw6gmx6pp5lnv93hd3vzxhu2zt4rfk8tdtrsweul45x32zchmd44gdvx7a8edsdqqcqzpgazxk578m8w2uccc3fka4nvk6ugv7g3fcj2j74vpwksvac4tysg6kkszhk5cwdh5qwtp0ay5s7ukm782z077glqh7p8w0j0zwvwsjj9gq0lumug"}
	t.Run("validate a good payment request", func(t *testing.T) {
		if err := validate.Struct(goodPaymentRequest); err != nil {
			testutil.FatalMsgf(t, "good struct %+v did not pass validation: %v", goodPaymentRequest, err)
		}
	})

	badPaymentRequest := Struct{PaymentRequest: "bad_payment_request"}
	t.Run("invalidate a bad payment request", func(t *testing.T) {
		if validate.Struct(badPaymentRequest) == nil {
			testutil.FatalMsgf(t, "bad struct %+v passed validation", badPaymentRequest)
		}
	})

}

func TestIsValidBitcoinAddress(t *testing.T) {

	if err := registerValidator(validate, address, isValidBitcoinAddress(&chaincfg.RegressionNetParams)); err != nil {
		log.Fatal(err)
	}
	type Struct struct {
		Address string `binding:"address"`
	}

	// address for regtest
	goodAddress := Struct{Address: "bcrt1qu6zvu2uxfmac6xyzq9zn5r70ke92w7ndrfme4t"}
	t.Run("validate a good address", func(t *testing.T) {
		if err := validate.Struct(goodAddress); err != nil {
			testutil.FatalMsgf(t, "good struct %+v did not pass validation: %v", goodAddress, err)
		}
	})

	badAddress := Struct{Address: "bad_address"}
	t.Run("invalidate a bad address", func(t *testing.T) {
		if validate.Struct(badAddress) == nil {
			testutil.FatalMsgf(t, "bad struct %+v passed validation", badAddress)
		}
	})

	// address for mainnet, as chainCfg.RegressionNetParams identify as testnet
	wrongNetworkAddress := Struct{Address: "39qSSDqoBcGpQfFALNxozB9JQKv66tjjDy"}
	t.Run("invalidate address for the wrong network", func(t *testing.T) {
		if validate.Struct(wrongNetworkAddress) == nil {
			testutil.FatalMsgf(t, "bad struct %+v passed validation", wrongNetworkAddress)
		}
	})

}
