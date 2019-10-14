// Package validation provides validation functionality for struct tag
// fields such as "binding", used in Gin/Validator.
package validation

import (
	"reflect"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/zpay32"

	"github.com/nbutton23/zxcvbn-go"
	"github.com/pkg/errors"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gopkg.in/go-playground/validator.v8"
)

var log = build.Log

// RequiredValidationScore password validation we require for a password to be
// deemed acceptable. It can be between 0 and 4, with values corresponding to
// these password cracking times:
// 0 -> less than 10^2 seconds
// 1 -> less than 10^4 seconds
// 2 -> less than 10^6 seconds
// 3 -> less than 10^8 seconds
// 4 -> more than 10^8 seconds
const RequiredValidationScore = 3

// IsValidPassword checks if a password is strong enough.
func IsValidPassword(
	v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value,
	field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	stringVal := field.String()

	// custom wordlist that password is checked against
	// TODO: could put username, bitcoin terms etc into here?
	inputs := []string{}

	strength := zxcvbn.PasswordStrength(stringVal, inputs)
	return strength.Score >= RequiredValidationScore
}

// IsValidPaymentRequest checks if a payment request is valid per the configured network
func IsValidPaymentRequest(chainCfg chaincfg.Params) validator.Func {
	return func(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value,
		field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {

		stringVal := field.String()

		// if tag is payreq, check that the value is decodable
		if _, err := zpay32.Decode(stringVal, &chainCfg); err != nil {
			return false
		}

		return true
	}
}

// RegisterValidator registers a validator in our validation engine with the
// given name.
func RegisterValidator(engine *validator.Validate, name string, function validator.Func) error {
	err := engine.RegisterValidation(name, function)
	if err != nil {
		return errors.Wrapf(err, "could not register %q validation", name)
	}
	return nil
}

// RegisterAllValidators registers all known validators to the Validator engine,
// quitting if this results in an error. This function should typically be
// called at startup.
func RegisterAllValidators(engine *validator.Validate, chainCfg chaincfg.Params) []string {
	type Validator struct {
		Name     string
		Function validator.Func
	}
	validators := []Validator{{
		Name:     "password",
		Function: IsValidPassword,
	},
		{
			Name:     "payreq",
			Function: IsValidPaymentRequest(chainCfg),
		},
	}
	names := make([]string, len(validators))
	for i, elem := range validators {
		names[i] = elem.Name
		if err := RegisterValidator(engine, elem.Name, elem.Function); err != nil {
			log.Fatalf("Fatal error during validation registration: %s", err)
		}
	}
	return names
}
