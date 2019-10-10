package httptypes

import "fmt"

// StandardResponse is the standard response type that all responses sent
// frm our API should conform to.
type StandardResponse struct {
	Result interface{}    `json:"result"`
	Error  *StandardError `json:"error"`
}

// StandardError is the standard error type that all error fields in our
// standard response should conform to.
type StandardError struct {
	Message string       `json:"message"`
	Code    string       `json:"code"`
	Fields  []FieldError `json:"fields" binding:"required"`
}

func (s StandardError) Error() string {
	return fmt.Errorf("%s: %s", s.Code, s.Message).Error()
}

// FieldError is the type for a request field validation error message.
type FieldError struct {
	Field   string `json:"field" binding:"required"`
	Message string `json:"message" binding:"required"`
	Code    string `json:"code" binding:"required"`
}

// Response returns a new StandardResponse
func Response(result interface{}) StandardResponse {
	return StandardResponse{Result: result}
}
