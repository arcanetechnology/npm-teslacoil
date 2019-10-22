package mock

import (
	"sync"

	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/email"
	"gitlab.com/arcanecrypto/teslacoil/internal/users"
)

var _ email.Sender = &mockSendGridClient{}
var log = build.Log

type mockSendGridClient struct {
	sync.Mutex
	sentPasswordMails     int
	sentVerificationMails int
}

func (mock *mockSendGridClient) SendPasswordReset(user users.User, token string) error {
	mock.Mutex.Lock()
	log.WithField("email", user.Email).Info("Sending password verification email")
	mock.sentPasswordMails += 1
	mock.Mutex.Unlock()
	return nil
}

func (mock *mockSendGridClient) SendEmailVerification(user users.User, token string) error {
	mock.Mutex.Lock()
	log.WithField("email", user.Email).Info("Sending email verification email")
	mock.sentVerificationMails += 1
	mock.Mutex.Unlock()
	return nil
}

// GetMockSendGridClient returns a SendGrid client that can be used for testing
func GetMockSendGridClient() *mockSendGridClient {
	return &mockSendGridClient{}
}

func (mock *mockSendGridClient) GetEmailVerificationMails() int {
	return mock.sentVerificationMails
}

func (mock *mockSendGridClient) GetPasswordResetMails() int {
	return mock.sentPasswordMails
}
