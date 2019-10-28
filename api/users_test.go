package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit"
	"github.com/dchest/passwordreset"
	"github.com/pquerna/otp/totp"

	"gitlab.com/arcanecrypto/teslacoil/api/apierr"
	"gitlab.com/arcanecrypto/teslacoil/async"
	"gitlab.com/arcanecrypto/teslacoil/models/users"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/httptestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/mock"
	"gitlab.com/arcanecrypto/teslacoil/testutil/userstestutil"
)

func init() {
	testDB = testutil.InitDatabase(databaseConfig)

	app, err := NewApp(testDB, mockLightningClient,
		mockSendGridClient, mockBitcoindClient,
		mockHttpPoster, conf)
	if err != nil {
		panic(err)
	}

	h = httptestutil.NewTestHarness(app.Router, testDB)
}

func TestCreateUser(t *testing.T) {
	t.Run("creating a user must fail with a bad password", func(t *testing.T) {
		testutil.DescribeTest(t)
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path: "/users", Method: "POST",
			Body: fmt.Sprintf(`{
				"password": "foobar",
				"email": %q
			}`, gofakeit.Email()),
		})
		_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})
	t.Run("Creating a user must fail without an email", func(t *testing.T) {
		testutil.DescribeTest(t)
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path: "/users", Method: "POST",
			Body: fmt.Sprintf(`{
				"password": %q
			}`, gofakeit.Password(true, true, true, true, true, 32)),
		})
		_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})

	t.Run("Creating a user must fail with an empty email", func(t *testing.T) {
		testutil.DescribeTest(t)
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path: "/users", Method: "POST",
			Body: fmt.Sprintf(`{
				"password": %q,
				"email": ""
			}`, gofakeit.Password(true, true, true, true, true, 32)),
		})
		_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})

	t.Run("creating an user with an invalid email should fail", func(t *testing.T) {
		pass := gofakeit.Password(true, true, true, true, true, 32)
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/users",
			Method: "POST",
			Body: fmt.Sprintf(`{
				"password": %q,
				"email": "this-is-not@a-valid-mail"
			}`, pass),
		})
		_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})

	t.Run("It should be possible to create a user without any names", func(t *testing.T) {
		email := gofakeit.Email()
		pass := gofakeit.Password(true, true, true, true, true, 32)
		testutil.DescribeTest(t)
		body := httptestutil.RequestArgs{
			Path: "/users", Method: "POST",
			Body: fmt.Sprintf(`{
				"password": %q,
				"email": "%s"
			}`, pass, email),
		}
		req := httptestutil.GetRequest(t, body)
		jsonRes := h.AssertResponseOkWithJson(t, req)
		testutil.AssertEqual(t, jsonRes["email"], email)
		testutil.AssertEqual(t, jsonRes["firstName"], nil)
		testutil.AssertEqual(t, jsonRes["lastName"], nil)
	})

	t.Run("It should be possible to create a user with names", func(t *testing.T) {
		email := gofakeit.Email()
		pass := gofakeit.Password(true, true, true, true, true, 32)
		firstName := gofakeit.FirstName()
		lastName := gofakeit.LastName()
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path: "/users", Method: "POST",
			Body: fmt.Sprintf(`{
				"password": %q,
				"email": "%s",
				"firstName": "%s", 
				"lastName": "%s"
			}`, pass, email, firstName, lastName),
		})
		jsonRes := h.AssertResponseOkWithJson(t, req)
		testutil.AssertEqual(t, jsonRes["email"], email)

		user := h.VerifyEmail(t, email)
		testutil.AssertEqual(t, *user.Firstname, firstName)
		testutil.AssertEqual(t, *user.Lastname, lastName)
	})
}

func TestPostUsersRoute(t *testing.T) {
	testutil.DescribeTest(t)
	t.Parallel()

	// we need a new email sender here to record the amount of emails
	// sent correctly
	postUserEmailClient := mock.GetMockSendGridClient()
	app, err := NewApp(testDB, mockLightningClient,
		postUserEmailClient, mockBitcoindClient,
		mockHttpPoster, conf)
	if err != nil {
		testutil.FatalMsg(t, err)
	}

	harness := httptestutil.NewTestHarness(app.Router, testDB)

	preCreationEmails := postUserEmailClient.GetEmailVerificationMails()
	t.Run("create a user", func(t *testing.T) {
		t.Parallel()
		email := gofakeit.Email()
		pass := gofakeit.Password(true, true, true, true, true, 32)
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/users",
			Method: "POST",
			Body: fmt.Sprintf(`{
				"email": %q,
				"password": %q
			}`, email, pass),
		})
		jsonRes := harness.AssertResponseOkWithJson(t, req)

		var postCreation int
		// emails are sent in a go routine
		if err = async.RetryNoBackoff(10, time.Millisecond*10, func() error {
			postCreation = postUserEmailClient.GetEmailVerificationMails()
			if preCreationEmails+1 != postCreation {
				return fmt.Errorf("emails sent before creating user (%d) does not match ut with emails sent after creating user (%d)", preCreationEmails, postCreation)
			}
			return nil
		}); err != nil {
			testutil.FatalMsg(t, err)
		}
		testutil.AssertEqual(t, jsonRes["firstName"], nil)
		testutil.AssertEqual(t, jsonRes["lastName"], nil)
		testutil.AssertEqual(t, jsonRes["email"], email)
		testutil.AssertEqual(t, jsonRes["balance"], 0)

		t.Run("not create the same user twice", func(t *testing.T) {
			otherReq := httptestutil.GetRequest(t, httptestutil.RequestArgs{
				Path:   "/users",
				Method: "POST",
				Body: fmt.Sprintf(`{
				"email": %q,
				"password": %q
			}`, email, pass),
			})
			_, err = harness.AssertResponseNotOk(t, otherReq)
			testutil.AssertEqual(t, apierr.ErrUserAlreadyExists, err)
			testutil.AssertEqual(t, postCreation, postUserEmailClient.GetEmailVerificationMails())
		})
	})

	t.Run("not create a user with no pass", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/users",
			Method: "POST",
			Body: fmt.Sprintf(`{
				"email": %q
			}`, gofakeit.Email()),
		})
		_, _ = harness.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})

	t.Run("not create a user with no email", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/users",
			Method: "POST",
			Body: fmt.Sprintf(`{
				"password": %q
			}`, gofakeit.Password(true, true, true, true, true, 32)),
		})
		_, _ = harness.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})
}

func TestPostLoginRoute(t *testing.T) {
	testutil.DescribeTest(t)
	t.Parallel()

	email := gofakeit.Email()
	password := gofakeit.Password(true, true, true, true, true, 32)
	first := gofakeit.FirstName()
	second := gofakeit.LastName()

	_ = h.CreateUser(t, users.CreateUserArgs{
		Email:     email,
		Password:  password,
		FirstName: &first,
		LastName:  &second,
	})

	t.Run("fail to login with invalid email", func(t *testing.T) {
		t.Parallel()
		badEmail := "foobar"
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/login",
			Method: "POST",
			Body: fmt.Sprintf(`{
			"email": %q,
			"password": %q
		}`, badEmail, password),
		})
		_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})

	t.Run("login with proper email", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/login",
			Method: "POST",
			Body: fmt.Sprintf(`{
			"email": %q,
			"password": %q
		}`, email, password),
		})
		res := h.AssertResponseOkWithJson(t, req)
		testutil.AssertEqual(t, res["firstName"], first)
		testutil.AssertEqual(t, res["lastName"], second)
		testutil.AssertEqual(t, res["email"], email)
		testutil.AssertEqual(t, res["balance"], 0.0)
	})
	t.Run("fail to login with invalid credentials", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/login",
			Method: "POST",
			Body: fmt.Sprintf(`{
			"email": %q,
			"password": %q
		}`, email, gofakeit.Password(true, true, true, true, true, 32)),
		})
		_, err := h.AssertResponseNotOkWithCode(t, req, http.StatusUnauthorized)
		testutil.AssertEqual(t, apierr.ErrNoSuchUser, err)
	})

	t.Run("fail to login with non-existant credentials", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/login",
			Method: "POST",
			Body: fmt.Sprintf(`{
			"email": %q,
			"password": %q
		}`, gofakeit.Email(), gofakeit.Password(true, true, true, true, true, 32)),
		})
		_, err := h.AssertResponseNotOkWithCode(t, req, http.StatusUnauthorized)
		testutil.AssertEqual(t, apierr.ErrNoSuchUser, err)
	})

	t.Run("fail to login with non-verified email", func(t *testing.T) {
		nonVerifiedEmail := gofakeit.Email()
		newPass := gofakeit.Password(true, true, true, true, true, 32)
		_ = h.CreateUserNoVerifyEmail(t, users.CreateUserArgs{Email: nonVerifiedEmail, Password: newPass})

		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/login",
			Method: "POST",
			Body: fmt.Sprintf(`{
			"email": %q,
			"password": %q
		}`, nonVerifiedEmail, newPass),
		})

		_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusUnauthorized)
	})
}

func TestChangePasswordRoute(t *testing.T) {
	testutil.DescribeTest(t)
	email := gofakeit.Email()
	pass := gofakeit.Password(true, true, true, true, true, 32)
	newPass := gofakeit.Password(true, true, true, true, true, 32)

	accessToken, _ := h.CreateAndAuthenticateUser(t, users.CreateUserArgs{
		Email:    email,
		Password: pass,
	})

	t.Run("Should give an error if not including the old password", func(t *testing.T) {
		changePassReq := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/auth/change_password",
			Method:      "PUT",
			Body: fmt.Sprintf(`{
			"newPassword": %q,
			"repeatedNewPassword": %q
		}`, newPass, newPass),
		})

		_, _ = h.AssertResponseNotOkWithCode(t, changePassReq, http.StatusBadRequest)
	})

	t.Run("Should give an error if not including the new password", func(t *testing.T) {
		changePassReq := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/auth/change_password",
			Method:      "PUT",
			Body: fmt.Sprintf(`{
			"oldPassword": %q,
			"repeatedNewPassword": %q
		}`, pass, newPass),
		})

		_, _ = h.AssertResponseNotOkWithCode(t, changePassReq, http.StatusBadRequest)
	})

	t.Run("Should give an error if not including the repeated password", func(t *testing.T) {
		changePassReq := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/auth/change_password",
			Method:      "PUT",
			Body: fmt.Sprintf(`{
			"oldPassword": %q,
			"newPassword": %q
		}`, pass, newPass),
		})

		_, _ = h.AssertResponseNotOkWithCode(t, changePassReq, http.StatusBadRequest)
	})

	t.Run("Should give an error if including the wrong repeated password", func(t *testing.T) {
		anotherNewPassword := gofakeit.Password(true, true, true, true, true, 32)
		changePassReq := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/auth/change_password",
			Method:      "PUT",
			Body: fmt.Sprintf(`{
			"oldPassword": %q,
			"newPassword": %q,
			"repeatedNewPassword": %q
		}`, pass, newPass, anotherNewPassword),
		})

		_, _ = h.AssertResponseNotOkWithCode(t, changePassReq, http.StatusBadRequest)
	})

	t.Run("Should give an error if not including the access token", func(t *testing.T) {
		changePassReq := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/auth/change_password",
			Method: "PUT",
			Body: fmt.Sprintf(`{
			"newPassword": %q,
			"oldPassword": %q,
			"repeatedNewPassword": %q
		}`, newPass, pass, pass),
		})

		_, _ = h.AssertResponseNotOkWithCode(t, changePassReq, http.StatusBadRequest)
	})

	t.Run("should give an error on bad password", func(t *testing.T) {
		badNewPass := "badNewPass"
		changePassReq := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/auth/change_password",
			Method:      "PUT",
			Body: fmt.Sprintf(`{
			"oldPassword": %q,
			"newPassword": %q,
			"repeatedNewPassword": %q
		}`, pass, badNewPass, badNewPass),
		})
		_, _ = h.AssertResponseNotOkWithCode(t, changePassReq, http.StatusBadRequest)
	})

	t.Run("Must be able to change password", func(t *testing.T) {
		changePassReq := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/auth/change_password",
			Method:      "PUT",
			Body: fmt.Sprintf(`{
			"oldPassword": %q,
			"newPassword": %q,
			"repeatedNewPassword": %q
		}`, pass, newPass, newPass),
		})
		h.AssertResponseOk(t, changePassReq)

		// should be possible to log in with new password
		loginReq := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/login",
			Method: "POST",
			Body: fmt.Sprintf(`{
				"email": %q,
				"password": %q
			}`, email, newPass),
		})
		h.AssertResponseOk(t, loginReq)

		// using old password should not suceed
		badLoginReq := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/login",
			Method: "POST",
			Body: fmt.Sprintf(`{
				"email": %q,
				"password": %q
			}`, email, pass),
		})
		_, _ = h.AssertResponseNotOk(t, badLoginReq)

	})

	t.Run("Must not be able to change the password by providing a bad old password", func(t *testing.T) {
		badPass := gofakeit.Password(true, true, true, true, true, 32)
		changePassReq := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/auth/change_password",
			Method:      "PUT",
			Body: fmt.Sprintf(`{
			"oldPassword": %q,
			"newPassword": %q,
			"repeatedNewPassword": %q
		}`, badPass, newPass, newPass),
		})

		_, _ = h.AssertResponseNotOkWithCode(t, changePassReq, http.StatusForbidden)
	})

}

func TestResetPasswordRoute(t *testing.T) {
	pass := gofakeit.Password(true, true, true, true, true, 32)
	newPass := gofakeit.Password(true, true, true, true, true, 32)
	user := userstestutil.CreateUserOrFailWithPassword(t, testDB, pass)

	t.Run("should not be able to reset to a weak password", func(t *testing.T) {
		badPass := "12345678"
		token, err := users.GetPasswordResetToken(testDB, user.Email)
		if err != nil {
			testutil.FatalMsgf(t, "Could not get password reset token: %v", err)
		}
		resetPassReq := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/auth/reset_password",
			Method: "PUT",
			Body: fmt.Sprintf(`{
				"token": %q,
				"password": %q
			}`, token, badPass),
		})
		_, _ = h.AssertResponseNotOkWithCode(t, resetPassReq, http.StatusBadRequest)
	})

	t.Run("Should not be able to reset the user password by using a bad token", func(t *testing.T) {
		badSecretKey := []byte("this is a secret key which we expect to fail")
		badToken := passwordreset.NewToken(user.Email, users.PasswordResetTokenDuration,
			user.HashedPassword, badSecretKey)
		badTokenReq := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/auth/reset_password",
			Method: "PUT",
			Body: fmt.Sprintf(`{
				"token": %q,
				"password": %q
			}`, badToken, newPass),
		})
		_, _ = h.AssertResponseNotOkWithCode(t, badTokenReq, http.StatusForbidden)

		// we should be able to log in with old credentials
		loginReq := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/login",
			Method: "POST",
			Body: fmt.Sprintf(`{
				"password": %q,
				"email": %q
			}`, pass, user.Email),
		})
		h.AssertResponseOk(t, loginReq)

		// we should NOT be able to log in with new credentials
		badLoginReq := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/login",
			Method: "POST",
			Body: fmt.Sprintf(`{
				"password": %q,
				"email": %q
			}`, newPass, user.Email),
		})
		_, _ = h.AssertResponseNotOk(t, badLoginReq)
	})

	t.Run("Reset the password by using the correct token", func(t *testing.T) {
		token, err := users.GetPasswordResetToken(testDB, user.Email)
		if err != nil {
			testutil.FatalMsgf(t, "Could not password reset token: %v", err)
		}
		resetPassReq := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/auth/reset_password",
			Method: "PUT",
			Body: fmt.Sprintf(`{
				"token": %q,
				"password": %q
			}`, token, newPass),
		})
		h.AssertResponseOk(t, resetPassReq)

		// we should be able to log in with new credentials
		loginReq := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/login",
			Method: "POST",
			Body: fmt.Sprintf(`{
				"password": %q,
				"email": %q
			}`, newPass, user.Email),
		})
		h.AssertResponseOk(t, loginReq)

		// we should NOT be able to log in with old credentials
		badLoginReq := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/login",
			Method: "POST",
			Body: fmt.Sprintf(`{
				"password": %q,
				"email": %q
			}`, pass, user.Email),
		})
		_, _ = h.AssertResponseNotOk(t, badLoginReq)
	})

	t.Run("Should not be able to reset the password twice", func(t *testing.T) {
		token, err := users.GetPasswordResetToken(testDB, user.Email)
		if err != nil {
			testutil.FatalMsgf(t, "Could not password reset token: %v", err)
		}
		resetPassReq := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/auth/reset_password",
			Method: "PUT",
			Body: fmt.Sprintf(`{
				"token": %q,
				"password": %q
			}`, token, newPass),
		})
		h.AssertResponseOk(t, resetPassReq)

		yetAnotherNewPass := gofakeit.Password(true, true, true, true, true, 32)
		secondResetPassReq := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/auth/reset_password",
			Method: "PUT",
			Body: fmt.Sprintf(`{
				"token": %q,
				"password": %q
			}`, token, yetAnotherNewPass),
		})
		_, _ = h.AssertResponseNotOkWithCode(t, secondResetPassReq, http.StatusForbidden)

	})
}

func TestPutUserRoute(t *testing.T) {
	testutil.DescribeTest(t)

	email := gofakeit.Email()
	password := gofakeit.Password(true, true, true, true, true, 32)
	accessToken, _ := h.CreateAndAuthenticateUser(t, users.CreateUserArgs{
		Email:    email,
		Password: password,
	})

	t.Run("updating with an invalid email should fail", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/user",
			Method:      "PUT",
			Body: `{
				"email": "bad-email.coming.through"
			}`,
		})
		_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})

	t.Run("update email, first name and last name", func(t *testing.T) {
		newFirst := "new-firstname"
		newLast := "new-lastname"
		newEmail := gofakeit.Email()

		// Update User endpoint
		updateUserReq := httptestutil.GetAuthRequest(t,
			httptestutil.AuthRequestArgs{
				AccessToken: accessToken,
				Path:        "/user", Method: "PUT",
				Body: fmt.Sprintf(`
			{
				"firstName": %q,
				"lastName": %q,
				"email": %q
			}`, newFirst, newLast, newEmail)})

		jsonRes := h.AssertResponseOkWithJson(t, updateUserReq)
		testutil.AssertEqual(t, jsonRes["firstName"], newFirst)
		testutil.AssertEqual(t, jsonRes["lastName"], newLast)
		testutil.AssertEqual(t, jsonRes["email"], newEmail)

		// Get User endpoint
		getUserReq := httptestutil.GetAuthRequest(t,
			httptestutil.AuthRequestArgs{
				AccessToken: accessToken,
				Method:      "GET", Path: "/user",
			})

		// Verify that update and get returns the same
		jsonRes = h.AssertResponseOkWithJson(t, getUserReq)
		testutil.AssertEqual(t, jsonRes["firstName"], newFirst)
		testutil.AssertEqual(t, jsonRes["lastName"], newLast)
		testutil.AssertEqual(t, jsonRes["email"], newEmail)
	})
}

func TestSendPasswordResetEmail(t *testing.T) {
	t.Parallel()

	email := gofakeit.Email()
	h.CreateAndAuthenticateUser(t, users.CreateUserArgs{
		Email:    email,
		Password: gofakeit.Password(true, true, true, true, true, 32),
	})

	req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
		Path:   "/auth/reset_password",
		Method: "POST",
		Body: fmt.Sprintf(`{
			"email": %q
		}`, email),
	})

	h.AssertResponseOk(t, req)
	testutil.AssertMsg(t, mockSendGridClient.GetPasswordResetMails() > 0,
		"Sendgrid client didn't send any emails!")

	// requesting a password reset email to a non existant user should
	// return 200 but not actually send an email
	emailsBeforeOtherReq := mockSendGridClient.GetPasswordResetMails()
	otherReq := httptestutil.GetRequest(t, httptestutil.RequestArgs{
		Path:   "/auth/reset_password",
		Method: "POST",
		Body: fmt.Sprintf(`{
			"email": %q
		}`, gofakeit.Email()),
	})
	h.AssertResponseOk(t, otherReq)
	testutil.AssertEqual(t, mockSendGridClient.GetPasswordResetMails(), emailsBeforeOtherReq)

}

func TestRestServer_SendEmailVerificationEmail(t *testing.T) {
	t.Parallel()

	// we need a new email sender here to record the amount of emails
	// sent correctly
	emailClient := mock.GetMockSendGridClient()
	app, err := NewApp(testDB, mockLightningClient,
		emailClient, mockBitcoindClient,
		mockHttpPoster, conf)
	if err != nil {
		testutil.FatalMsg(t, err)
	}

	harness := httptestutil.NewTestHarness(app.Router, testDB)

	t.Run("reject a bad email request", func(t *testing.T) {
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/user/verify_email",
			Method: "POST",
			Body: `{
			"email": "foobar"
			}`,
		})
		_, err := harness.AssertResponseNotOk(t, req)
		testutil.AssertEqual(t, apierr.ErrRequestValidationFailed, err)
	})

	// we don't want to leak information about users, so we respond with 200
	t.Run("respond with 200 for a non-existant user", func(t *testing.T) {
		email := gofakeit.Email()
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/user/verify_email",
			Method: "POST",
			Body: fmt.Sprintf(`{
			"email": %q
			}`, email),
		})

		harness.AssertResponseOk(t, req)

	})

	t.Run("Send out a password verification email for an existing user", func(t *testing.T) {
		email := gofakeit.Email()
		password := gofakeit.Password(true, true, true, true, true, 32)
		_ = h.CreateUserNoVerifyEmail(t, users.CreateUserArgs{
			Email:    email,
			Password: password,
		})

		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/user/verify_email",
			Method: "POST",
			Body: fmt.Sprintf(`{
			"email": %q
			}`, email),
		})
		emailsPreReq := emailClient.GetEmailVerificationMails()
		harness.AssertResponseOk(t, req)
		if err = async.RetryNoBackoff(5, time.Millisecond*10, func() error {
			postReq := emailClient.GetEmailVerificationMails()
			if emailsPreReq+1 != postReq {
				return fmt.Errorf("mismatch between emails pre equest (%d) and post request (%d", emailsPreReq, postReq)
			}
			return nil
		}); err != nil {
			testutil.FatalMsg(t, err)
		}
	})

	t.Run("not send out an email for already verified users", func(t *testing.T) {
		email := gofakeit.Email()
		password := gofakeit.Password(true, true, true, true, true, 32)
		_ = h.CreateUser(t, users.CreateUserArgs{
			Email:    email,
			Password: password,
		})

		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/user/verify_email",
			Method: "POST",
			Body: fmt.Sprintf(`{
			"email": %q
			}`, email),
		})
		emailsPreReq := emailClient.GetEmailVerificationMails()
		harness.AssertResponseOk(t, req)
		if err = async.RetryNoBackoff(5, time.Millisecond*10, func() error {
			postReq := emailClient.GetEmailVerificationMails()
			if emailsPreReq+1 != postReq {
				return fmt.Errorf("mismatch between emails pre equest (%d) and post request (%d", emailsPreReq, postReq)
			}
			return nil
		}); err == nil {
			testutil.FatalMsg(t, "Sent out emails for already verified user!")
		}
	})

}

func TestRestServer_EnableConfirmAndDelete2fa(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	email := gofakeit.Email()
	password := gofakeit.Password(true, true, true, true, true, 32)
	accessToken, _ := h.CreateAndAuthenticateUser(t, users.CreateUserArgs{
		Email:    email,
		Password: password,
	})

	t.Run("must have access token to enable 2FA", func(t *testing.T) {
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/auth/2fa",
			Method: "POST",
		})
		_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})

	t.Run("enable 2FA", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/auth/2fa",
			Method:      "POST",
		})

		h.AssertResponseOk(t, req)

		user, err := users.GetByEmail(testDB, email)
		if err != nil {
			testutil.FatalMsg(t, err)
		}
		testutil.AssertMsg(t, user.TotpSecret != nil, "TOTP secret was nil!")
		testutil.AssertMsg(t, !user.ConfirmedTotpSecret, "User confirmed TOTP secret!")

		t.Run("fail to confirm 2FA with bad code", func(t *testing.T) {
			req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
				AccessToken: accessToken,
				Path:        "/auth/2fa",
				Method:      "PUT",
				Body: `{
					"code": "123456"
				}`,
			})

			_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusForbidden)
		})

		t.Run("confirm 2FA", func(t *testing.T) {

			code, err := totp.GenerateCode(*user.TotpSecret, time.Now())
			if err != nil {
				testutil.FatalMsg(t, err)
			}

			req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
				AccessToken: accessToken,
				Path:        "/auth/2fa",
				Method:      "PUT",
				Body: fmt.Sprintf(`{
					"code": %q
				}`, code),
			})

			h.AssertResponseOk(t, req)

			t.Run("fail to confirm 2FA twice", func(t *testing.T) {
				code, err = totp.GenerateCode(*user.TotpSecret, time.Now())
				if err != nil {
					testutil.FatalMsg(t, err)
				}

				req = httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
					AccessToken: accessToken,
					Path:        "/auth/2fa",
					Method:      "PUT",
					Body: fmt.Sprintf(`{
					"code": %q
				}`, code),
				})

				_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
			})

			t.Run("should need TOTP code for login", func(t *testing.T) {
				req = httptestutil.GetRequest(t, httptestutil.RequestArgs{
					Path:   "/login",
					Method: "POST",
					Body: fmt.Sprintf(`{
						"email": %q,
						"password": %q
					}`, email, password),
				})
				_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
			})

			t.Run("should be able to login with TOTP code", func(t *testing.T) {
				code, err = totp.GenerateCode(*user.TotpSecret, time.Now())
				if err != nil {
					testutil.FatalMsg(t, err)
				}
				req = httptestutil.GetRequest(t, httptestutil.RequestArgs{
					Path:   "/login",
					Method: "POST",
					Body: fmt.Sprintf(`{
						"email": %q,
						"password": %q,
						"totp": %q
					}`, email, password, code),
				})
				h.AssertResponseOk(t, req)
			})

			t.Run("don't delete 2FA credentials with an invalid code", func(t *testing.T) {
				req = httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
					AccessToken: accessToken,
					Path:        "/auth/2fa",
					Method:      "DELETE",
					Body: `{
						"code": "123456"
					}`,
				})
				_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusForbidden)
			})

			t.Run("delete 2FA credentials", func(t *testing.T) {
				code, err = totp.GenerateCode(*user.TotpSecret, time.Now())
				if err != nil {
					testutil.FatalMsg(t, err)
				}

				deleteReq := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
					AccessToken: accessToken,
					Path:        "/auth/2fa",
					Method:      "DELETE",
					Body: fmt.Sprintf(`{
						"code": %q
					}`, code),
				})

				h.AssertResponseOk(t, deleteReq)

				loginReq := httptestutil.GetRequest(t, httptestutil.RequestArgs{
					Path:   "/login",
					Method: "POST",
					Body: fmt.Sprintf(`{
						"email": %q,
						"password": %q
					}`, email, password),
				})
				h.AssertResponseOk(t, loginReq)

				t.Run("fail to delete already deleted 2FA credentials", func(t *testing.T) {
					code, err = totp.GenerateCode(*user.TotpSecret, time.Now())
					if err != nil {
						testutil.FatalMsg(t, err)
					}
					req = httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
						AccessToken: accessToken,
						Path:        "/auth/2fa",
						Method:      "DELETE",
						Body: fmt.Sprintf(`{
							"code": %q
						}`, code),
					})
					_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
				})
			})

		})
	})
}
