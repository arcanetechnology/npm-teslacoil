package httptestutil

import (
	"bytes"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/pkg/errors"
	"gitlab.com/arcanecrypto/teslacoil/internal/apierr"
	"gitlab.com/arcanecrypto/teslacoil/internal/auth"
	"gitlab.com/arcanecrypto/teslacoil/internal/httptypes"
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
	randomKey, err := rsa.GenerateKey(cryptorand.Reader, 4096)
	if err != nil {
		panic(errors.Wrap(err, "could not generate RSA key"))
	}

	auth.SetJwtPrivateKey(randomKey)

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

// Word that starts with ERR_ and only contains A-Z, _ or digits
var uppercaseAndUnderScoreRegex = regexp.MustCompile("^ERR_([A-Z]|_|[0-9])+$")

func assertErrorIsOk(t *testing.T, response *httptest.ResponseRecorder) (*httptest.ResponseRecorder, httptypes.StandardErrorResponse) {

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		testutil.FatalMsg(t, errors.Wrap(err, "could not read body"))
	}
	var parsed httptypes.StandardErrorResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		testutil.FatalMsg(t, errors.Wrap(err, "could not parse body into StandardResponse"))
	}
	testutil.AssertMsgf(t, parsed.ErrorField.Message != "", "Error message was empty! JSON: %s", body)
	testutil.AssertMsgf(t, parsed.ErrorField.Code != "", "Error code was empty! JSON: %s", body)
	testutil.AssertMsgf(t, uppercaseAndUnderScoreRegex.MatchString(parsed.ErrorField.Code), "Code didn't match regex! Code: %s", parsed.ErrorField.Code)

	testutil.AssertMsg(t, !stderrors.Is(parsed, apierr.ErrUnknownError), "Error was ErrUnknownError! We should always make sure we're setting a sensible error")

	testutil.AssertMsg(t, parsed.ErrorField.Fields != nil, "Fields was nil! Expected at least empty list")
	for _, field := range parsed.ErrorField.Fields {
		testutil.AssertMsgf(t, field.Code != apierr.UnknownValidationTag, "Encountered unknown validation tag %q! We should make sure all valiation tags get a nice error message.", field.Code)
	}
	return response, parsed
}

// Asserts that the given request fails, and that it conforms to our
// expected error format.
func (harness *TestHarness) AssertResponseNotOk(t *testing.T, request *http.Request) (*httptest.ResponseRecorder, httptypes.StandardErrorResponse) {
	t.Helper()
	response := httptest.NewRecorder()
	harness.server.ServeHTTP(response, request)
	if response.Code < 300 {
		testutil.FailMsgf(t, "Got success code (%d) on path %s", response.Code, extractMethodAndPath(request))
	}

	return assertErrorIsOk(t, response)

}

// AssertResponseNotOkWithCode checks that the given request results in the
// given HTTP status code. It returns the response to the request.
func (harness *TestHarness) AssertResponseNotOkWithCode(t *testing.T, request *http.Request, code int) (*httptest.ResponseRecorder, httptypes.StandardErrorResponse) {
	testutil.AssertMsgf(t, code >= 100 && code <= 500, "Given code (%d) is not a valid HTTP code", code)
	t.Helper()

	response, error := harness.AssertResponseNotOk(t, request)
	testutil.AssertMsgf(t, response.Code == code,
		"Expected code (%d) does not match with found code (%d)", code, response.Code)
	return response, error
}

func (harness *TestHarness) AssertResponseOkWithBody(t *testing.T, request *http.Request) bytes.Buffer {
	t.Helper()
	response := harness.AssertResponseOk(t, request)

	testutil.AssertMsg(t, response.Body.Len() != 0, "Body was empty!")

	return *response.Body
}

// First performs `assertResponseOk`, then asserts that the body of the response
// can be parsed as JSON, and then returns the parsed JSON
func (harness *TestHarness) AssertResponseOkWithJson(t *testing.T, request *http.Request) map[string]interface{} {
	t.Helper()
	var destination map[string]interface{}
	harness.AssertResponseOKWithStruct(t, request, &destination)
	if err, ok := destination["error"]; ok {
		testutil.FailMsgf(t, `JSON body had field named "error": %v`, err)
	}
	return destination
}

// First performs `assertResponseOk`, then asserts that the body of the response
// can be parsed as a JSON list, and then returns the parsed JSON list
func (harness *TestHarness) AssertResponseOkWithJsonList(t *testing.T, request *http.Request) []map[string]interface{} {
	t.Helper()

	var destination []map[string]interface{}
	harness.AssertResponseOKWithStruct(t, request, &destination)

	return destination

}

func extractMethodAndPath(req *http.Request) string {
	return req.Method + " " + req.URL.Path
}

// Performs the given request against the API. Asserts that the
// response completed successfully. Returns the response from the API
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
		testutil.FailMsgf(t, "Got failure code (%d) on path %s: %s",
			response.Code, extractMethodAndPath(request), response.Body.String())
		_, _ = assertErrorIsOk(t, response)
	}

	return response
}

// AssertResponseOKWithStruct attempts to unmarshal the body into the
// struct passed as an argument. third argument MUST be a pointer to a
// struct
func (harness *TestHarness) AssertResponseOKWithStruct(t *testing.T, request *http.Request, s interface{}) {
	t.Helper()

	response := harness.AssertResponseOkWithBody(t, request)

	if err := json.Unmarshal(response.Bytes(), s); err != nil {
		testutil.FailMsg(t, errors.Wrap(err, "could not unmarshal JSON"))
	}
}

func (harness *TestHarness) AuthenticaticateUser(t *testing.T, args users.CreateUserArgs) string {

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

// Creates and and authenticates a user with the given email and password.
// We either log in (and return an access token), or create an API key (and
// return that). They should be equivalent (until scopes are implemented, so
// this should not matter and might uncover some edge cases.
func (harness *TestHarness) CreateAndAuthenticateUser(t *testing.T, args users.CreateUserArgs) string {
	_ = harness.CreateUser(t, args)

	return harness.AuthenticaticateUser(t, args)

}
