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

	"github.com/brianvoe/gofakeit"
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
)

func TestMain(m *testing.M) {
	build.SetLogLevel(logrus.InfoLevel)
	testDB = testutil.InitDatabase(databaseConfig)

	// new values for gofakeit every time
	gofakeit.Seed(0)

	var err error
	mockLndApp, err = NewApp(testDB, testutil.GetLightningMockClient(), conf)
	if err != nil {
		panic(err.Error())
	}

	// this is not good, but a workaround until we have a proper testing/CI
	// harness with nodes and the whole shebang
	if os.Getenv("CI") != "" {
		realLndApp, err = NewApp(testDB, testutil.GetLightningMockClient(), conf)
		if err != nil {
			panic(err.Error())
		}
	} else {
		lndConfig := testutil.GetLightingConfig()
		lnd, err := ln.NewLNDClient(lndConfig)
		if err != nil {
			panic(err.Error())
		}

		realLndApp, err = NewApp(testDB, lnd, conf)
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
		assertResponseNotOk(t, req)

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
		assertResponseNotOk(t, req)
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
}

func TestPutUserRoute(t *testing.T) {
	testutil.DescribeTest(t)

	accessToken := createAndLoginUser(t, users.CreateUserArgs{
		Email:    "foobar",
		Password: "password",
	})

	newFirst := "new-firstname"
	newLast := "new-lastname"
	newEmail := "new-email"

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

		// gofakeit panics with too low value here...
		// https://github.com/brianvoe/gofakeit/issues/56

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
