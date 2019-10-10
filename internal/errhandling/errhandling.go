package errhandling

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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

		c.JSON(httpCode, response)
	}
}

func handleValidationErrors(c *gin.Context, log *logrus.Logger) []httptypes.FieldError {
	// initialize to empty list instead of pointer, to make sure the empty list
	// is returned instead of nil
	fieldErrors := []httptypes.FieldError{}
	for _, err := range c.Errors.ByType(gin.ErrorTypeBind) {
		validationErrors, ok := err.Err.(validator.ValidationErrors)
		if !ok {
			continue
		}
		for _, validationErr := range validationErrors {
			field := decapitalize(validationErr.Field)
			var message string
			var code string
			switch validationErr.Tag {
			case "required":
				message = fmt.Sprintf("%s is required", field)
				code = "required"
			default:
				log.WithField("tag", validationErr.Tag).Warn("Encountered unknown validation field")
				message = fmt.Sprintf("%s is invalid", field)
				code = validationErr.Tag
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
