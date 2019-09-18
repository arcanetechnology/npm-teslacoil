package testutil

import (
	"fmt"
	"math/rand"
	"testing"
)

// GetTestEmail generates a random email for a given test
func GetTestEmail(t *testing.T) string {
	return fmt.Sprintf("%d-%s@example.com", rand.Int(), t.Name())
}
