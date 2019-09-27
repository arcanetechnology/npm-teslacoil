package httptestutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
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

	bodyBytes := []byte{}
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

	if response.Code != 200 {
		var parsedJsonBody string

		// this is a bit strange way of doing things, but we unmarshal and then
		// marshal again to get rid of any weird formatting, so we can print
		// the JSON body in a compact, one-line way
		var jsonDest map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &jsonDest); err != nil {
			testutil.FatalMsgf(t, "Could not unmarshal JSON: %v. Body: %s", err, string(bodyBytes))
		}
		jsonBytes, err := json.Marshal(jsonDest)
		parsedJsonBody = string(jsonBytes)
		if err != nil {
			testutil.FatalMsgf(t, "Could not marshal JSON: %v", err)
		}

		testutil.FatalMsgf(t, "Got failure code (%d) on path %s %s",
			response.Code, extractMethodAndPath(request), string(parsedJsonBody))
	}

	return response
}

// Creates and logs in a user with the given email and password. Returns
// the access token for this session.
func (harness *TestHarness) CreateAndLoginUser(t *testing.T, args users.CreateUserArgs) string {
	_ = harness.CreateUser(t, args)

	loginUserReq := GetRequest(t, RequestArgs{
		Path:   "/login",
		Method: "POST",
		Body: fmt.Sprintf(`{
			"email": %q,
			"password": %q
		}`, args.Email, args.Password),
	})

	jsonRes := harness.AssertResponseOkWithJson(t, loginUserReq)

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
