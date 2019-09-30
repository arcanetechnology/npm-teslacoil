package api

import (
	"bytes"
	"fmt"
	"math"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit"
	"github.com/dchest/passwordreset"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/pquerna/otp/totp"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/asyncutil"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/internal/payments"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/ln"
	"gitlab.com/arcanecrypto/teslacoil/internal/transactions"
	"gitlab.com/arcanecrypto/teslacoil/internal/users"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/httptestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/lntestutil"
)

var (
	databaseConfig = testutil.GetDatabaseConfig("routes")
	testDB         *db.DB
	conf           = Config{LogLevel: logrus.InfoLevel}

	h httptestutil.TestHarness

	mockSendGridClient  = testutil.GetMockSendGridClient()
	mockLightningClient = lntestutil.GetLightningMockClient()
	mockHttpPoster      = testutil.GetMockHttpPoster()
)

func init() {
	testDB = testutil.InitDatabase(databaseConfig)

	app, err := NewApp(testDB, mockLightningClient,
		mockSendGridClient, mockHttpPoster, conf)
	if err != nil {
		panic(err.Error())
	}

	h = httptestutil.NewTestHarness(app.Router)
}

func TestMain(m *testing.M) {
	build.SetLogLevel(logrus.DebugLevel)

	// new values for gofakeit every time
	gofakeit.Seed(0)

	result := m.Run()
	if err := testDB.Close(); err != nil {
		panic(err.Error())
	}
	os.Exit(result)
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
		h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})
	t.Run("Creating a user must fail without an email", func(t *testing.T) {
		testutil.DescribeTest(t)
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path: "/users", Method: "POST",
			Body: fmt.Sprintf(`{
				"password": %q
			}`, gofakeit.Password(true, true, true, true, true, 32)),
		})
		h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
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
		h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
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
		h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
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
		testutil.DescribeTest(t)
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
		testutil.AssertEqual(t, jsonRes["firstName"], firstName)
		testutil.AssertEqual(t, jsonRes["lastName"], lastName)
	})
}

func TestPostUsersRoute(t *testing.T) {
	testutil.DescribeTest(t)

	email := gofakeit.Email()
	pass := gofakeit.Password(true, true, true, true, true, 32)
	accessToken := h.CreateAndLoginUser(t, users.CreateUserArgs{
		Email:    email,
		Password: pass,
	})

	req := httptestutil.GetAuthRequest(t,
		httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/user", Method: "GET",
		})

	jsonRes := h.AssertResponseOkWithJson(t, req)
	testutil.AssertEqual(t, jsonRes["firstName"], nil)
	testutil.AssertEqual(t, jsonRes["lastName"], nil)
	testutil.AssertEqual(t, jsonRes["email"], email)
}

func TestPostLoginRoute(t *testing.T) {
	testutil.DescribeTest(t)

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

	t.Run("fail to login with bad password", func(t *testing.T) {
		badPassword := "this-is-bad"
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/login",
			Method: "POST",
			Body: fmt.Sprintf(`{
			"email": %q,
			"password": %q
		}`, email, badPassword),
		})
		h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})

	t.Run("fail to login with invalid email", func(t *testing.T) {
		badEmail := "foobar"
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/login",
			Method: "POST",
			Body: fmt.Sprintf(`{
			"email": %q,
			"password": %q
		}`, badEmail, password),
		})
		h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})

	t.Run("login with proper email", func(t *testing.T) {
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
}

func TestChangePasswordRoute(t *testing.T) {
	testutil.DescribeTest(t)
	email := gofakeit.Email()
	pass := gofakeit.Password(true, true, true, true, true, 32)
	newPass := gofakeit.Password(true, true, true, true, true, 32)

	accessToken := h.CreateAndLoginUser(t, users.CreateUserArgs{
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

		h.AssertResponseNotOkWithCode(t, changePassReq, http.StatusBadRequest)
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

		h.AssertResponseNotOkWithCode(t, changePassReq, http.StatusBadRequest)
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

		h.AssertResponseNotOkWithCode(t, changePassReq, http.StatusBadRequest)
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

		h.AssertResponseNotOkWithCode(t, changePassReq, http.StatusBadRequest)
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

		h.AssertResponseNotOkWithCode(t, changePassReq, http.StatusForbidden)
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
		h.AssertResponseNotOkWithCode(t, changePassReq, http.StatusBadRequest)
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
		h.AssertResponseNotOk(t, badLoginReq)

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

		h.AssertResponseNotOkWithCode(t, changePassReq, http.StatusForbidden)
	})

}

func TestResetPasswordRoute(t *testing.T) {
	email := gofakeit.Email()
	pass := gofakeit.Password(true, true, true, true, true, 32)
	newPass := gofakeit.Password(true, true, true, true, true, 32)
	user, err := users.Create(testDB, users.CreateUserArgs{
		Email:    email,
		Password: pass,
	})
	if err != nil {
		testutil.FatalMsg(t, err)
	}

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
		h.AssertResponseNotOkWithCode(t, resetPassReq, http.StatusBadRequest)
	})

	t.Run("Should not be able to reset the user password by using a bad token", func(t *testing.T) {
		badSecretKey := []byte("this is a secret key which we expect to fail")
		badToken := passwordreset.NewToken(email, users.PasswordResetTokenDuration,
			user.HashedPassword, badSecretKey)
		badTokenReq := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/auth/reset_password",
			Method: "PUT",
			Body: fmt.Sprintf(`{
				"token": %q,
				"password": %q
			}`, badToken, newPass),
		})
		h.AssertResponseNotOkWithCode(t, badTokenReq, http.StatusForbidden)

		// we should be able to log in with old credentials
		loginReq := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/login",
			Method: "POST",
			Body: fmt.Sprintf(`{
				"password": %q,
				"email": %q
			}`, pass, email),
		})
		h.AssertResponseOk(t, loginReq)

		// we should NOT be able to log in with new credentials
		badLoginReq := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/login",
			Method: "POST",
			Body: fmt.Sprintf(`{
				"password": %q,
				"email": %q
			}`, newPass, email),
		})
		h.AssertResponseNotOk(t, badLoginReq)
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
			}`, newPass, email),
		})
		h.AssertResponseOk(t, loginReq)

		// we should NOT be able to log in with old credentials
		badLoginReq := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/login",
			Method: "POST",
			Body: fmt.Sprintf(`{
				"password": %q,
				"email": %q
			}`, pass, email),
		})
		h.AssertResponseNotOk(t, badLoginReq)
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
		h.AssertResponseNotOkWithCode(t, secondResetPassReq, http.StatusForbidden)

	})
}

func TestPutUserRoute(t *testing.T) {
	testutil.DescribeTest(t)

	email := gofakeit.Email()
	password := gofakeit.Password(true, true, true, true, true, 32)
	accessToken := h.CreateAndLoginUser(t, users.CreateUserArgs{
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
		h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
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
	email := gofakeit.Email()
	h.CreateAndLoginUser(t, users.CreateUserArgs{
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
	testutil.AssertMsg(t, mockSendGridClient.GetSentEmails() > 0,
		"Sendgrid client didn't send any emails!")
}

func TestRestServer_EnableConfirmAndDelete2fa(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	email := gofakeit.Email()
	password := gofakeit.Password(true, true, true, true, true, 32)
	accessToken := h.CreateAndLoginUser(t, users.CreateUserArgs{
		Email:    email,
		Password: password,
	})

	t.Run("must have access token to enable 2FA", func(t *testing.T) {
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/auth/2fa",
			Method: "POST",
		})
		h.AssertResponseNotOkWithCode(t, req, http.StatusForbidden)
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

			h.AssertResponseNotOkWithCode(t, req, http.StatusForbidden)
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

				h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
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
				h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
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
				h.AssertResponseNotOkWithCode(t, req, http.StatusForbidden)
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
					h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
				})
			})

		})
	})
}

// var (
// simnetAddress = "sb1qnl462s336uu4n8xanhyvpega4zwjr9jrhc26x4"
// )

// func createFakeWithdrawal(t *testing.T, accessToken string) int {
// req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
// AccessToken: accessToken,
// Path:        "/withdraw",
// Method:      "POST",
// Body:        fmt.Sprintf(`{ "address": %q }`, simnetAddress),
// })
// res := h.AssertResponseOk(t, req)
//
// var trans transactions.Transaction
// h.AssertResponseOKWithStruct(t, res.Body, &trans)
//
// return trans.ID
// }

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
	res := h.AssertResponseOk(t, req)

	var trans transactions.Transaction
	h.AssertResponseOKWithStruct(t, res.Body, &trans)

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

// createFakeTransactions creates `amount` transactions, has a 50/50 chance
// of creating either a withdrawal or a deposit
// func createFakeTransactions(t *testing.T, amount int, accessToken string) []int {
// ids := make([]int, amount)
// for i := 0; i < amount; i++ {
// if gofakeit.Int8()%2 == 0 {
// ids[i] = createFakeWithdrawal(t, accessToken)
// } else {
// ids[i] = createFakeDeposit(t, accessToken, true, "")
// }
// }
// return ids
// }

func TestGetTransactionByID(t *testing.T) {
	token := h.CreateAndLoginUser(t, users.CreateUserArgs{
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

		res := h.AssertResponseOk(t, req)
		var trans transactions.Transaction
		h.AssertResponseOKWithStruct(t, res.Body, &trans)

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

		h.AssertResponseNotOkWithCode(t, req, 404)
	})

}

func assertGetsRightAmount(t *testing.T, body *bytes.Buffer, expected int) {
	var trans []transactions.Transaction
	h.AssertResponseOKWithStruct(t, body, &trans)
	if len(trans) != expected {
		testutil.FailMsgf(t, "expected %d transactions, got %d", expected, len(trans))
	}
}

func TestGetAllTransactions(t *testing.T) {
	token := h.CreateAndLoginUser(t, users.CreateUserArgs{
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
		res := h.AssertResponseOk(t, req)

		assertGetsRightAmount(t, res.Body, 10)
	})
	t.Run("get transactions with limit 10 should get 10", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions?limit=10",
			Method:      "GET",
		})
		res := h.AssertResponseOk(t, req)

		assertGetsRightAmount(t, res.Body, 10)
	})
	t.Run("get transactions with limit 5 should get 5", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions?limit=5",
			Method:      "GET",
		})
		res := h.AssertResponseOk(t, req)

		assertGetsRightAmount(t, res.Body, 5)
	})
	t.Run("get transactions with limit 0 should get all", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions?limit=0",
			Method:      "GET",
		})
		res := h.AssertResponseOk(t, req)

		assertGetsRightAmount(t, res.Body, 10)
	})
	t.Run("get /transactions with offset 10 should get 0", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions?offset=10",
			Method:      "GET",
		})
		res := h.AssertResponseOk(t, req)

		assertGetsRightAmount(t, res.Body, 0)
	})

	t.Run("get /transactions with offset 0 should get 10", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions?offset=0",
			Method:      "GET",
		})
		res := h.AssertResponseOk(t, req)

		assertGetsRightAmount(t, res.Body, 10)
	})

	t.Run("get /transactions with offset 5 should get 5", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions?offset=5",
			Method:      "GET",
		})
		res := h.AssertResponseOk(t, req)

		assertGetsRightAmount(t, res.Body, 5)
	})
	t.Run("get /transactions with offset 5 and limit 3 should get 3", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/transactions?limit=3&offset=5",
			Method:      "GET",
		})
		res := h.AssertResponseOk(t, req)

		assertGetsRightAmount(t, res.Body, 3)
	})
}

func TestNewDeposit(t *testing.T) {
	token := h.CreateAndLoginUser(t, users.CreateUserArgs{
		Email:    gofakeit.Email(),
		Password: gofakeit.Password(true, true, true, true, true, 21),
	})

	description := "fooDescription"
	t.Run("can create new deposit with description", func(t *testing.T) {
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: token,
			Path:        "/deposit",
			Method:      "POST",
			Body: fmt.Sprintf(
				`{ "forceNewAddress": %t, "description": "%s" }`,
				true,
				description),
		})

		res := h.AssertResponseOk(t, req)
		var trans transactions.Transaction
		h.AssertResponseOKWithStruct(t, res.Body, &trans)

		if trans.Description != description {
			testutil.FailMsgf(t, "descriptions not equal, expected %s got %s", description, trans.Description)
		}
	})
}

func TestCreateInvoice(t *testing.T) {
	testutil.DescribeTest(t)

	randomMockClient := lntestutil.GetRandomLightningMockClient()
	app, _ := NewApp(testDB, randomMockClient, mockSendGridClient, mockHttpPoster, conf)
	otherH := httptestutil.NewTestHarness(app.Router)

	password := gofakeit.Password(true, true, true, true, true, 32)
	accessToken := otherH.CreateAndLoginUser(t, users.CreateUserArgs{
		Email:    gofakeit.Email(),
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

		otherH.AssertResponseNotOk(t, req)
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

		otherH.AssertResponseNotOk(t, req)
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

		otherH.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
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
		otherH.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})

	t.Run("create an invoice with a valid callback URL", func(t *testing.T) {
		t.Parallel()
		mockInvoice, _ := ln.AddInvoice(randomMockClient, lnrpc.Invoice{})
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/invoices/create",
			Method:      "POST",
			Body: fmt.Sprintf(`{
				"amountSat": %d,
				"callbackUrl": "https://example.com"
			}`, mockInvoice.Value),
		})
		json := otherH.AssertResponseOkWithJson(t, req)
		testutil.AssertMsg(t, json["callbackUrl"] != nil, "callback URL was nil!")

		t.Run("receive a POST to the given URL when paying the invoice",
			func(t *testing.T) {
				if _, err := payments.UpdateInvoiceStatus(*mockInvoice,
					testDB, mockHttpPoster); err != nil {
					testutil.FatalMsg(t, err)
				}

				checkPostSent := func() bool {
					return mockHttpPoster.GetSentPostRequests() == 1
				}

				// emails are sent in a go-routine, so can't assume they're sent fast
				// enough for test to pick up
				if err := asyncutil.Await(10,
					time.Millisecond*20, checkPostSent); err != nil {
					testutil.FatalMsg(t, err)
				}
			})
	})
}
