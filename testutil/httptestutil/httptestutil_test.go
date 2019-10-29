package httptestutil

import (
	"net/http"
	"testing"

	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
)

var emptyDb = &db.DB{}

type badJson struct{}

func (s badJson) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if _, err := response.Write([]byte(`-----`)); err != nil {
		panic(err)
	}
}

type goodObject struct{}

func (s goodObject) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if _, err := response.Write([]byte(`{
		"foo": "bar"
	}`)); err != nil {
		panic(err)
	}
}

type goodList struct{}

func (s goodList) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if _, err := response.Write([]byte(`[]`)); err != nil {
		panic(err)
	}
}

type badServer struct{}

func (s badServer) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if _, err := response.Write([]byte(`{
		"error": "bar"
	}`)); err != nil {
		panic(err)
	}
}

func TestTestHarness_AssertResponseOkWithJson(t *testing.T) {
	t.Run("accept a normal JSON body", func(t *testing.T) {
		server := goodObject{}
		h := NewTestHarness(server, emptyDb)
		req := GetRequest(t, RequestArgs{
			Path:   "/ping",
			Method: "GET",
		})
		h.AssertResponseOkWithJson(t, req)
	})

	t.Run(`fail with a JSON body containing "error"`, func(t *testing.T) {
		server := badServer{}
		h := NewTestHarness(server, emptyDb)
		req := GetRequest(t, RequestArgs{
			Path:   "/ping",
			Method: "GET",
		})
		testThatShouldFail := testing.T{}
		h.AssertResponseOkWithJson(&testThatShouldFail, req)
		testutil.AssertMsg(t, testThatShouldFail.Failed(), "Test didn't fail with bad response")
	})

	t.Run(`fail with invalid JSON`, func(t *testing.T) {
		server := badJson{}
		h := NewTestHarness(server, emptyDb)
		req := GetRequest(t, RequestArgs{
			Path:   "/ping",
			Method: "GET",
		})
		testThatShouldFail := testing.T{}
		h.AssertResponseOkWithJson(&testThatShouldFail, req)
		testutil.AssertMsg(t, testThatShouldFail.Failed(), "Test didn't fail with bad response")
	})
}

func TestTestHarness_AssertResponseOkWithJsonList(t *testing.T) {
	t.Run("accept a normal JSON body", func(t *testing.T) {
		server := goodList{}
		h := NewTestHarness(server, emptyDb)
		req := GetRequest(t, RequestArgs{
			Path:   "/ping",
			Method: "GET",
		})
		h.AssertResponseOkWithJsonList(t, req)
	})

	t.Run(`fail with invalid JSON`, func(t *testing.T) {
		server := badJson{}
		h := NewTestHarness(server, emptyDb)
		req := GetRequest(t, RequestArgs{
			Path:   "/ping",
			Method: "GET",
		})
		testThatShouldFail := testing.T{}
		h.AssertResponseOkWithJsonList(&testThatShouldFail, req)
		testutil.AssertMsg(t, testThatShouldFail.Failed(), "Test didn't fail with bad response")
	})
}

type badError struct{}

func (s badError) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	response.WriteHeader(http.StatusUnauthorized)
	if _, err := response.Write([]byte(`{
		"there_should": "be stuff here"
	}`)); err != nil {
		panic(err)
	}
}

type goodError struct{}

func (s goodError) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	response.WriteHeader(http.StatusUnauthorized)
	if _, err := response.Write([]byte(`{ "error": {
		"message": "this is an error message",
		"code": "ERR_WITH_A_CODE",
		"fields": [
			{
				"code": "ERR_FIELD_CODE",
				"message": "field error",
				"field": "foo-field"
			}
		]
	}}`)); err != nil {
		panic(err)
	}
}

func TestTestHarness_AssertResponseNotOk(t *testing.T) {
	t.Run("accept a good error response", func(t *testing.T) {
		server := goodError{}
		h := NewTestHarness(server, emptyDb)
		req := GetRequest(t, RequestArgs{
			Path:   "/ping",
			Method: "GET",
		})
		_, _ = h.AssertResponseNotOk(t, req)
	})

	t.Run("fail with a 200 code", func(t *testing.T) {
		server := goodObject{}
		h := NewTestHarness(server, emptyDb)
		req := GetRequest(t, RequestArgs{
			Path:   "/ping",
			Method: "GET",
		})
		testThatShouldFail := testing.T{}
		res, _ := h.AssertResponseNotOk(&testThatShouldFail, req)
		testutil.AssertMsg(t, res.Code == 200, "Code wasn't 200")
		testutil.AssertMsg(t, testThatShouldFail.Failed(), "test didn't fail with 200 code")
	})

	t.Run("fail with a error code that doesn't have a correct error response", func(t *testing.T) {
		server := badError{}
		h := NewTestHarness(server, emptyDb)
		req := GetRequest(t, RequestArgs{
			Path:   "/ping",
			Method: "GET",
		})
		testThatShouldFail := testing.T{}
		res, _ := h.AssertResponseNotOk(&testThatShouldFail, req)
		testutil.AssertMsg(t, res.Code != 200, "Code was 200")
		testutil.AssertMsg(t, testThatShouldFail.Failed(), "test didn't fail with bad error message")

	})
}
