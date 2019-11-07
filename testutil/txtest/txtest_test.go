package txtest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMockAddress(t *testing.T) {
	t.Run("can create random addresses", func(t *testing.T) {
		var set [1000]string
		for i := 0; i < 1000; i++ {
			newAddress := MockAddress().EncodeAddress()
			require.NotContains(t, set, newAddress)

			set[i] = newAddress
		}
	})
}
