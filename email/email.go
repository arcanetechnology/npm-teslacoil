package email

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/models/users"

	"github.com/sendgrid/rest"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/sirupsen/logrus"
)

var log = build.AddSubLogger("EMAL")

// ErrCouldNotSendEmail means the HTTP request to send an email did not get a
// success status code.
var ErrCouldNotSendEmail = errors.New("could not send email")

// Sender can handle the various email sending needs of our API
type Sender interface {
	SendPasswordReset(user users.User, token string) error
	SendEmailVerification(user users.User, token string) error
}

var _ Sender = SendGridSender{}

// NewSendGridSender creates a new SendGrid email sender
func NewSendGridSender(baseUrl, key string) SendGridSender {
	log.WithField("baseUrl", baseUrl).Info("Creating new SendGrid email sender")
	return SendGridSender{
		client:  sendgrid.NewSendClient(key),
		baseUrl: baseUrl,
	}
}

// SendGridSender can send emails by communicating with the SendGrid API
type SendGridSender struct {
	client  *sendgrid.Client
	baseUrl string
}

// SendEmailVerification sends out an email where the user can verify their email
func (s SendGridSender) SendEmailVerification(user users.User, token string) error {
	from := mail.NewEmail("Teslacoil", "noreply@teslacoil.io")
	const subject = "Verify email"
	to := getRecipient(user)

	verifyEmailUrl := s.getVerifyEmailUrl(token)
	htmlText := fmt.Sprintf(
		`<p>You have recently signed up to Teslacoil. Go to <a href="%s">%s</a> to verify your email and complete the process.</p>`,
		verifyEmailUrl, verifyEmailUrl)
	plainText := fmt.Sprintf(
		`You have recently signed up to Teslacoil. Go to %s to verify your email and complete the process.`,
		verifyEmailUrl)
	message := mail.NewSingleEmail(from, subject, to, plainText, htmlText)
	log.WithFields(logrus.Fields{
		"recipient": to.Address,
	}).Info("Sending email verification email")

	response, err := s.send(message)
	if err != nil {
		return err
	}

	log.WithFields(logrus.Fields{
		"recipient": to.Address,
		"status":    response.StatusCode,
	}).Info("Sent email verification email successfully")
	return nil

}

// SendPasswordReset sends out a email where a user can reset their password
func (s SendGridSender) SendPasswordReset(user users.User, token string) error {
	from := mail.NewEmail("Teslacoil", "noreply@teslacoil.io")
	const subject = "Password reset"
	to := getRecipient(user)

	resetPasswordUrl := s.getResetPasswordUrl(token)
	htmlText := fmt.Sprintf(
		`<p>You have requested a password reset. Go to <a href="%s">%s</a> to complete this process.</p>`,
		resetPasswordUrl, resetPasswordUrl)
	plainTextContent := fmt.Sprintf(
		`You have requested a password reset. Go to %s to complete this process.`,
		resetPasswordUrl)

	message := mail.NewSingleEmail(from, subject, to, plainTextContent, htmlText)
	log.WithFields(logrus.Fields{
		"recipient": to.Address,
	}).Info("Sending password reset email")

	response, err := s.send(message)
	if err != nil {
		return err
	}

	log.WithFields(logrus.Fields{
		"recipient": to.Address,
		"status":    response.StatusCode,
	}).Info("Sent password reset email successfully")
	return nil

}

// send sends the given email. It expects a single recipient
func (s SendGridSender) send(email *mail.SGMailV3) (*rest.Response, error) {
	recipient := "Unknown recipient"
	if len(email.Personalizations) != 0 && len(email.Personalizations[0].To) != 0 {
		recipient = email.Personalizations[0].To[0].Address
	} else {
		log.WithField("personalizations",
			email.Personalizations).Warn("Unexpected recipient format when sending email")
	}

	response, err := s.client.Send(email)
	if err != nil {
		log.WithError(err).WithField("recipient", recipient).Error("Could not send email")
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		log.WithFields(logrus.Fields{
			"recipient": recipient,
			"status":    response.StatusCode,
			"body":      response.Body,
		}).Error("Got error status when sending email")
		return nil, fmt.Errorf("%w: %s", ErrCouldNotSendEmail, response.Body)
	}
	log.WithFields(logrus.Fields{}).Info("Sent email successfully")

	return response, nil
}

func (s SendGridSender) getResetPasswordUrl(token string) string {
	return fmt.Sprintf("%s/reset-password?token=%s", s.baseUrl, url.QueryEscape(token))
}

func (s SendGridSender) getVerifyEmailUrl(token string) string {
	return fmt.Sprintf("%s/verify-email?token=%s", s.baseUrl, url.QueryEscape(token))
}

func getRecipient(user users.User) *mail.Email {
	var recipientName string
	var names []string
	if user.Firstname != nil {
		names = append(names, *user.Firstname)
	}
	if user.Lastname != nil {
		names = append(names, *user.Lastname)
	}
	if len(names) == 0 {
		recipientName = user.Email
	} else {
		recipientName = strings.Join(names, " ")
	}

	return mail.NewEmail(recipientName, user.Email)
}
