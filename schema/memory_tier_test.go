package schema

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeMemoryTier(t *testing.T) {
	require.Equal(t, MemoryTierCore, NormalizeMemoryTier("core"))
	require.Equal(t, MemoryTierGeneral, NormalizeMemoryTier(" GENERAL "))
	require.Equal(t, MemoryTierData, NormalizeMemoryTier("data"))
	require.Equal(t, MemoryTierStorage, NormalizeMemoryTier("storage"))
	require.Equal(t, MemoryTierGeneral, NormalizeMemoryTier("unknown"))
}

func TestMemoryTierFromDataPoint(t *testing.T) {
	require.Equal(t, MemoryTierGeneral, MemoryTierFromDataPoint(nil))
	require.Equal(t, MemoryTierGeneral, MemoryTierFromDataPoint(&DataPoint{}))
	require.Equal(t, MemoryTierGeneral, MemoryTierFromDataPoint(&DataPoint{Metadata: map[string]interface{}{"memory_tier": ""}}))
	require.Equal(t, MemoryTierData, MemoryTierFromDataPoint(&DataPoint{Metadata: map[string]interface{}{"memory_tier": "data"}}))
}

func TestMemoryTierFromVectorMetadata(t *testing.T) {
	require.Equal(t, "", MemoryTierFromVectorMetadata(nil))
	require.Equal(t, "", MemoryTierFromVectorMetadata(map[string]interface{}{}))
	require.Equal(t, "", MemoryTierFromVectorMetadata(map[string]interface{}{"memory_tier": 1}))
	require.Equal(t, MemoryTierCore, MemoryTierFromVectorMetadata(map[string]interface{}{"memory_tier": "core"}))
}

func TestEffectiveMemoryTierFromVectorMetadata(t *testing.T) {
	require.Equal(t, MemoryTierGeneral, EffectiveMemoryTierFromVectorMetadata(nil))
	require.Equal(t, MemoryTierGeneral, EffectiveMemoryTierFromVectorMetadata(map[string]interface{}{"x": "y"}))
	require.Equal(t, MemoryTierStorage, EffectiveMemoryTierFromVectorMetadata(map[string]interface{}{"memory_tier": "storage"}))
}

