package redis

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFloat32BytesRoundTrip(t *testing.T) {
	in := []float32{0, 1.25, -3.5, 42}
	b := float32ToBytes(in)
	out := bytesToFloat32(b)
	require.Equal(t, len(in), len(out))
	for i := range in {
		require.InDelta(t, in[i], out[i], 0.0001)
	}
}

func TestBytesToFloat32_InvalidLength(t *testing.T) {
	require.Nil(t, bytesToFloat32([]byte{1, 2, 3}))
}

