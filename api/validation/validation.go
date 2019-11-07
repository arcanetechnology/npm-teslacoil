// Package validation provides validation functionality for struct tag
// fields such as "binding", used in Gin/Validator.
package validation

import (
	"encoding/base64"
	"reflect"

	"github.com/btcsuite/btcutil"

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
const (
	requiredValidationScore = 3
	password                = "password"
	paymentrequest          = "paymentrequest"
	urlbase64               = "urlbase64"
	address                 = "address"
)

// isValidPassword checks if a password is strong enough.
func isValidPassword(
	_ *validator.Validate, _ reflect.Value, _ reflect.Value,
	field reflect.Value, _ reflect.Type, _ reflect.Kind, _ string) bool {
	stringVal := field.String()

	// custom wordlist that password is checked against
	// TODO: could put username, bitcoin terms etc into here?
	var inputs []string

	strength := zxcvbn.PasswordStrength(stringVal, inputs)
	return strength.Score >= requiredValidationScore
}

// isValidPaymentRequest checks if a payment request is valid per the configured network
func isValidPaymentRequest(chainCfg *chaincfg.Params) validator.Func {
	return func(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value,
		field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {

		stringVal := field.String()

		// if tag is payreq, check that the value is decodable
		if _, err := zpay32.Decode(stringVal, chainCfg); err != nil {
			return false
		}

		return true
	}
}

// isValidBitcoinAddress checks if a payment request is valid per the configured network
func isValidBitcoinAddress(chainCfg *chaincfg.Params) validator.Func {
	return func(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value,
		field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {

		stringVal := field.String()

		// assert address is valid by attempting to decode it
		addr, err := btcutil.DecodeAddress(stringVal, chainCfg)
		if err != nil {
			log.WithError(err).Errorf("could not decode %s", stringVal)
			return false
		}

		if !addr.IsForNet(chainCfg) {
			return false
		}

		return true
	}
}

func isValidUrlBase64(
	_ *validator.Validate, _ reflect.Value, _ reflect.Value,
	field reflect.Value, _ reflect.Type, _ reflect.Kind, _ string) bool {
	stringVal := field.String()
	_, err := base64.URLEncoding.DecodeString(stringVal)
	return err == nil
}

// registerValidator registers a validator in our validation engine with the
// given name.
func registerValidator(engine *validator.Validate, name string, function validator.Func) error {
	err := engine.RegisterValidation(name, function)
	if err != nil {
		return errors.Wrapf(err, "could not register %q validation", name)
	}
	return nil
}

// RegisterAllValidators registers all known validators to the Validator engine,
// quitting if this results in an error. This function should typically be
// called at startup.
func RegisterAllValidators(engine *validator.Validate, chainCfg *chaincfg.Params) []string {
	type Validator struct {
		Name     string
		Function validator.Func
	}
	validators := []Validator{
		{
			Name:     password,
			Function: isValidPassword,
		},
		{
			Name:     paymentrequest,
			Function: isValidPaymentRequest(chainCfg),
		},
		{
			Name:     urlbase64,
			Function: isValidUrlBase64,
		},
		{
			Name:     address,
			Function: isValidBitcoinAddress(chainCfg),
		},
	}
	names := make([]string, len(validators))
	for i, elem := range validators {
		names[i] = elem.Name
		if err := registerValidator(engine, elem.Name, elem.Function); err != nil {
			log.Fatalf("Fatal error during validation registration: %s", err)
		}
	}
	return names
}
