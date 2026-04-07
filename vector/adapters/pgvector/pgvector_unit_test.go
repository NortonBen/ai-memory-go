package pgvector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeleteBatchEmbeddings_EmptyIDs(t *testing.T) {
	s := &PgVectorStore{}
	err := s.DeleteBatchEmbeddings(context.Background(), nil)
	require.NoError(t, err)
}

