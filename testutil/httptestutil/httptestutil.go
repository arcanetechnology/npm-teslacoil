package httptestutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"

	"gitlab.com/arcanecrypto/teslacoil/internal/users"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
)

// Server is something that can serve HTTP requests
type Server interface {
	ServeHTTP(response http.ResponseWriter, request *http.Request)
}

// TestHarness is a structure that allows us to execute tests that need
// HTTP serving capabilities, as well as other potential external services.
// At the moment this only includes HTTP serving, but in the future this
// is likely to expand.
type TestHarness struct {
	server Server
}

func NewTestHarness(server Server) TestHarness {
	return TestHarness{server: server}
}

// Checks if the given string is valid JSON
func isJSONString(s string) bool {
	var js interface{}
	err := json.Unmarshal([]byte(s), &js)
	return err == nil
}

func (harness *TestHarness) CreateUser(t *testing.T, args users.CreateUserArgs) map[string]interface{} {
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

	createUserRequest := GetRequest(t, RequestArgs{
		Path:   "/users",
		Method: "POST",
		Body: fmt.Sprintf(`{
			"email": %q,
			"password": %q,
			"firstName": %s,
			"lastName": %s
		}`, args.Email, args.Password, firstName, lastName),
	})

	return harness.AssertResponseOkWithJson(t, createUserRequest)
}

type AuthRequestArgs struct {
	AccessToken string
	Path        string
	Method      string
	Body        string
}

// GetAuthRequest returns a HTTP request that carries a proper auth header
// and an optional JSON body
func GetAuthRequest(t *testing.T, args AuthRequestArgs) *http.Request {
	t.Helper()
	if args.AccessToken == "" {
		testutil.FatalMsg(t, "You forgot to set AccessToken")
	}
	req := GetRequest(t, RequestArgs{Path: args.Path,
		Method: args.Method, Body: args.Body})
	req.Header.Set("Authorization", args.AccessToken)
	return req
}

type RequestArgs struct {
	Path   string
	Method string
	Body   string
}

// GetRequest returns a HTTP request with an optional JSON body
func GetRequest(t *testing.T, args RequestArgs) *http.Request {
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

// TODO Assert that if failure, contains reasonably shaped JSON
func (harness *TestHarness) AssertResponseNotOk(t *testing.T, request *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	harness.server.ServeHTTP(response, request)
	if response.Code < 300 {
		testutil.FatalMsgf(t, "Got success code (%d) on path %s", response.Code, extractMethodAndPath(request))
	}
	return response
}

// AssertResponseNotOkWithCode checks that the given request results in the
// given HTTP status code. It returns the response to the request.
func (harness *TestHarness) AssertResponseNotOkWithCode(t *testing.T, request *http.Request, code int) *httptest.ResponseRecorder {
	testutil.AssertMsgf(t, code >= 100 && code <= 500, "Given code (%d) is not a valid HTTP code", code)
	t.Helper()

	response := harness.AssertResponseNotOk(t, request)
	testutil.AssertMsgf(t, response.Code == code,
		"Expected code (%d) does not match with found code (%d)", code, response.Code)
	return response
}

// First performs `assertResponseOk`, then asserts that the body of the response
// can be parsed as JSON, and then returns the parsed JSON
func (harness *TestHarness) AssertResponseOkWithJson(t *testing.T, request *http.Request) map[string]interface{} {

	t.Helper()
	response := harness.AssertResponseOk(t, request)
	var destination map[string]interface{}

	if err := json.Unmarshal(response.Body.Bytes(), &destination); err != nil {
		stringBody := response.Body.String()
		testutil.FatalMsgf(t, "%+v. Body: %s ",
			err, stringBody)

	}
	return destination
}

func extractMethodAndPath(req *http.Request) string {
	return req.Method + " " + req.URL.Path
}

// Performs the given request against the API. Asserts that the
// response completed successfully. Returns the response from the API
// TODO Assert that if failure, contains reasonably shaped JSON
func (harness *TestHarness) AssertResponseOk(t *testing.T, request *http.Request) *httptest.ResponseRecorder {
	t.Helper()

	var bodyBytes []byte
	var err error
	if request.Body != nil {
		// read the body bytes for potential error messages later
		bodyBytes, err = ioutil.ReadAll(request.Body)
		if err != nil {
			testutil.FatalMsgf(t, "Could not read body: %v", err)
		}
		// restore the original buffer so it can be read later
		request.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	response := httptest.NewRecorder()
	harness.server.ServeHTTP(response, request)

	if response.Code >= 300 {
		testutil.FatalMsgf(t, "Got failure code (%d) on path %s: %s",
			response.Code, extractMethodAndPath(request), response.Body.String())
	}

	return response
}

// AssertResponseOKWithStruct attempts to unmarshal the body into the
// struct passed as an argument. third argument MUST be a pointer to a
// struct
func (harness *TestHarness) AssertResponseOKWithStruct(t *testing.T, body *bytes.Buffer, s interface{}) {
	t.Helper()

	err := json.Unmarshal(body.Bytes(), s)
	if err != nil {
		t.Fatalf("could not unmarshal body into %+v", s)
	}
}

// Creates and and authenticates a user with the given email and password.
// We either log in (and return an access token), or create an API key (and
// return that). They should be equivalent (until scopes are implemented, so
// this should not matter and might uncover some edge cases.
func (harness *TestHarness) CreateAndAuthenticateUser(t *testing.T, args users.CreateUserArgs) string {
	_ = harness.CreateUser(t, args)

	loginUserReq := GetRequest(t, RequestArgs{
		Path:   "/login",
		Method: "POST",
		Body: fmt.Sprintf(`{
			"email": %q,
			"password": %q
		}`, args.Email, args.Password),
	})

	fail := func(json map[string]interface{}, key, methodAndPath string) {
		testutil.FatalMsgf(t, "Returned JSON (%+v) did not have string property '%s'. Path: %s",
			json, key, methodAndPath)
	}

	jsonRes := harness.AssertResponseOkWithJson(t, loginUserReq)

	tokenPath := "accessToken"
	maybeNilToken, ok := jsonRes[tokenPath]
	if !ok {
		fail(jsonRes, tokenPath, extractMethodAndPath(loginUserReq))
	}

	var token string
	switch untypedToken := maybeNilToken.(type) {
	case string:
		token = untypedToken
	default:
		fail(jsonRes, tokenPath, extractMethodAndPath(loginUserReq))
	}

	// we want to alternate between authenticating users with an API key or
	// a JWT. We flip a coin here, and if the coin flip succeeds we create an
	// API key and return that.
	coinFlip := rand.Float32()
	if coinFlip < 0.5 {
		return token
	}

	apiKeyRequest := GetAuthRequest(t, AuthRequestArgs{
		Path:        "/apikey",
		Method:      "POST",
		AccessToken: token,
	})
	apiKeyJson := harness.AssertResponseOkWithJson(t, apiKeyRequest)
	apiKeyPath := "key"
	maybeNilKey, ok := apiKeyJson[apiKeyPath]
	if !ok {
		fail(apiKeyJson, apiKeyPath, extractMethodAndPath(apiKeyRequest))
	}

	switch untypedKey := maybeNilKey.(type) {
	case string:
		return untypedKey
	default:
		fail(apiKeyJson, apiKeyPath, extractMethodAndPath(apiKeyRequest))
		// won't reach this
		panic("unreachable")
	}

}
