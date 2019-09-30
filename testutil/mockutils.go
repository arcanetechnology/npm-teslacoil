package testutil

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"testing"

	"github.com/sendgrid/rest"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

// GetTestEmail generates a random email for a given test
func GetTestEmail(t *testing.T) string {
	return fmt.Sprintf("%d-%s@example.com", rand.Int(), t.Name())
}

type mockSendGridClient struct {
	sentEmails int
}

// GetMockSendGridClient returns a SendGrid client that can be used for testing
func GetMockSendGridClient() *mockSendGridClient {
	return &mockSendGridClient{}
}

func (mock *mockSendGridClient) Send(email *mail.SGMailV3) (*rest.Response, error) {
	mock.sentEmails += 1
	return &rest.Response{
		StatusCode: 202,
		Body:       "",
		Headers:    nil,
	}, nil
}

func (mock *mockSendGridClient) GetSentEmails() int {
	return mock.sentEmails
}

type mockHttpPoster struct {
	sentPostRequests int
}

func GetMockHttpPoster() *mockHttpPoster {
	return &mockHttpPoster{}
}

func (m *mockHttpPoster) Post(url, contentType string, reader io.Reader) (*http.Response, error) {
	m.sentPostRequests += 1
	return &http.Response{
		StatusCode: 200,
	}, nil
}

func (m *mockHttpPoster) GetSentPostRequests() int {
	return m.sentPostRequests
}
