package redis

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedisGraphStore_NoOpMethods(t *testing.T) {
	s := &RedisGraphStore{}

	err := s.DeleteGraphBySessionID(context.Background(), "s1")
	require.NoError(t, err)

	cc, err := s.GetConnectedComponents(context.Background())
	require.NoError(t, err)
	require.Len(t, cc, 0)

	nodes, err := s.FindNodesByProperty(context.Background(), "k", "v")
	require.NoError(t, err)
	require.Len(t, nodes, 0)
}

