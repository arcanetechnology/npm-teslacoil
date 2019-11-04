// package apierr provides functionality for handling errors in our API.
// This includes both creating middleware for this, as well as terminating
// requests in a way that ensure a smooth user experience.

package apierr

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"unicode"

	"github.com/gin-gonic/gin"
	pkgerrors "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/api/httptypes"
	"gitlab.com/arcanecrypto/teslacoil/models/transactions"
	"gopkg.in/go-playground/validator.v8"
)

// apiError is a type we can pass in to the Public method of this package.
// It ensure we're both giving a unique error code and a meaningful error
// message.
type apiError struct {
	err  error
	code string
}

func (a apiError) Error() string {
	return pkgerrors.Wrap(a.err, a.code).Error()
}

// Is provides functionality for comparing errors
func (a apiError) Is(err error) bool {
	if stdErr, ok := err.(httptypes.StandardErrorResponse); ok {
		return stdErr.ErrorField.Code == a.code
	}
	if aErr, ok := err.(apiError); ok {
		return a.code == aErr.code
	}
	return a.err.Error() == err.Error()
}

var (
	// ErrEmailNotVerified means the user has signed up bot not verified their email
	ErrEmailNotVerified = apiError{
		err:  errors.New("email is not verified"),
		code: "ERR_EMAIL_NOT_VERIFIED",
	}

	// ErrUserAlreadyExists means the user already exists
	ErrUserAlreadyExists = apiError{
		err:  errors.New("user already exists"),
		code: "ERR_USER_ALREADY_EXISTS",
	}
	// ErrNoSuchUser means no user with the given email and password exists
	ErrNoSuchUser = apiError{
		err:  errors.New("no user with the given email and password exists"),
		code: "ERR_NO_SUCH_USER",
	}

	// ErrBalanceTooLow means the user tried to spend or withdraw more money than
	// they had available
	ErrBalanceTooLow = apiError{
		err:  transactions.ErrBalanceTooLow,
		code: "ERR_BALANCE_TOO_LOW",
	}

	// errInvalidJson means we got sent invalid JSON
	errInvalidJson = apiError{
		err:  errors.New("invalid JSON"),
		code: "ERR_INVALID_JSON",
	}

	errBodyRequired = apiError{
		err:  errors.New("JSON body required"),
		code: "ERR_BODY_REQUIRED",
	}

	// ErrUnknownError means we don't know exactly what went wrong
	ErrUnknownError = apiError{
		err:  errors.New("something went wrong"),
		code: "ERR_UNKNOWN_ERROR",
	}

	// ErrRouteNotFound means the requested HTTP route wasn't found
	ErrRouteNotFound = apiError{
		err:  errors.New("route not found"),
		code: "ERR_ROUTE_NOT_FOUND",
	}

	// ErrMissingAuthHeader means the HTTP request had an empty auth header
	ErrMissingAuthHeader = apiError{
		err:  errors.New("missing authentication header"),
		code: "ERR_MISSING_AUTH_HEADER",
	}

	//ErrIncorrectPassword means we were given an invalid password
	ErrIncorrectPassword = apiError{
		err:  errors.New("incorrect password"),
		code: "ERR_INCORRECT_PASSWORD",
	}

	//Err2faNotEnabled means the user hasn't enabled 2FA
	Err2faNotEnabled = apiError{
		err:  errors.New("2FA is not enabled"),
		code: "ERR_2FA_NOT_ENABLED",
	}

	//Err2faAlreadyEnabled means the user already has enabled 2FA
	Err2faAlreadyEnabled = apiError{
		err:  errors.New("2FA is already enabled"),
		code: "ERR_2FA_ALREADY_ENABLED",
	}

	// ErrInvalidTotpCode means the given TOTP code was not on a valid format
	ErrInvalidTotpCode = apiError{
		err:  errors.New("invalid TOTP code format"),
		code: "ERR_INVALID_TOTP_CODE",
	}
	//ErrBadRequest means we got a malformed request
	ErrBadRequest = apiError{
		err:  errors.New("bad request"),
		code: "ERR_BAD_REQUEST",
	}
	//ErrMalformedApiKey means we received a malformed API key
	ErrMalformedApiKey = apiError{
		err:  errors.New("malformed API key"),
		code: "ERR_MALFORMED_API_KEY",
	}
	//ErrApiKeyNotFound means the given API key was not found
	ErrApiKeyNotFound = apiError{
		err:  errors.New("API key not found"),
		code: "ERR_API_KEY_NOT_FOUND",
	}
	// ErrBadApiKey means the given API key did not have the correct permissions
	ErrBadApiKey = apiError{
		err:  errors.New("the given API key does not have the correct permissions"),
		code: "ERR_BAD_API_KEY",
	}
	//ErrMalformedJwt means the given JWT was malformed
	ErrMalformedJwt = apiError{
		err:  errors.New("malformed JWT"),
		code: "ERR_MALFORMED_JWT",
	}
	//ErrInvalidJwtSignature means the JWT signature was invalid
	ErrInvalidJwtSignature = apiError{
		err:  errors.New("invalid JWT signature"),
		code: "ERR_INVALID_JWT_SIGNATURE",
	}
	//ErrExpiredJwt means we were given an expired JWT
	ErrExpiredJwt = apiError{
		err:  errors.New("expired JWT"),
		code: "ERR_EXPIRED_JWT",
	}
	//ErrJwtNotValidYet means the given JWT has a start time set in the future
	ErrJwtNotValidYet = apiError{
		err:  errors.New("JWT is not valid yet"),
		code: "ERR_JWT_NOT_VALID_YET",
	}
	//ErrMissingTotpCode means no TOTP code was given where one was required
	ErrMissingTotpCode = apiError{
		err:  errors.New("missing TOTP code"),
		code: "ERR_MISSING_TOTP_CODE",
	}

	// ErrBadTotpCode means the given TOTP code did not match up with the expected one
	ErrBadTotpCode = apiError{
		err:  errors.New("bad TOTP code"),
		code: "ERR_BAD_TOTP_CODE",
	}

	// ErrRequestValidationFailed means the user gave us an invalid request, either
	// in JSON, URL or query format
	ErrRequestValidationFailed = apiError{
		err:  errors.New("request validation failed"),
		code: "ERR_REQUEST_VALIDATION_FAILED",
	}
	//ErrInvoiceNotFound means the requested invoice was not found
	ErrInvoiceNotFound = apiError{
		err:  errors.New("invoice not found"),
		code: "ERR_INVOICE_NOT_FOUND",
	}
	//ErrTransactionNotFound means the requested transaction was not found
	ErrTransactionNotFound = apiError{
		err:  errors.New("transaction not found"),
		code: "ERR_TRANSACTION_NOT_FOUND",
	}
)

// decapitalize makes the first element of a string lowercase
func decapitalize(str string) string {
	if str == "" {
		return ""
	}
	var decapitalized string
	for index, c := range str {
		if index == 0 {
			decapitalized = string(unicode.ToLower(c))
			continue
		}
		decapitalized = decapitalized + string(c)
	}
	return decapitalized
}

// capitalize makes the first element of a string uppercase
func capitalize(str string) string {
	if str == "" {
		return ""
	}
	var capitalized string
	for index, c := range str {
		if index == 0 {
			capitalized = string(unicode.ToUpper(c))
			continue
		}
		capitalized = capitalized + string(c)
	}
	return capitalized
}

// GetMiddleware returns a Gin middleware that handles errors
func GetMiddleware(log *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {

		// let previous handlers run
		c.Next()

		if len(c.Errors) == 0 {
			return
		}

		// if HTTP code is set to -1 it doesn't overwrite what's already there
		httpCode := -1
		if c.Writer.Status() == http.StatusOK {
			// default to 500 if no status has been set
			httpCode = http.StatusInternalServerError
		}

		fieldErrors := handleValidationErrors(c, log)
		response := &httptypes.StandardErrorResponse{
			ErrorField: httptypes.StandardError{
				Fields: fieldErrors,
			},
		}

		// Check for JSON parsing errors
		for _, err := range c.Errors {
			var syntaxErr *json.SyntaxError
			if errors.Is(err.Err, io.EOF) {
				response.ErrorField.Code = errBodyRequired.code
				response.ErrorField.Message = errBodyRequired.err.Error()
				c.JSON(http.StatusBadRequest, response)
				return
			} else if errors.As(err.Err, &syntaxErr) {
				response.ErrorField.Code = errInvalidJson.code
				response.ErrorField.Message = errInvalidJson.err.Error()
				c.JSON(http.StatusBadRequest, response)
				return
			}
		}

		// public errors are errors that can be shown to the end user
		publicErrors := c.Errors.ByType(gin.ErrorTypePublic)
		if len(publicErrors) > 0 {
			// we only take the last one because our error format only has space for one error.
			// as of writing, we immediately return from all places where we send a public error,
			// so this shouldn't really matter
			err := publicErrors.Last()
			if apiErr, ok := err.Err.(apiError); ok {
				response.ErrorField.Code = apiErr.code
				response.ErrorField.Message = apiErr.err.Error()
			} else {
				log.WithError(err).Warn("Got public error in error handler that was not apiError type")
				response.ErrorField.Code = ErrUnknownError.code
				response.ErrorField.Message = ErrUnknownError.err.Error()
			}
		}

		// ensure all responses have a code
		if response.ErrorField.Code == "" {
			if len(fieldErrors) > 0 {
				// if we have any field errors, request validation failed
				response.ErrorField.Code = ErrRequestValidationFailed.code
				response.ErrorField.Message = ErrRequestValidationFailed.err.Error()
			} else {
				// this is bad, but should be picked up by tests
				response.ErrorField.Code = ErrUnknownError.code
				response.ErrorField.Message = ErrUnknownError.err.Error()
			}
		}

		response.ErrorField.Message = capitalize(response.ErrorField.Message)
		c.JSON(httpCode, response)
	}
}

// Public fails the given Gin request with the given error. It sets the error
// type as public, causing it to later be returned to the end user with a
// fitting error message.
func Public(c *gin.Context, code int, err apiError) {
	cErr := c.AbortWithError(code, err)
	_ = cErr.SetType(gin.ErrorTypePublic)
}

// UnknownValidationTag is the tag we apply when encountering a validation tag
// we don't know how to handle
const UnknownValidationTag = "unknown"

func handleValidationErrors(c *gin.Context, log *logrus.Logger) []httptypes.FieldError {
	// initialize to empty list instead of pointer, to make sure the empty list
	// is returned instead of nil
	//noinspection GoPreferNilSlice
	fieldErrors := []httptypes.FieldError{}
	for _, err := range c.Errors.ByType(gin.ErrorTypeBind) {
		// not all errors encountered in validation is a nice validator.ValidationErrors type
		// if you request an int in a form for example, parsing of that int will fail before
		// proper validation happens, and we're left with this ugly error type.
		// see these GitHub issues:  https://github.com/gin-gonic/gin/issues/1093
		//							 https://github.com/gin-gonic/gin/issues/1907
		if numError, ok := err.Err.(*strconv.NumError); ok {
			fieldErrors = append(fieldErrors, httptypes.FieldError{
				// don't know how to find out which field failed here...
				Field:   "unknown",
				Message: fmt.Sprintf("%q is not a valid number, %q failed", numError.Num, numError.Func),
				Code:    "invalid-number",
			})
			continue
		}

		// if we pass an int to a JSON field expecting a string (or something similar),
		// we end up with this kind of error, not a validator.ValidationErrors
		if jsonError, ok := err.Err.(*json.UnmarshalTypeError); ok {
			log.WithError(jsonError).WithFields(logrus.Fields{
				"field":  jsonError.Field,
				"value":  jsonError.Value,
				"type":   jsonError.Type,
				"struct": jsonError.Struct,
			}).Debug("Handling JSON error")
			fieldErrors = append(fieldErrors, httptypes.FieldError{
				Field:   jsonError.Field,
				Message: fmt.Sprintf("%q requires a %s, got a %s", jsonError.Field, jsonError.Type, jsonError.Value),
				Code:    "invalid-type",
			})
			continue
		}

		validationErrors, ok := err.Err.(validator.ValidationErrors)
		if !ok {
			continue
		}
		for _, validationErr := range validationErrors {
			// When doing field validation, it's not possible to get the name of
			// the JSON/Query field we're validating, only the field of the struct.
			// The assumption here is that all struct fields are named the same
			// as corresponding form/JSON fields, except for the first letter.
			field := decapitalize(validationErr.Field)
			var message string
			var code string
			switch validationErr.Tag {
			case "required":
				message = fmt.Sprintf("%q is required", field)
				code = "required"
			case "password":
				message = fmt.Sprintf("%q field does not contain a valid password", field)
				code = "password"
			case "paymentrequest":
				message = fmt.Sprintf("%q is not a valid payment request", field)
				code = "paymentrequest"
			case "email":
				message = fmt.Sprintf("%q field does not contain a valid email", field)
				code = "email"
			case "gte":
				message = fmt.Sprintf("%q field must be greater than or equal %s. Got: %s",
					field, validationErr.Param, validationErr.Value)
				code = "gte"
			case "lte":
				message = fmt.Sprintf("%q field must be less than or equal %s. Got: %s",
					field, validationErr.Param, validationErr.Value)
				code = "gte"
			case "gt":
				message = fmt.Sprintf("%q field must be greater than %s. Got: %s",
					field, validationErr.Param, validationErr.Value)
				code = "gt"
			case "url":
				message = fmt.Sprintf("%q field is not a valid URL", field)
				code = "url"
			case "eqfield":
				message = fmt.Sprintf("%q must the equal to %q", field,
					// see comment above on capitalization of fields
					decapitalize(validationErr.Param))
				code = "eqfield"
			case "max":
				message = fmt.Sprintf("%q cannot be longer than %s characters", field, validationErr.Param)
				code = "max"
			default:
				log.WithField("tag", validationErr.Tag).Warn("Encountered unknown validation field")
				message = fmt.Sprintf("%s is invalid", field)
				code = UnknownValidationTag
			}
			fieldErrors = append(fieldErrors, httptypes.FieldError{
				Field:   field,
				Message: message,
				Code:    code,
			})
		}
	}
	return fieldErrors
}
