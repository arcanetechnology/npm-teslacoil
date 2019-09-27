package testutil

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/sendgrid/rest"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

// GetTestEmail generates a random email for a given test
func GetTestEmail(t *testing.T) string {
	return fmt.Sprintf("%d-%s@example.com", rand.Int(), t.Name())
}

type MockSendGridClient struct {
	SentEmails int
}

// GetMockSendGridClient returns a SendGrid client that can be used for testing
func GetMockSendGridClient() *MockSendGridClient {
	return &MockSendGridClient{}
}

func (mock *MockSendGridClient) Send(email *mail.SGMailV3) (*rest.Response, error) {
	mock.SentEmails += 1
	return &rest.Response{
		StatusCode: 202,
		Body:       "",
		Headers:    nil,
	}, nil
}
