package sqlite

import (
	"testing"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/stretchr/testify/require"
)

func TestNewSQLiteAdapter_NilConfig(t *testing.T) {
	adapter, err := NewSQLiteAdapter(nil)
	require.Error(t, err)
	require.Nil(t, adapter)
}

func TestSQLiteTableSelectionHelpers(t *testing.T) {
	require.False(t, isInputDataPoint(nil))
	require.Equal(t, "datapoints", tableForDataPoint(nil))

	dpInput := &schema.DataPoint{Metadata: map[string]interface{}{"is_input": true}}
	dpNormal := &schema.DataPoint{Metadata: map[string]interface{}{"is_input": false}}

	require.True(t, isInputDataPoint(dpInput))
	require.False(t, isInputDataPoint(dpNormal))
	require.Equal(t, "input_datapoints", tableForDataPoint(dpInput))
	require.Equal(t, "datapoints", tableForDataPoint(dpNormal))
}

