package apiauth_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/dchest/passwordreset"
	"github.com/pquerna/otp/totp"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/api"
	"gitlab.com/arcanecrypto/teslacoil/api/apierr"
	"gitlab.com/arcanecrypto/teslacoil/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/models/users"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/httptestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/lntestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/mock"
	"gitlab.com/arcanecrypto/teslacoil/testutil/userstestutil"
)

var (
	testDB              *db.DB
	h                   httptestutil.TestHarness
	mockLightningClient = lntestutil.GetLightningMockClient()
	mockBitcoindClient  = bitcoind.GetBitcoinMockClient()
	mockHttpPoster      = testutil.GetMockHttpPoster()
	mockSendGridClient  = mock.GetMockSendGridClient()
	conf                = api.Config{
		LogLevel: logrus.InfoLevel,
		Network:  chaincfg.RegressionNetParams,
	}
)

func init() {
	dbConf := testutil.GetDatabaseConfig("api_auth")
	testDB = testutil.InitDatabase(dbConf)

	app, err := api.NewApp(testDB, mockLightningClient,
		mockSendGridClient, mockBitcoindClient,
		mockHttpPoster, conf)

	if err != nil {
		panic(err)
	}

	h = httptestutil.NewTestHarness(app.Router, testDB)
}

func TestPostLoginRoute(t *testing.T) {
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
		t.Parallel()
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
	t.Parallel()
	email := gofakeit.Email()
	pass := gofakeit.Password(true, true, true, true, true, 32)
	newPass := gofakeit.Password(true, true, true, true, true, 32)

	accessToken, _ := h.CreateAndAuthenticateUser(t, users.CreateUserArgs{
		Email:    email,
		Password: pass,
	})

	t.Run("Should give an error if not including the old password", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
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

func TestResetPasswordRouteBadToken(t *testing.T) {
	t.Parallel()
	pass := gofakeit.Password(true, true, true, true, true, 32)
	newPass := gofakeit.Password(true, true, true, true, true, 32)

	user := userstestutil.CreateUserOrFailWithPassword(t, testDB, pass)

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
}

func TestResetPasswordRoute(t *testing.T) {
	t.Parallel()
	pass := gofakeit.Password(true, true, true, true, true, 32)
	newPass := gofakeit.Password(true, true, true, true, true, 32)
	user := userstestutil.CreateUserOrFailWithPassword(t, testDB, pass)

	t.Run("should not be able to reset to a weak password", func(t *testing.T) {
		t.Parallel()
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

	t.Run("Reset the password by using the correct token", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
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

func TestEnableConfirmAndDelete2fa(t *testing.T) {
	t.Parallel()

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
