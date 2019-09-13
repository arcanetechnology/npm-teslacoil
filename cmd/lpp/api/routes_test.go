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
	"gitlab.com/arcanecrypto/teslacoil/internal/users"
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

// Returns JSON you can send to create a user
func getCreateUserBody(email string, password string) string {
	return fmt.Sprintf(`{ "email": "%s", "password": "%s" }`, email, password)
}

// returns JSON you can send to login a user
func getLoginBody(email string, password string) string {
	return fmt.Sprintf(`{ "email": "%s", "password": "%s" }`, email, password)
}

// Returns valid JSON you can send to the API to update an user
func getUpdateUserBody(opts users.UpdateOptions) string {
	var email string
	var firstName string
	var lastName string
	if opts.NewEmail != nil {
		email = `"` + *opts.NewEmail + `"`
	} else {
		email = "null"
	}
	if opts.NewFirstName != nil {
		firstName = `"` + *opts.NewFirstName + `"`
	} else {
		firstName = "null"
	}
	if opts.NewLastName != nil {
		lastName = `"` + *opts.NewLastName + `"`
	} else {
		lastName = "null"
	}
	return fmt.Sprintf(`{ "email": %s, "firstName": %s, "lastName": %s }`,
		email, firstName, lastName)
}

func stringToBuffer(str string) *bytes.Buffer {
	return bytes.NewBuffer([]byte(str))
}

// Performs the given request against the API. Asserts that the
// response completed successfully. Returns the response from the API
// TODO Assert that if failure, contains reasonably shaped JSON
func assertResponseOk(t *testing.T, request *http.Request) *httptest.ResponseRecorder {
	t.Helper()

	response := httptest.NewRecorder()
	app.Router.ServeHTTP(response, request)

	if response.Code != 200 {
		testutil.FailMsgf(t, "Got failure code on path %s", extractMethodAndPath(request))
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
	t.Helper()

	createUserRequest, _ := http.NewRequest(
		"POST", "/users",
		stringToBuffer(getCreateUserBody(email, password)))
	assertResponseOk(t, createUserRequest)

	loginUserReq := httptest.NewRequest(
		"POST", "/login",
		stringToBuffer(getLoginBody(email, password)))

	json := assertResponseOkWithJson(t, loginUserReq)

	tokenPath := "accessToken"
	fail := func() {
		methodAndPath := extractMethodAndPath(loginUserReq)
		testutil.FatalMsgf(t, "Returned JSON (%+v) did have string property '%s'. Path: %s",
			json, tokenPath, methodAndPath)
	}

	maybeNilToken, ok := json[tokenPath]
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

func TestPostUsersRoute(t *testing.T) {
	testutil.DescribeTest(t)

	email := gofakeit.Email()
	accessToken := createAndLoginUser(t, email, "password")

	req := getAuthRequest(t,
		AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/user", Method: "GET",
		})

	json := assertResponseOkWithJson(t, req)
	testutil.AssertEqual(t, json["firstName"], nil)
	testutil.AssertEqual(t, json["lastName"], nil)
	testutil.AssertEqual(t, json["email"], email)

}

func TestPutUserRoute(t *testing.T) {
	testutil.DescribeTest(t)

	accessToken := createAndLoginUser(t, "foobar", "password")

	newFirst := "new-firstname"
	newLast := "new-lastname"
	newEmail := "new-email"
	jsonBody := getUpdateUserBody(users.UpdateOptions{
		NewFirstName: &newFirst,
		NewLastName:  &newLast,
		NewEmail:     &newEmail,
	})

	// Update User endpoint
	updateUserReq := getAuthRequest(t,
		AuthRequestArgs{
			AccessToken: accessToken, Body: jsonBody,
			Path: "/user", Method: "PUT",
		})

	json := assertResponseOkWithJson(t, updateUserReq)
	testutil.AssertEqual(t, json["firstName"], newFirst)
	testutil.AssertEqual(t, json["lastName"], newLast)
	testutil.AssertEqual(t, json["email"], newEmail)

	// Get User endpoint
	getUserReq := getAuthRequest(t,
		AuthRequestArgs{
			AccessToken: accessToken,
			Method:      "GET", Path: "/user",
		})

	// Verify that update and get returns the same
	json = assertResponseOkWithJson(t, getUserReq)
	testutil.AssertMapEquals(t, map[string]interface{}{
		"firstName": newFirst,
		"lastName":  newLast,
		"email":     newEmail,
	}, json)

}
