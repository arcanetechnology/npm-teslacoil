package testutil

import (
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

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
	sentBodies       [][]byte
}

func GetMockHttpPoster() *mockHttpPoster {
	return &mockHttpPoster{}
}

func (m *mockHttpPoster) Post(url, contentType string, reader io.Reader) (*http.Response, error) {
	m.sentPostRequests += 1

	body, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	m.sentBodies = append(m.sentBodies, body)

	return &http.Response{
		StatusCode: 200,
	}, nil
}

func (m *mockHttpPoster) GetSentPostRequests() int {
	return m.sentPostRequests
}

func (m *mockHttpPoster) GetSentPostRequest(index int) []byte {
	return m.sentBodies[index]
}

func MockTxid() string {
	var letters = []rune("abcdef1234567890")

	b := make([]rune, 64)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func MockStringOfLength(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")

	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// GetPortOrFail returns a unused port
func GetPortOrFail(t *testing.T) int {
	const minPortNumber = 1024
	const maxPortNumber = 40000
	rand.Seed(time.Now().UnixNano())
	port := rand.Intn(maxPortNumber)
	// port is reserved, try again
	if port < minPortNumber {
		return GetPortOrFail(t)
	}

	listener, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	// port is busy, try again
	if err != nil {
		return GetPortOrFail(t)
	}
	if err := listener.Close(); err != nil {
		FatalMsgf(t, "Couldn't close port: %sl", err)
	}
	return port
}
