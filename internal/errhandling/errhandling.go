package errhandling

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/internal/httptypes"
	"gopkg.in/go-playground/validator.v8"
)

const (
	// ErrInvalidJson means we got sent invalid JSON
	ErrInvalidJson = "ERR_INVALID_JSON"

	// ErrUnknownError means we don't know exactly what went wrong
	ErrUnknownError = "ERR_UNKNOWN_ERROR"

	// ErrRouteNotFound means the requested HTTP route wasn't found
	ErrRouteNotFound = "ERR_ROUTE_NOT_FOUND"

	// ErrMissingAuthHeader means the HTTP request had an empty auth header
	ErrMissingAuthHeader = "ERR_MISSING_AUTH_HEADER"

	ErrIncorrectPassword = "ERR_INCORRECT_PASSWORD"

	Err2faNotEnabled     = "ERR_2FA_NOT_ENABLED"
	Err2faAlreadyEnabled = "ERR_2FA_ALREADY_ENABLED"

	// The given TOTP code was not on a valid format
	ErrInvalidTotpCode     = "ERR_INVALID_TOTP_CODE"
	ErrBadRequest          = "ERR_BAD_REQUEST"
	ErrMalformedApiKey     = "ERR_MALFORMED_API_KEY"
	ErrApiKeyNotFound      = "ERR_API_KEY_NOT_FOUND"
	ErrMalformedJwt        = "ERR_MALFORMED_JWT"
	ErrInvalidJwtSignature = "ERR_INVALID_JWT_SIGNATURE"
	ErrExpiredJwt          = "ERR_EXPIRED_JWT"
	ErrJwtNotValidYet      = "ERR_JWT_NOT_VALID_YET"
	ErrMissingTotpCode     = "ERR_MISSING_TOTP_CODE"

	// The given TOTP code did not match up with the expected one
	ErrBadTotpCode = "ERR_BAD_TOTP_CODE"

	ErrRequestValidationFailed = "ERR_REQUEST_VALIDATION_FAILED"
	ErrInvoiceNotFound         = "ERR_INVOICE_NOT_FOUND"
	ErrTransactionNotFound     = "ERR_TRANSACTION_NOT_FOUND"
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
		var response httptypes.StandardResponse
		response.Error = &httptypes.StandardError{
			Fields: fieldErrors,
		}

		// Check for JSON parsing errors
		for _, err := range c.Errors {
			var syntaxErr *json.SyntaxError
			if errors.Is(err.Err, io.EOF) || errors.As(err.Err, &syntaxErr) {
				response.Error.Code = ErrInvalidJson
				response.Error.Message = "Not valid JSON"
				c.JSON(httpCode, response)
				return
			}
		}

		// public errors are errors that can be shown to the end user
		publicErrors := c.Errors.ByType(gin.ErrorTypePublic)
		if len(publicErrors) > 0 {
			// we only take the last one
			err := publicErrors.Last()
			response.Error.Message = err.Err.Error()
			if metaString, ok := err.Meta.(string); ok && metaString != "" {
				response.Error.Code = metaString
			} else {
				// TODO log this
				response.Error.Code = ErrUnknownError
			}
		}

		if response.Error.Code == "" {
			if len(fieldErrors) > 0 {
				response.Error.Code = ErrRequestValidationFailed
				response.Error.Message = "Request validation failed"
			} else {
				response.Error.Code = ErrUnknownError
				response.Error.Message = "Something bad happened..."
			}
		}

		c.JSON(httpCode, response)
	}
}

const UnknownValidationTag = "unknown"

func handleValidationErrors(c *gin.Context, log *logrus.Logger) []httptypes.FieldError {
	// initialize to empty list instead of pointer, to make sure the empty list
	// is returned instead of nil
	fieldErrors := []httptypes.FieldError{}
	for _, err := range c.Errors.ByType(gin.ErrorTypeBind) {
		if numError, ok := err.Err.(*strconv.NumError); ok {
			fieldErrors = append(fieldErrors, httptypes.FieldError{
				// don't know how to find out which field failed here...
				Field:   "unknown",
				Message: fmt.Sprintf("%q is not a valid number, %q failed", numError.Num, numError.Func),
				Code:    "invalid-number",
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
