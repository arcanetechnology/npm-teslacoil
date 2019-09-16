package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/brianvoe/gofakeit"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
)

var (
	databaseConfig = testutil.GetDatabaseConfig("routes")
	testDB         *db.DB
	conf           = Config{LogLevel: logrus.InfoLevel}
	app            RestServer
)

func TestMain(m *testing.M) {
	build.SetLogLevel(logrus.InfoLevel)
	testDB = testutil.InitDatabase(databaseConfig)

	var err error
	app, err = NewApp(testDB, testutil.GetLightningMockClient(), conf)
	if err != nil {
		panic(err.Error())
	}

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

// Creates and logs in a user with the given email and password. Returns
// the access token for this session.
func createAndLoginUser(t *testing.T, email, password string) string {
	createUserRequest := getRequest(t, RequestArgs{
		Path:   "/users",
		Method: "POST",
		Body: fmt.Sprintf(`{
			"email": %q,
			"password": %q
		}`, email, password),
	})

	assertResponseOk(t, createUserRequest)
	loginUserReq := getRequest(t, RequestArgs{
		Path:   "/login",
		Method: "POST",
		Body: fmt.Sprintf(`{
			"email": %q,
			"password": %q
		}`, email, password),
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
	accessToken := createAndLoginUser(t, email, "password")

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

func TestPutUserRoute(t *testing.T) {
	testutil.DescribeTest(t)

	accessToken := createAndLoginUser(t, "foobar", "password")

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
