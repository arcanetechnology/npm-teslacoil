package testutil

import (
	"os"
	"testing"
)

//SkipIfCI skips the given test if we're running on CI
func SkipIfCI(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping test on CI")
	}
}
