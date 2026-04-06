package schema

import (
	"strings"
)

// Các giá trị memory_tier thống nhất cho vector metadata và DataPoint.Metadata.
const (
	MemoryTierCore    = "core"
	MemoryTierGeneral = "general"
	MemoryTierData    = "data"
	MemoryTierStorage = "storage"
)

// FourTierEngineConfig bật/tắt chế độ tìm kiếm bốn tầng ở cấp engine (mặc định tắt).
type FourTierEngineConfig struct {
	Enabled bool `json:"enabled"`
}

// FourTierSearchOptions ghi đè theo từng request Search.
type FourTierSearchOptions struct {
	// Enabled nil → dùng cấu hình engine; true/false ghi đè.
	Enabled *bool `json:"enabled,omitempty"`

	// SearchCore / SearchGeneral / SearchData nil → coi như bật khi chế độ 4 tầng đang bật.
	SearchCore    *bool `json:"search_core,omitempty"`
	SearchGeneral *bool `json:"search_general,omitempty"`
	SearchData    *bool `json:"search_data,omitempty"`

	// IncludeStorageTier: tìm tầng storage song song với các tầng khác (khối dữ liệu lớn).
	IncludeStorageTier bool `json:"include_storage_tier,omitempty"`

	// AutoStorageIfWeak: sau khi gộp core/general/data, nếu điểm vector tối đa vẫn thấp thì mới quét storage.
	AutoStorageIfWeak bool `json:"auto_storage_if_weak,omitempty"`

	// WeakScoreThreshold ngưỡng so với điểm similarity (cosine) sau khi nhân trọng số tầng; mặc định 0.35.
	WeakScoreThreshold float64 `json:"weak_score_threshold,omitempty"`
}

// FourTierSearchStats thống kê từng tầng sau một lần Search (debug/observability).
type FourTierSearchStats struct {
	CoreHitCount     int  `json:"core_hit_count"`
	GeneralHitCount  int  `json:"general_hit_count"`
	DataHitCount     int  `json:"data_hit_count"`
	StorageHitCount  int  `json:"storage_hit_count"`
	StorageSearched  bool `json:"storage_searched"`
	StorageLazyRun   bool `json:"storage_lazy_run"` // bật bởi AutoStorageIfWeak
	LegacyHitCount   int  `json:"legacy_hit_count"`
}

// NormalizeMemoryTier chuẩn hóa chuỗi tier; không hợp lệ → general.
func NormalizeMemoryTier(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case MemoryTierCore:
		return MemoryTierCore
	case MemoryTierGeneral:
		return MemoryTierGeneral
	case MemoryTierData:
		return MemoryTierData
	case MemoryTierStorage:
		return MemoryTierStorage
	default:
		return MemoryTierGeneral
	}
}

// MemoryTierFromDataPoint đọc memory_tier từ DataPoint; thiếu → general.
func MemoryTierFromDataPoint(dp *DataPoint) string {
	if dp == nil || dp.Metadata == nil {
		return MemoryTierGeneral
	}
	v, ok := dp.Metadata["memory_tier"].(string)
	if !ok || strings.TrimSpace(v) == "" {
		return MemoryTierGeneral
	}
	return NormalizeMemoryTier(v)
}

// MemoryTierFromVectorMetadata đọc tier từ payload vector; thiếu → rỗng (legacy, xử lý như general khi gộp).
func MemoryTierFromVectorMetadata(m map[string]interface{}) string {
	if m == nil {
		return ""
	}
	v, ok := m["memory_tier"].(string)
	if !ok || strings.TrimSpace(v) == "" {
		return ""
	}
	return NormalizeMemoryTier(v)
}

// EffectiveMemoryTier cho một vector hit: metadata rỗng → general.
func EffectiveMemoryTierFromVectorMetadata(m map[string]interface{}) string {
	t := MemoryTierFromVectorMetadata(m)
	if t == "" {
		return MemoryTierGeneral
	}
	return t
}
