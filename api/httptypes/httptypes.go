package httptypes

import (
	"fmt"
)

type StandardError struct {
	Message string       `json:"message"`
	Code    string       `json:"code"`
	Fields  []FieldError `json:"fields" binding:"required"`
}

// StandardErrorResponse is the standard type that all error responses from our API should conform to
type StandardErrorResponse struct {
	ErrorField StandardError `json:"error"`
}

func (s StandardErrorResponse) Error() string {
	return fmt.Errorf("%s: %s", s.ErrorField.Code, s.ErrorField.Message).Error()
}

func (s StandardErrorResponse) Is(err error) bool {
	if stdErr, ok := err.(StandardErrorResponse); ok {
		return stdErr.ErrorField.Code == s.ErrorField.Code
	}
	return s.Error() == err.Error()
}

// FieldError is the type for a request field validation error message.
type FieldError struct {
	Field   string `json:"field" binding:"required"`
	Message string `json:"message" binding:"required"`
	Code    string `json:"code" binding:"required"`
}
