package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/async"
	"gitlab.com/arcanecrypto/teslacoil/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/ln"

	"github.com/brianvoe/gofakeit"
	"github.com/dchest/passwordreset"
	"github.com/pquerna/otp/totp"
	"gitlab.com/arcanecrypto/teslacoil/api/apierr"
	"gitlab.com/arcanecrypto/teslacoil/models/apikeys"
	"gitlab.com/arcanecrypto/teslacoil/models/transactions"
	"gitlab.com/arcanecrypto/teslacoil/models/users"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/httptestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/lntestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/mock"
	"gitlab.com/arcanecrypto/teslacoil/testutil/userstestutil"
)

var (
	databaseConfig = testutil.GetDatabaseConfig("routes")
	testDB         *db.DB
	conf           = Config{
		LogLevel: logrus.InfoLevel,
		Network:  chaincfg.RegressionNetParams,
	}

	h httptestutil.TestHarness

	mockSendGridClient                             = mock.GetMockSendGridClient()
	mockLightningClient lnrpc.LightningClient      = lntestutil.GetLightningMockClient()
	mockBitcoindClient  bitcoind.TeslacoilBitcoind = bitcoind.GetBitcoinMockClient()
	mockHttpPoster                                 = testutil.GetMockHttpPoster()
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
		if err := asyncutil.RetryNoBackoff(10, time.Millisecond*10, func() error {
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
			_, err := harness.AssertResponseNotOk(t, otherReq)
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
		if err := asyncutil.RetryNoBackoff(5, time.Millisecond*10, func() error {
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
		if err := asyncutil.RetryNoBackoff(5, time.Millisecond*10, func() error {
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

				_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
			})

			t.Run("should need TOTP code for login", func(t *testing.T) {
				req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
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
				code, err := totp.GenerateCode(*user.TotpSecret, time.Now())
				if err != nil {
					testutil.FatalMsg(t, err)
				}
				req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
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
				req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
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
				code, err := totp.GenerateCode(*user.TotpSecret, time.Now())
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
					code, err := totp.GenerateCode(*user.TotpSecret, time.Now())
					if err != nil {
						testutil.FatalMsg(t, err)
					}
					req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
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

func createFakeDeposit(t *testing.T, accessToken string, forceNewAddress bool, description string) int {
	req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
		AccessToken: accessToken,
		Path:        "/deposit",
		Method:      "POST",
		Body: fmt.Sprintf(
			`{ "forceNewAddress": %t, "description": %q }`,
			forceNewAddress,
			description),
	})

	var trans transactions.Transaction
	h.AssertResponseOKWithStruct(t, req, &trans)

	return trans.ID
}
func createFakeDeposits(t *testing.T, amount int, accessToken string) []int {
	t.Helper()

	ids := make([]int, amount)
	for i := 0; i < amount; i++ {
		ids[i] = createFakeDeposit(t, accessToken, true, "")
	}
	return ids
}

func TestGetTransactionByID(t *testing.T) {
	token, _ := h.CreateAndAuthenticateUser(t, users.CreateUserArgs{
		Email:    gofakeit.Email(),
		Password: gofakeit.Password(true, true, true, true, true, 21),
	})

	ids := createFakeDeposits(t, 3, token)

	t.Run("can get transaction by ID", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        fmt.Sprintf("/transaction/%d", ids[0]),
			Method:      "GET",
		})

		var trans transactions.Transaction
		h.AssertResponseOKWithStruct(t, req, &trans)

		if trans.ID != ids[0] {
			testutil.FailMsgf(t, "id's not equal, expected %d got %d", ids[0], trans.ID)
		}
	})
	t.Run("getting transaction with wrong ID returns error", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			// createFakeTransaction will always return the transaction in ascending order
			// where the highest index is the highest index saved to the user. therefore we +1
			Path:   fmt.Sprintf("/transaction/%d", ids[len(ids)-1]+1),
			Method: "GET",
		})

		_, _ = h.AssertResponseNotOkWithCode(t, req, 404)
	})

}

func assertGetsRightAmount(t *testing.T, req *http.Request, expected int) {
	var trans []transactions.Transaction
	h.AssertResponseOKWithStruct(t, req, &trans)
	if len(trans) != expected {
		testutil.FailMsgf(t, "expected %d transactions, got %d", expected, len(trans))
	}
}

func TestGetAllTransactions(t *testing.T) {
	token, _ := h.CreateAndAuthenticateUser(t, users.CreateUserArgs{
		Email:    gofakeit.Email(),
		Password: gofakeit.Password(true, true, true, true, true, 21),
	})
	createFakeDeposits(t, 10, token)

	t.Run("get transactions without query params should get all", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions",
			Method:      "GET",
		})

		assertGetsRightAmount(t, req, 10)
	})
	t.Run("get transactions with limit 10 should get 10", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions?limit=10",
			Method:      "GET",
		})

		assertGetsRightAmount(t, req, 10)
	})
	t.Run("get transactions with limit 5 should get 5", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions?limit=5",
			Method:      "GET",
		})

		assertGetsRightAmount(t, req, 5)
	})
	t.Run("get transactions with limit 0 should get all", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions?limit=0",
			Method:      "GET",
		})

		assertGetsRightAmount(t, req, 10)
	})
	t.Run("get /transactions with offset 10 should get 0", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions?offset=10",
			Method:      "GET",
		})

		assertGetsRightAmount(t, req, 0)
	})

	t.Run("get /transactions with offset 0 should get 10", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions?offset=0",
			Method:      "GET",
		})

		assertGetsRightAmount(t, req, 10)
	})

	t.Run("get /transactions with offset 5 should get 5", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions?offset=5",
			Method:      "GET",
		})

		assertGetsRightAmount(t, req, 5)
	})
	t.Run("get /transactions with offset 5 and limit 3 should get 3", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions?limit=3&offset=5",
			Method:      "GET",
		})

		assertGetsRightAmount(t, req, 3)
	})
}

func TestNewDeposit(t *testing.T) {
	t.Parallel()
	token, _ := h.CreateAndAuthenticateUser(t, users.CreateUserArgs{
		Email:    gofakeit.Email(),
		Password: gofakeit.Password(true, true, true, true, true, 21),
	})

	t.Run("can create new deposit with description", func(t *testing.T) {
		description := "fooDescription"
		t.Parallel()
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/deposit",
			Method:      "POST",
			Body: fmt.Sprintf(
				`{ "forceNewAddress": %t, "description": %q }`,
				true,
				description),
		})

		var trans transactions.Transaction
		h.AssertResponseOKWithStruct(t, req, &trans)
		testutil.AssertNotEqual(t, trans.Description, nil)
		testutil.AssertEqual(t, *trans.Description, description)
	})

	t.Run("can create new deposit without description", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/deposit",
			Method:      "POST",
			Body: fmt.Sprintf(
				`{"forceNewAddress":%t}`,
				true),
		})

		var trans transactions.Transaction
		h.AssertResponseOKWithStruct(t, req, &trans)
		testutil.AssertEqual(t, trans.Description, nil)

	})
}

func TestCreateInvoice(t *testing.T) {
	testutil.DescribeTest(t)

	randomMockClient := lntestutil.GetRandomLightningMockClient()
	app, _ := NewApp(testDB, randomMockClient, mockSendGridClient,
		mockBitcoindClient, mockHttpPoster, conf)
	otherH := httptestutil.NewTestHarness(app.Router, testDB)

	password := gofakeit.Password(true, true, true, true, true, 32)
	email := gofakeit.Email()
	accessToken, _ := otherH.CreateAndAuthenticateUser(t, users.CreateUserArgs{
		Email:    email,
		Password: password,
	})

	t.Run("Not create an invoice with non-positive amount ", func(t *testing.T) {
		testutil.DescribeTest(t)

		// gofakeit panics with too low value here...
		// https://github.com/brianvoe/gofakeit/issues/56
		amountSat := gofakeit.Number(math.MinInt64+2, -1)

		req := httptestutil.GetAuthRequest(t,
			httptestutil.AuthRequestArgs{
				AccessToken: accessToken,
				Path:        "/invoices/create",
				Method:      "POST",
				Body: fmt.Sprintf(`{
					"amountSat": %d
				}`, amountSat),
			})

		_, _ = otherH.AssertResponseNotOk(t, req)
	})

	t.Run("Not create an invoice with too large amount", func(t *testing.T) {
		testutil.DescribeTest(t)

		amountSat := gofakeit.Number(ln.MaxAmountSatPerInvoice, math.MaxInt64)

		req := httptestutil.GetAuthRequest(t,
			httptestutil.AuthRequestArgs{
				AccessToken: accessToken,
				Path:        "/invoices/create",
				Method:      "POST",
				Body: fmt.Sprintf(`{
					"amountSat": %d
				}`, amountSat),
			})

		_, _ = otherH.AssertResponseNotOk(t, req)
	})

	t.Run("Not create an invoice with a very long customer order id", func(t *testing.T) {
		t.Parallel()
		longId := gofakeit.Sentence(1000)

		req := httptestutil.GetAuthRequest(t,
			httptestutil.AuthRequestArgs{
				AccessToken: accessToken,
				Path:        "/invoices/create",
				Method:      "POST",
				Body: fmt.Sprintf(`{
					"amountSat": %d,
					"orderId": %q
				}`, 1337, longId),
			})

		_, _ = otherH.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)

	})

	t.Run("Create an invoice with a customer order id", func(t *testing.T) {
		t.Parallel()
		const orderId = "this-is-my-order-id"

		req := httptestutil.GetAuthRequest(t,
			httptestutil.AuthRequestArgs{
				AccessToken: accessToken,
				Path:        "/invoices/create",
				Method:      "POST",
				Body: fmt.Sprintf(`{
					"amountSat": %d,
					"orderId": %q
				}`, 1337, orderId),
			})

		res := otherH.AssertResponseOkWithJson(t, req)
		testutil.AssertEqual(t, res["customerOrderId"], orderId)

		t.Run("create an invoice with the same order ID twice", func(t *testing.T) {
			req := httptestutil.GetAuthRequest(t,
				httptestutil.AuthRequestArgs{
					AccessToken: accessToken,
					Path:        "/invoices/create",
					Method:      "POST",
					Body: fmt.Sprintf(`{
					"amountSat": %d,
					"orderId": %q
				}`, 1337, orderId),
				})

			otherH.AssertResponseOk(t, req)

		})
	})

	t.Run("Not create an invoice with zero amount ", func(t *testing.T) {
		testutil.DescribeTest(t)

		req := httptestutil.GetAuthRequest(t,
			httptestutil.AuthRequestArgs{
				AccessToken: accessToken,
				Path:        "/invoices/create",
				Method:      "POST",
				Body: `{
					"amountSat": 0
				}`,
			})

		_, _ = otherH.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})

	t.Run("Not create an invoice with an invalid callback URL", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/invoices/create",
			Method:      "POST",
			Body: `{
				"amountSat": 1000,
				"callbackUrl": "bad-url"
			}`,
		})
		_, _ = otherH.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})

	t.Run("create an invoice with a valid callback URL", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/invoices/create",
			Method:      "POST",
			Body: fmt.Sprintf(`{
				"amountSat": %d,
				"callbackUrl": "https://example.com"
			}`, 1339),
		})
		invoicesJson := otherH.AssertResponseOkWithJson(t, req)
		invoice, ok := invoicesJson["paymentRequest"].(string)
		lnRpcInvoice := lnrpc.Invoice{PaymentRequest: invoice}
		mockInvoice, _ := ln.AddInvoice(randomMockClient, lnRpcInvoice)
		if !ok {
			testutil.FatalMsgf(t, "invoicesJson['paymentRequest'] was not a string! %+v", invoicesJson)
		}
		testutil.AssertMsg(t, invoicesJson["callbackUrl"] != nil, "callback URL was nil!")

		t.Run("receive a POST to the given URL when paying the invoice",
			func(t *testing.T) {
				user, err := users.GetByEmail(testDB, email)
				if err != nil {
					testutil.FatalMsg(t, err)
				}

				var apiKey apikeys.Key
				// check if there are any API keys
				if keys, err := apikeys.GetByUserId(testDB, user.ID); err == nil && len(keys) > 0 {
					apiKey = keys[0]
					// if not, try to create one, fail it if doesn't work
				} else if _, key, err := apikeys.New(testDB, user); err != nil {
					testutil.FatalMsg(t, err)
				} else {
					apiKey = key
				}

				if _, err := payments.UpdateInvoiceStatus(*mockInvoice,
					testDB, mockHttpPoster); err != nil {
					testutil.FatalMsg(t, err)
				}

				checkPostSent := func() bool {
					reqs := mockHttpPoster.GetSentPostRequests()
					return reqs == 1
				}

				// emails are sent in a go-routine, so can't assume they're sent fast
				// enough for test to pick up
				if err := async.Await(8,
					time.Millisecond*20, checkPostSent); err != nil {
					testutil.FatalMsg(t, err)
				}

				bodyBytes := mockHttpPoster.GetSentPostRequest(0)
				body := payments.CallbackBody{}

				if err := json.Unmarshal(bodyBytes, &body); err != nil {
					testutil.FatalMsg(t, err)
				}
				hmac := hmac.New(sha256.New, apiKey.HashedKey)
				_, _ = hmac.Write([]byte(fmt.Sprintf("%d", body.Payment.ID)))

				sum := hmac.Sum(nil)
				testutil.AssertEqual(t, sum, body.Hash)
			})
	})
}

func TestRestServer_CreateApiKey(t *testing.T) {
	testutil.DescribeTest(t)

	password := gofakeit.Password(true, true, true, true, true, 32)
	accessToken, _ := h.CreateAndAuthenticateUser(t, users.CreateUserArgs{
		Email:    gofakeit.Email(),
		Password: password,
	})

	t.Run("create an API key", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/apikey",
			Method:      "POST",
		})
		json := h.AssertResponseOkWithJson(t, req)

		testutil.AssertMsg(t, json["key"] != "", "`key` was empty!")

		t.Run("creating a new key should yield a different one", func(t *testing.T) {

			req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
				AccessToken: accessToken,
				Path:        "/apikey",
				Method:      "POST",
			})
			newJson := h.AssertResponseOkWithJson(t, req)
			testutil.AssertNotEqual(t, json["key"], newJson["key"])
			testutil.AssertEqual(t, json["userId"], newJson["userId"])
			testutil.AssertNotEqual(t, json["userId"], nil)
		})
	})
}

func TestRestServer_GetAllPayments(t *testing.T) {
	t.Parallel()
	pass := gofakeit.Password(true, true, true, true, true, 32)
	user := userstestutil.CreateUserOrFailWithPassword(t, testDB, pass)
	accessToken, _ := h.AuthenticaticateUser(t, users.CreateUserArgs{
		Email:    user.Email,
		Password: pass,
	})

	t.Run("fail with bad query parameters", func(t *testing.T) {
		t.Run("string argument", func(t *testing.T) {
			req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
				AccessToken: accessToken,
				Path:        fmt.Sprintf("/payments?limit=foobar&offset=0"),
				Method:      "GET",
			})

			_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
		})
		t.Run("negative argument", func(t *testing.T) {
			req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
				AccessToken: accessToken,
				Path:        fmt.Sprintf("/payments?offset=-1"),
				Method:      "GET",
			})

			_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
		})
	})

	t.Run("succeed with query parameters", func(t *testing.T) {
		opts := payments.NewPaymentOpts{
			UserID:    user.ID,
			AmountSat: 123,
		}
		_ = payments.CreateNewPaymentOrFail(t, testDB, mockLightningClient, opts)
		_ = payments.CreateNewPaymentOrFail(t, testDB, mockLightningClient, opts)
		_ = payments.CreateNewPaymentOrFail(t, testDB, mockLightningClient, opts)
		_ = payments.CreateNewPaymentOrFail(t, testDB, mockLightningClient, opts)
		_ = payments.CreateNewPaymentOrFail(t, testDB, mockLightningClient, opts)
		_ = payments.CreateNewPaymentOrFail(t, testDB, mockLightningClient, opts)
		const numPayments = 6

		const limit = 3
		const offset = 2
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        fmt.Sprintf("/payments?limit=%d&offset=%d", limit, offset),
			Method:      "GET",
		})

		res := h.AssertResponseOkWithJsonList(t, req)
		testutil.AssertMsgf(t, len(res) == numPayments-limit, "Unexpected number of payments: %d", len(res))

	})
}

func TestRestServer_WithdrawOnChain(t *testing.T) {
	t.Parallel()
	const balanceSats = 10000

	t.Run("regular withdrawal", func(t *testing.T) {
		t.Parallel()
		pass := gofakeit.Password(true, true, true, true, true, 32)
		user := userstestutil.CreateUserOrFailWithPassword(t, testDB, pass)
		userstestutil.IncreaseBalanceOrFail(t, testDB, user, balanceSats)
		const withdrawAmount = 1234

		accessToken, _ := h.AuthenticaticateUser(t, users.CreateUserArgs{
			Email:    user.Email,
			Password: pass,
		})

		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/withdraw",
			Method:      "POST",
			Body: fmt.Sprintf(`{
			"amountSat": %d,
			"address": %q
		}`, withdrawAmount, "bcrt1qvn9hnzlpgrvcmrusj6cfh6cvgppp2z8fqeuxmy"),
		})

		h.AssertResponseOk(t, req)

		balanceReq := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/user",
			Method:      "GET",
		})

		balanceRes := h.AssertResponseOkWithJson(t, balanceReq)
		testutil.AssertEqual(t, balanceRes["balance"], balanceSats-withdrawAmount)
	})

	t.Run("fail to withdraw too much", func(t *testing.T) {
		t.Parallel()
		pass := gofakeit.Password(true, true, true, true, true, 32)
		user := userstestutil.CreateUserOrFailWithPassword(t, testDB, pass)
		userstestutil.IncreaseBalanceOrFail(t, testDB, user, balanceSats)
		accessToken, _ := h.AuthenticaticateUser(t, users.CreateUserArgs{
			Email:    user.Email,
			Password: pass,
		})

		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/withdraw",
			Method:      "POST",
			Body: fmt.Sprintf(`{
			"amountSat": %d,
			"address": %q
		}`, balanceSats+1, "bcrt1qvn9hnzlpgrvcmrusj6cfh6cvgppp2z8fqeuxmy"),
		})

		_, err := h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
		testutil.AssertEqual(t, apierr.ErrBalanceTooLowForWithdrawal, err)

	})
}
