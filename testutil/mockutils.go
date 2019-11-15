package testutil

import (
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type mockHttpPoster struct {
	sync.Mutex
	sentPostRequests int
	sentBodies       [][]byte
}

func GetMockHttpPoster() *mockHttpPoster {
	return &mockHttpPoster{}
}

func (m *mockHttpPoster) Post(url, contentType string, reader io.Reader) (*http.Response, error) {
	m.Lock()
	m.sentPostRequests += 1

	log.WithField("url", url).Info("Mock HTTP poster POSTing")

	body, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	m.sentBodies = append(m.sentBodies, body)
	m.Unlock()

	return &http.Response{
		StatusCode: 200,
	}, nil
}

func (m *mockHttpPoster) GetSentPostRequests() int {
	return m.sentPostRequests
}

func (m *mockHttpPoster) GetSentPostRequest(index int) []byte {
	return m.sentBodies[index]
}

func MockStringOfLength(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")

	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// GetPortOrFail returns a unused port
func GetPortOrFail(t *testing.T) int {
	const minPortNumber = 1024
	const maxPortNumber = 40000
	rand.Seed(time.Now().UnixNano())
	port := rand.Intn(maxPortNumber)
	// port is reserved, try again
	if port < minPortNumber {
		return GetPortOrFail(t)
	}

	listener, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	// port is busy, try again
	if err != nil {
		return GetPortOrFail(t)
	}
	require.NoError(t, listener.Close())
	return port
}
