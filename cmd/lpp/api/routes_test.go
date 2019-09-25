package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit"
	"github.com/dchest/passwordreset"
	"github.com/pquerna/otp/totp"
	"github.com/sendgrid/rest"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/internal/payments"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/ln"
	"gitlab.com/arcanecrypto/teslacoil/internal/users"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
)

var (
	databaseConfig = testutil.GetDatabaseConfig("routes")
	testDB         *db.DB
	conf           = Config{LogLevel: logrus.InfoLevel}
	app            *RestServer
	mockLndApp     RestServer
	realLndApp     RestServer

	mockSendGridClient *MockSendGridClient
)

type MockSendGridClient struct {
	SentEmails int
}

func (mock *MockSendGridClient) Send(email *mail.SGMailV3) (*rest.Response, error) {
	mock.SentEmails += 1
	return &rest.Response{
		StatusCode: 202,
		Body:       "",
		Headers:    nil,
	}, nil
}

func TestMain(m *testing.M) {
	build.SetLogLevel(logrus.InfoLevel)
	testDB = testutil.InitDatabase(databaseConfig)

	// new values for gofakeit every time
	gofakeit.Seed(0)

	mockSendGridClient = &MockSendGridClient{}

	var err error
	mockLndApp, err = NewApp(testDB,
		testutil.GetLightningMockClient(),
		mockSendGridClient,
		conf)
	if err != nil {
		panic(err.Error())
	}

	// this is not good, but a workaround until we have a proper testing/CI
	// harness with nodes and the whole shebang
	if os.Getenv("CI") != "" {
		realLndApp, err = NewApp(testDB,
			testutil.GetLightningMockClient(),
			mockSendGridClient,
			conf)
		if err != nil {
			panic(err.Error())
		}
	} else {
		lndConfig := testutil.GetLightingConfig()
		lnd, err := ln.NewLNDClient(lndConfig)
		if err != nil {
			panic(err.Error())
		}

		realLndApp, err = NewApp(testDB,
			lnd,
			mockSendGridClient,
			conf)
		if err != nil {
			panic(err.Error())
		}

	}

	// default app is mocked out version
	app = &mockLndApp

	result := m.Run()
	if err := testDB.Close(); err != nil {
		panic(err.Error())
	}
	os.Exit(result)
}

// Performs the given request against the API. Asserts that the
// response completed successfully. Returns the response from the API
// TODO Assert that if failure, contains reasonably shaped JSON
func assertResponseOk(t *testing.T, request *http.Request) *httptest.ResponseRecorder {
	t.Helper()

	response := httptest.NewRecorder()
	app.Router.ServeHTTP(response, request)

	if response.Code != 200 {
		testutil.FailMsgf(t, "Got failure code (%d) on path %s", response.Code, extractMethodAndPath(request))
	}

	return response
}

// TODO Assert that if failure, contains reasonably shaped JSON
func assertResponseNotOk(t *testing.T, request *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	app.Router.ServeHTTP(response, request)
	if response.Code < 300 {
		testutil.FatalMsgf(t, "Got success code (%d) on path %s", response.Code, extractMethodAndPath(request))
	}
	return response
}

// assertResponseNotOkWithCode checks that the given request results in the
// given HTTP status code. It returns the response to the request.
func assertResponseNotOkWithCode(t *testing.T, request *http.Request, code int) *httptest.ResponseRecorder {
	testutil.AssertMsgf(t, code >= 100 && code <= 500, "Given code (%d) is not a valid HTTP code", code)
	t.Helper()

	response := assertResponseNotOk(t, request)
	testutil.AssertMsgf(t, response.Code == code,
		"Expected code (%d) does not match with found code (%d)", code, response.Code)
	return response
}

func extractMethodAndPath(req *http.Request) string {
	return req.Method + " " + req.URL.Path
}

// First performs `assertResponseOk`, then asserts that the body of the response
// can be parsed as JSON, and then returns the parsed JSON
func assertResponseOkWithJson(t *testing.T, request *http.Request) map[string]interface{} {

	t.Helper()
	response := assertResponseOk(t, request)
	var destination map[string]interface{}

	if err := json.Unmarshal(response.Body.Bytes(), &destination); err != nil {
		stringBody := response.Body.String()
		testutil.FatalMsgf(t, "%+v. Body: %s ",
			err, stringBody)

	}
	return destination
}

func createUser(t *testing.T, args users.CreateUserArgs) map[string]interface{} {
	if args.Password == "" {
		testutil.FatalMsg(t, "You forgot to set the password!")
	}

	if args.Email == "" {
		testutil.FatalMsg(t, "You forgot to set the email!")
	}

	var firstName string
	var lastName string
	if args.FirstName != nil {
		firstName = fmt.Sprintf("%q", *args.FirstName)
	} else {
		firstName = "null"
	}

	if args.LastName != nil {
		lastName = fmt.Sprintf("%q", *args.LastName)
	} else {
		lastName = "null"
	}

	createUserRequest := getRequest(t, RequestArgs{
		Path:   "/users",
		Method: "POST",
		Body: fmt.Sprintf(`{
			"email": %q,
			"password": %q,
			"firstName": %s,
			"lastName": %s
		}`, args.Email, args.Password, firstName, lastName),
	})

	return assertResponseOkWithJson(t, createUserRequest)
}

// Creates and logs in a user with the given email and password. Returns
// the access token for this session.
func createAndLoginUser(t *testing.T, args users.CreateUserArgs) string {
	_ = createUser(t, args)

	loginUserReq := getRequest(t, RequestArgs{
		Path:   "/login",
		Method: "POST",
		Body: fmt.Sprintf(`{
			"email": %q,
			"password": %q
		}`, args.Email, args.Password),
	})

	jsonRes := assertResponseOkWithJson(t, loginUserReq)

	tokenPath := "accessToken"
	fail := func() {
		methodAndPath := extractMethodAndPath(loginUserReq)
		testutil.FatalMsgf(t, "Returned JSON (%+v) did have string property '%s'. Path: %s",
			jsonRes, tokenPath, methodAndPath)
	}

	maybeNilToken, ok := jsonRes[tokenPath]
	if !ok {
		fail()
	}

	var token string
	switch untypedToken := maybeNilToken.(type) {
	case string:
		token = untypedToken
	default:
		fail()
	}

	return token
}

// Checks if the given string is valid JSON
func isJSONString(s string) bool {
	var js interface{}
	err := json.Unmarshal([]byte(s), &js)
	return err == nil
}

type AuthRequestArgs struct {
	AccessToken string
	Path        string
	Method      string
	Body        string
}

// Returns a HTTP request that carries a proper auth header and an optional
// JSON body
func getAuthRequest(t *testing.T, args AuthRequestArgs) *http.Request {
	t.Helper()
	if args.AccessToken == "" {
		testutil.FatalMsg(t, "You forgot to set AccessToken")
	}
	req := getRequest(t, RequestArgs{Path: args.Path,
		Method: args.Method, Body: args.Body})
	req.Header.Set("Authorization", args.AccessToken)
	return req
}

type RequestArgs struct {
	Path   string
	Method string
	Body   string
}

// Returns a HTTP request with an optional JSON body
func getRequest(t *testing.T, args RequestArgs) *http.Request {
	t.Helper()
	if args.Path == "" {
		testutil.FatalMsg(t, "You forgot to set Path")
	}
	if args.Method == "" {
		testutil.FatalMsg(t, "You forgot to set Method")
	}

	var body *bytes.Buffer
	if args.Body == "" {
		body = &bytes.Buffer{}
	} else if isJSONString(args.Body) {
		body = bytes.NewBuffer([]byte(args.Body))
	} else {
		testutil.FatalMsgf(t, "Body was not valid JSON: %s", args.Body)
	}

	res, err := http.NewRequest(args.Method, args.Path, body)
	if err != nil {
		testutil.FatalMsgf(t, "Couldn't construct request: %+v", err)
	}
	return res
}

func TestCreateUser(t *testing.T) {
	t.Run("Creating a user must fail without an email", func(t *testing.T) {
		testutil.DescribeTest(t)
		req := getRequest(t, RequestArgs{
			Path: "/users", Method: "POST",
			Body: `{
				"password": "foobar"
			}`,
		})
		assertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})

	t.Run("Creating a user must fail with an empty email", func(t *testing.T) {
		testutil.DescribeTest(t)
		req := getRequest(t, RequestArgs{
			Path: "/users", Method: "POST",
			Body: `{
				"password": "foobar",
				"email": ""
			}`,
		})
		assertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})

	t.Run("creating an user with an invalid email should fail", func(t *testing.T) {
		pass := gofakeit.Password(true, true, true, true, true, 32)
		req := getRequest(t, RequestArgs{
			Path:   "/users",
			Method: "POST",
			Body: fmt.Sprintf(`{
				"password": %q,
				"email": "this-is-not@a-valid-mail"
			}`, pass),
		})
		assertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})

	t.Run("It should be possible to create a user without any names", func(t *testing.T) {
		email := gofakeit.Email()
		testutil.DescribeTest(t)
		body := RequestArgs{
			Path: "/users", Method: "POST",
			Body: fmt.Sprintf(`{
				"password": "foobar",
				"email": "%s"
			}`, email),
		}
		req := getRequest(t, body)
		jsonRes := assertResponseOkWithJson(t, req)
		testutil.AssertEqual(t, jsonRes["email"], email)
		testutil.AssertEqual(t, jsonRes["firstName"], nil)
		testutil.AssertEqual(t, jsonRes["lastName"], nil)
	})

	t.Run("It should be possible to create a user with names", func(t *testing.T) {
		email := gofakeit.Email()
		firstName := gofakeit.FirstName()
		lastName := gofakeit.LastName()
		testutil.DescribeTest(t)
		req := getRequest(t, RequestArgs{
			Path: "/users", Method: "POST",
			Body: fmt.Sprintf(`{
				"password": "foobar",
				"email": "%s",
				"firstName": "%s", 
				"lastName": "%s"
			}`, email, firstName, lastName),
		})
		jsonRes := assertResponseOkWithJson(t, req)
		testutil.AssertEqual(t, jsonRes["email"], email)
		testutil.AssertEqual(t, jsonRes["firstName"], firstName)
		testutil.AssertEqual(t, jsonRes["lastName"], lastName)
	})
}

func TestPostUsersRoute(t *testing.T) {
	testutil.DescribeTest(t)

	email := gofakeit.Email()
	accessToken := createAndLoginUser(t, users.CreateUserArgs{
		Email:    email,
		Password: "password",
	})

	req := getAuthRequest(t,
		AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/user", Method: "GET",
		})

	jsonRes := assertResponseOkWithJson(t, req)
	testutil.AssertEqual(t, jsonRes["firstName"], nil)
	testutil.AssertEqual(t, jsonRes["lastName"], nil)
	testutil.AssertEqual(t, jsonRes["email"], email)
}

func TestPostLoginRoute(t *testing.T) {
	testutil.DescribeTest(t)

	email := gofakeit.Email()
	password := "password"
	first := gofakeit.FirstName()
	second := gofakeit.LastName()

	_ = createUser(t, users.CreateUserArgs{
		Email:     email,
		Password:  password,
		FirstName: &first,
		LastName:  &second,
	})

	t.Run("fail to login with invalid email", func(t *testing.T) {
		badEmail := "foobar"
		req := getRequest(t, RequestArgs{
			Path:   "/login",
			Method: "POST",
			Body: fmt.Sprintf(`{
			"email": %q,
			"password": %q
		}`, badEmail, password),
		})
		assertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})

	t.Run("login with proper email", func(t *testing.T) {
		req := getRequest(t, RequestArgs{
			Path:   "/login",
			Method: "POST",
			Body: fmt.Sprintf(`{
			"email": %q,
			"password": %q
		}`, email, password),
		})
		res := assertResponseOkWithJson(t, req)
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

	accessToken := createAndLoginUser(t, users.CreateUserArgs{
		Email:    email,
		Password: pass,
	})

	t.Run("Should give an error if not including the old password", func(t *testing.T) {
		changePassReq := getAuthRequest(t, AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/auth/change_password",
			Method:      "PUT",
			Body: fmt.Sprintf(`{
			"newPassword": %q,
			"repeatedNewPassword": %q
		}`, newPass, newPass),
		})

		assertResponseNotOkWithCode(t, changePassReq, http.StatusBadRequest)
	})

	t.Run("Should give an error if not including the new password", func(t *testing.T) {
		changePassReq := getAuthRequest(t, AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/auth/change_password",
			Method:      "PUT",
			Body: fmt.Sprintf(`{
			"oldPassword": %q,
			"repeatedNewPassword": %q
		}`, pass, newPass),
		})

		assertResponseNotOkWithCode(t, changePassReq, http.StatusBadRequest)
	})

	t.Run("Should give an error if not including the repeated password", func(t *testing.T) {
		changePassReq := getAuthRequest(t, AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/auth/change_password",
			Method:      "PUT",
			Body: fmt.Sprintf(`{
			"oldPassword": %q,
			"newPassword": %q
		}`, pass, newPass),
		})

		assertResponseNotOkWithCode(t, changePassReq, http.StatusBadRequest)
	})

	t.Run("Should give an error if including the wrong repeated password", func(t *testing.T) {
		anotherNewPassword := gofakeit.Password(true, true, true, true, true, 32)
		changePassReq := getAuthRequest(t, AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/auth/change_password",
			Method:      "PUT",
			Body: fmt.Sprintf(`{
			"oldPassword": %q,
			"newPassword": %q,
			"repeatedNewPassword": %q
		}`, pass, newPass, anotherNewPassword),
		})

		assertResponseNotOkWithCode(t, changePassReq, http.StatusBadRequest)
	})

	t.Run("Should give an error if not including the access token", func(t *testing.T) {
		changePassReq := getRequest(t, RequestArgs{
			Path:   "/auth/change_password",
			Method: "PUT",
			Body: fmt.Sprintf(`{
			"newPassword": %q,
			"oldPassword": %q,
			"repeatedNewPassword": %q
		}`, newPass, pass, pass),
		})

		assertResponseNotOkWithCode(t, changePassReq, http.StatusForbidden)
	})

	t.Run("Must be able to change password", func(t *testing.T) {
		changePassReq := getAuthRequest(t, AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/auth/change_password",
			Method:      "PUT",
			Body: fmt.Sprintf(`{
			"oldPassword": %q,
			"newPassword": %q,
			"repeatedNewPassword": %q
		}`, pass, newPass, newPass),
		})
		assertResponseOk(t, changePassReq)

		// should be possible to log in with new password
		loginReq := getRequest(t, RequestArgs{
			Path:   "/login",
			Method: "POST",
			Body: fmt.Sprintf(`{
				"email": %q,
				"password": %q
			}`, email, newPass),
		})
		assertResponseOk(t, loginReq)

		// using old password should not suceed
		badLoginReq := getRequest(t, RequestArgs{
			Path:   "/login",
			Method: "POST",
			Body: fmt.Sprintf(`{
				"email": %q,
				"password": %q
			}`, email, pass),
		})
		assertResponseNotOk(t, badLoginReq)

	})

	t.Run("Must not be able to change the password by providing a bad old password", func(t *testing.T) {
		badPass := gofakeit.Password(true, true, true, true, true, 32)
		changePassReq := getAuthRequest(t, AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/auth/change_password",
			Method:      "PUT",
			Body: fmt.Sprintf(`{
			"oldPassword": %q,
			"newPassword": %q,
			"repeatedNewPassword": %q
		}`, badPass, newPass, newPass),
		})

		assertResponseNotOkWithCode(t, changePassReq, http.StatusForbidden)
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

	t.Run("Should not be able to reset the user password by using a bad token", func(t *testing.T) {
		badSecretKey := []byte("this is a secret key which we expect to fail")
		badToken := passwordreset.NewToken(email, users.PasswordResetTokenDuration,
			user.HashedPassword, badSecretKey)
		badTokenReq := getRequest(t, RequestArgs{
			Path:   "/auth/reset_password",
			Method: "PUT",
			Body: fmt.Sprintf(`{
				"token": %q,
				"password": %q
			}`, badToken, newPass),
		})
		assertResponseNotOkWithCode(t, badTokenReq, http.StatusForbidden)

		// we should be able to log in with old credentials
		loginReq := getRequest(t, RequestArgs{
			Path:   "/login",
			Method: "POST",
			Body: fmt.Sprintf(`{
				"password": %q,
				"email": %q
			}`, pass, email),
		})
		assertResponseOk(t, loginReq)

		// we should NOT be able to log in with new credentials
		badLoginReq := getRequest(t, RequestArgs{
			Path:   "/login",
			Method: "POST",
			Body: fmt.Sprintf(`{
				"password": %q,
				"email": %q
			}`, newPass, email),
		})
		assertResponseNotOk(t, badLoginReq)
	})

	t.Run("Reset the password by using the correct token", func(t *testing.T) {
		token, err := users.GetPasswordResetToken(testDB, user.Email)
		if err != nil {
			testutil.FatalMsgf(t, "Could not password reset token: %v", err)
		}
		resetPassReq := getRequest(t, RequestArgs{
			Path:   "/auth/reset_password",
			Method: "PUT",
			Body: fmt.Sprintf(`{
				"token": %q,
				"password": %q
			}`, token, newPass),
		})
		assertResponseOk(t, resetPassReq)

		// we should be able to log in with new credentials
		loginReq := getRequest(t, RequestArgs{
			Path:   "/login",
			Method: "POST",
			Body: fmt.Sprintf(`{
				"password": %q,
				"email": %q
			}`, newPass, email),
		})
		assertResponseOk(t, loginReq)

		// we should NOT be able to log in with old credentials
		badLoginReq := getRequest(t, RequestArgs{
			Path:   "/login",
			Method: "POST",
			Body: fmt.Sprintf(`{
				"password": %q,
				"email": %q
			}`, pass, email),
		})
		assertResponseNotOk(t, badLoginReq)
	})

	t.Run("Should not be able to reset the password twice", func(t *testing.T) {
		token, err := users.GetPasswordResetToken(testDB, user.Email)
		if err != nil {
			testutil.FatalMsgf(t, "Could not password reset token: %v", err)
		}
		resetPassReq := getRequest(t, RequestArgs{
			Path:   "/auth/reset_password",
			Method: "PUT",
			Body: fmt.Sprintf(`{
				"token": %q,
				"password": %q
			}`, token, newPass),
		})
		assertResponseOk(t, resetPassReq)

		yetAnotherNewPass := gofakeit.Password(true, true, true, true, true, 32)
		secondResetPassReq := getRequest(t, RequestArgs{
			Path:   "/auth/reset_password",
			Method: "PUT",
			Body: fmt.Sprintf(`{
				"token": %q,
				"password": %q
			}`, token, yetAnotherNewPass),
		})
		assertResponseNotOkWithCode(t, secondResetPassReq, http.StatusForbidden)

	})
}

func TestPutUserRoute(t *testing.T) {
	testutil.DescribeTest(t)

	email := gofakeit.Email()
	password := gofakeit.Password(true, true, true, true, true, 32)
	accessToken := createAndLoginUser(t, users.CreateUserArgs{
		Email:    email,
		Password: password,
	})

	t.Run("updating with an invalid email should fail", func(t *testing.T) {
		req := getAuthRequest(t, AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/user",
			Method:      "PUT",
			Body: `{
				"email": "bad-email.coming.through"
			}`,
		})
		assertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})

	t.Run("update email, first name and last name", func(t *testing.T) {
		newFirst := "new-firstname"
		newLast := "new-lastname"
		newEmail := gofakeit.Email()

		// Update User endpoint
		updateUserReq := getAuthRequest(t,
			AuthRequestArgs{
				AccessToken: accessToken,
				Path:        "/user", Method: "PUT",
				Body: fmt.Sprintf(`
			{
				"firstName": %q,
				"lastName": %q,
				"email": %q
			}`, newFirst, newLast, newEmail)})

		jsonRes := assertResponseOkWithJson(t, updateUserReq)
		testutil.AssertEqual(t, jsonRes["firstName"], newFirst)
		testutil.AssertEqual(t, jsonRes["lastName"], newLast)
		testutil.AssertEqual(t, jsonRes["email"], newEmail)

		// Get User endpoint
		getUserReq := getAuthRequest(t,
			AuthRequestArgs{
				AccessToken: accessToken,
				Method:      "GET", Path: "/user",
			})

		// Verify that update and get returns the same
		jsonRes = assertResponseOkWithJson(t, getUserReq)
		testutil.AssertEqual(t, jsonRes["firstName"], newFirst)
		testutil.AssertEqual(t, jsonRes["lastName"], newLast)
		testutil.AssertEqual(t, jsonRes["email"], newEmail)
	})
}

func TestSendPasswordResetEmail(t *testing.T) {
	email := gofakeit.Email()
	createAndLoginUser(t, users.CreateUserArgs{
		Email:    email,
		Password: gofakeit.Password(true, true, true, true, true, 32),
	})

	req := getRequest(t, RequestArgs{
		Path:   "/auth/reset_password",
		Method: "POST",
		Body: fmt.Sprintf(`{
			"email": %q
		}`, email),
	})
	assertResponseOk(t, req)
	testutil.AssertMsg(t, mockSendGridClient.SentEmails > 0, "Sendgrid client didn't send any emails!")
}

// When called, this switches out the app used to serve our requests with on
// that actually calls out to LND. It returns a func that undoes this, i.e.
// a func you can defer at the start of your test
func runTestWithRealLnd(t *testing.T) func() {
	t.Helper()

	app = &realLndApp
	return func() {
		app = &mockLndApp
	}
}

func TestCreateInvoiceRoute(t *testing.T) {
	testutil.DescribeTest(t)
	testutil.SkipIfCI(t)

	lnCleanup := runTestWithRealLnd(t)
	defer lnCleanup()

	accessToken := createAndLoginUser(t, users.CreateUserArgs{
		Email:    gofakeit.Email(),
		Password: "password",
	})

	t.Run("Create an invoice without memo and description", func(t *testing.T) {
		testutil.DescribeTest(t)

		amountSat := gofakeit.Number(0,
			int(payments.MaxAmountSatPerInvoice))

		req := getAuthRequest(t,
			AuthRequestArgs{
				AccessToken: accessToken,
				Path:        "/invoices/create",
				Method:      "POST",
				Body: fmt.Sprintf(`{
					"amountSat": %d
				}`, amountSat),
			})

		res := assertResponseOkWithJson(t, req)
		testutil.AssertMsg(t, res["memo"] == nil, "Memo was not empty")
		testutil.AssertMsg(t, res["description"] == nil, "Description was not empty")

	})

	t.Run("Create an invoice with memo and description", func(t *testing.T) {
		testutil.DescribeTest(t)

		amountSat := gofakeit.Number(0,
			int(payments.MaxAmountSatPerInvoice))

		memo := gofakeit.Sentence(gofakeit.Number(1, 20))
		description := gofakeit.Sentence(gofakeit.Number(1, 20))

		req := getAuthRequest(t,
			AuthRequestArgs{
				AccessToken: accessToken,
				Path:        "/invoices/create",
				Method:      "POST",
				Body: fmt.Sprintf(`{
					"amountSat": %d,
					"memo": %q,
					"description": %q
				}`, amountSat, memo, description),
			})

		res := assertResponseOkWithJson(t, req)
		testutil.AssertEqual(t, res["memo"], memo)
		testutil.AssertEqual(t, res["description"], description)

	})

	t.Run("Not create an invoice with non-positive amount ", func(t *testing.T) {
		testutil.DescribeTest(t)

		// gofakeit panics with too low value here...
		// https://github.com/brianvoe/gofakeit/issues/56
		amountSat := gofakeit.Number(math.MinInt64+2, -1)

		req := getAuthRequest(t,
			AuthRequestArgs{
				AccessToken: accessToken,
				Path:        "/invoices/create",
				Method:      "POST",
				Body: fmt.Sprintf(`{
					"amountSat": %d
				}`, amountSat),
			})

		assertResponseNotOk(t, req)
	})

	t.Run("Not create an invoice with too large amount", func(t *testing.T) {
		testutil.DescribeTest(t)

		amountSat := gofakeit.Number(
			int(payments.MaxAmountSatPerInvoice), math.MaxInt64)

		req := getAuthRequest(t,
			AuthRequestArgs{
				AccessToken: accessToken,
				Path:        "/invoices/create",
				Method:      "POST",
				Body: fmt.Sprintf(`{
					"amountSat": %d
				}`, amountSat),
			})

		assertResponseNotOk(t, req)
	})

	t.Run("Not create an invoice with zero amount ", func(t *testing.T) {
		testutil.DescribeTest(t)

		req := getAuthRequest(t,
			AuthRequestArgs{
				AccessToken: accessToken,
				Path:        "/invoices/create",
				Method:      "POST",
				Body: `{
					"amountSat": 0
				}`,
			})

		assertResponseNotOk(t, req)

	})
}

func TestRestServer_EnableConfirmAndDelete2fa(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)

	email := gofakeit.Email()
	password := gofakeit.Password(true, true, true, true, true, 32)
	accessToken := createAndLoginUser(t, users.CreateUserArgs{
		Email:    email,
		Password: password,
	})

	t.Run("must have access token to enable 2FA", func(t *testing.T) {
		req := getRequest(t, RequestArgs{
			Path:   "/auth/2fa",
			Method: "POST",
		})
		assertResponseNotOkWithCode(t, req, http.StatusForbidden)
	})

	t.Run("enable 2FA", func(t *testing.T) {
		req := getAuthRequest(t, AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/auth/2fa",
			Method:      "POST",
		})

		assertResponseOk(t, req)

		user, err := users.GetByEmail(testDB, email)
		if err != nil {
			testutil.FatalMsg(t, err)
		}
		testutil.AssertMsg(t, user.TotpSecret != nil, "TOTP secret was nil!")
		testutil.AssertMsg(t, !user.ConfirmedTotpSecret, "User confirmed TOTP secret!")

		t.Run("fail to confirm 2FA with bad code", func(t *testing.T) {
			req := getAuthRequest(t, AuthRequestArgs{
				AccessToken: accessToken,
				Path:        "/auth/2fa",
				Method:      "PUT",
				Body: `{
					"code": "123456"
				}`,
			})

			assertResponseNotOkWithCode(t, req, http.StatusForbidden)
		})

		t.Run("confirm 2FA", func(t *testing.T) {

			code, err := totp.GenerateCode(*user.TotpSecret, time.Now())
			if err != nil {
				testutil.FatalMsg(t, err)
			}

			req := getAuthRequest(t, AuthRequestArgs{
				AccessToken: accessToken,
				Path:        "/auth/2fa",
				Method:      "PUT",
				Body: fmt.Sprintf(`{
					"code": %q
				}`, code),
			})

			assertResponseOk(t, req)

			t.Run("fail to confirm 2FA twice", func(t *testing.T) {
				code, err := totp.GenerateCode(*user.TotpSecret, time.Now())
				if err != nil {
					testutil.FatalMsg(t, err)
				}

				req := getAuthRequest(t, AuthRequestArgs{
					AccessToken: accessToken,
					Path:        "/auth/2fa",
					Method:      "PUT",
					Body: fmt.Sprintf(`{
					"code": %q
				}`, code),
				})

				assertResponseNotOkWithCode(t, req, http.StatusBadRequest)
			})

			t.Run("should need TOTP code for login", func(t *testing.T) {
				req := getRequest(t, RequestArgs{
					Path:   "/login",
					Method: "POST",
					Body: fmt.Sprintf(`{
						"email": %q,
						"password": %q
					}`, email, password),
				})
				assertResponseNotOkWithCode(t, req, http.StatusBadRequest)
			})

			t.Run("should be able to login with TOTP code", func(t *testing.T) {
				code, err := totp.GenerateCode(*user.TotpSecret, time.Now())
				if err != nil {
					testutil.FatalMsg(t, err)
				}
				req := getRequest(t, RequestArgs{
					Path:   "/login",
					Method: "POST",
					Body: fmt.Sprintf(`{
						"email": %q,
						"password": %q,
						"totp": %q
					}`, email, password, code),
				})
				assertResponseOk(t, req)
			})

			t.Run("don't delete 2FA credentials with an invalid code", func(t *testing.T) {
				req := getAuthRequest(t, AuthRequestArgs{
					AccessToken: accessToken,
					Path:        "/auth/2fa",
					Method:      "DELETE",
					Body: `{
						"code": "123456"
					}`,
				})
				assertResponseNotOkWithCode(t, req, http.StatusForbidden)
			})

			t.Run("delete 2FA credentials", func(t *testing.T) {
				code, err := totp.GenerateCode(*user.TotpSecret, time.Now())
				if err != nil {
					testutil.FatalMsg(t, err)
				}

				deleteReq := getAuthRequest(t, AuthRequestArgs{
					AccessToken: accessToken,
					Path:        "/auth/2fa",
					Method:      "DELETE",
					Body: fmt.Sprintf(`{
						"code": %q
					}`, code),
				})

				assertResponseOk(t, deleteReq)

				loginReq := getRequest(t, RequestArgs{
					Path:   "/login",
					Method: "POST",
					Body: fmt.Sprintf(`{
						"email": %q,
						"password": %q
					}`, email, password),
				})
				assertResponseOk(t, loginReq)

				t.Run("fail to delete already deleted 2FA credentials", func(t *testing.T) {
					code, err := totp.GenerateCode(*user.TotpSecret, time.Now())
					if err != nil {
						testutil.FatalMsg(t, err)
					}
					req := getAuthRequest(t, AuthRequestArgs{
						AccessToken: accessToken,
						Path:        "/auth/2fa",
						Method:      "DELETE",
						Body: fmt.Sprintf(`{
							"code": %q
						}`, code),
					})
					assertResponseNotOkWithCode(t, req, http.StatusBadRequest)
				})
			})

		})
	})
}
