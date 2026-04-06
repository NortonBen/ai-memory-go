package schema

import (
	"fmt"
	"sort"
	"strings"
)

// Nhãn gợi ý (built-in). Người dùng có thể thêm nhãn tùy ý (vd. tên truyện).
const (
	LabelRule    = "rule"
	LabelPolicy  = "policy"
	LabelInfo    = "info"
	LabelStory   = "story"
	LabelDoc     = "doc"
	LabelFAQ     = "faq"
	LabelSnippet = "snippet"
)

// CoreTierBuiltinLabels: nếu bản ghi có ít nhất một nhãn này và không chỉ định tier — gán memory_tier = core.
var CoreTierBuiltinLabels = map[string]struct{}{
	LabelRule:   {},
	LabelPolicy: {},
}

// MetadataKeyMemoryLabels key lưu slice nhãn trên DataPoint (JSON array string).
const MetadataKeyMemoryLabels = "memory_labels"

// MetadataKeyPrimaryLabel nhãn chính (tìm kiếm / lọc nhanh, thường là nhãn đầu tiên).
const MetadataKeyPrimaryLabel = "primary_label"

// MetadataKeyLabelsJoined chuỗi nối bằng | trên payload embedding (đồng bộ phân loại; lọc tùy chọn qua VectorFilterKeyLabelsAny).
const MetadataKeyLabelsJoined = "labels_joined"

// NormalizeLabel chuẩn hóa một nhãn: trim, lower.
func NormalizeLabel(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// NormalizeLabels loại trùng, giữ thứ tự xuất hiện đầu tiên.
func NormalizeLabels(in []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, s := range in {
		n := NormalizeLabel(s)
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out
}

// DefaultMemoryTierFromLabels khi chưa set tier explicit: có rule/policy → core, ngược lại "" (caller dùng general).
func DefaultMemoryTierFromLabels(labels []string) string {
	for _, L := range NormalizeLabels(labels) {
		if _, ok := CoreTierBuiltinLabels[L]; ok {
			return MemoryTierCore
		}
	}
	return ""
}

// LabelsToMetadataSlice để lưu JSON array trong metadata.
func LabelsToMetadataSlice(labels []string) []interface{} {
	n := NormalizeLabels(labels)
	out := make([]interface{}, len(n))
	for i, s := range n {
		out[i] = s
	}
	return out
}

// LabelsFromMetadata đọc memory_labels từ DataPoint ([]string, []interface{}, hoặc chuỗi phân tách).
func LabelsFromMetadata(meta map[string]interface{}) []string {
	if meta == nil {
		return nil
	}
	raw, ok := meta[MetadataKeyMemoryLabels]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return NormalizeLabels(v)
	case []interface{}:
		var ss []string
		for _, x := range v {
			if s, ok := x.(string); ok {
				ss = append(ss, s)
			}
		}
		return NormalizeLabels(ss)
	case string:
		parts := strings.FieldsFunc(v, func(r rune) bool {
			return r == ',' || r == '|' || r == ';'
		})
		return NormalizeLabels(parts)
	default:
		return nil
	}
}

// JoinLabelsForVector nối nhãn bằng | trên metadata vector (phân loại / debug; không dùng trong SearchQuery mặc định).
func JoinLabelsForVector(labels []string) string {
	n := NormalizeLabels(labels)
	if len(n) == 0 {
		return ""
	}
	sort.Strings(n)
	return strings.Join(n, "|")
}

// VectorFilterKeyLabelsAny là key đặc biệt cho SimilaritySearchWithFilter: payload vector phải khớp
// ít nhất một nhãn (OR), so với primary_label / labels_joined / memory_labels.
const VectorFilterKeyLabelsAny = "__labels_any__"

// VectorSearchTierFilter map filter chỉ theo memory_tier cho SimilaritySearchWithFilter (tìm kiếm theo nội dung/text, không lọc nhãn).
func VectorSearchTierFilter(memoryTier string) map[string]interface{} {
	t := strings.TrimSpace(memoryTier)
	if t == "" {
		return nil
	}
	return map[string]interface{}{"memory_tier": t}
}

// LabelsFromVectorFilterLabelsAnyValue chuẩn hóa giá trị filter nhãn ([]string, []interface{}, hoặc chuỗi phân tách).
func LabelsFromVectorFilterLabelsAnyValue(v interface{}) []string {
	switch t := v.(type) {
	case []string:
		return NormalizeLabels(t)
	case []interface{}:
		var ss []string
		for _, x := range t {
			if s, ok := x.(string); ok {
				ss = append(ss, s)
			}
		}
		return NormalizeLabels(ss)
	case string:
		parts := strings.FieldsFunc(t, func(r rune) bool {
			return r == ',' || r == '|' || r == ';'
		})
		return NormalizeLabels(parts)
	default:
		return nil
	}
}

// VectorSearchHasLabelFilter true nếu filters có ràng buộc nhãn hợp lệ.
func VectorSearchHasLabelFilter(filters map[string]interface{}) bool {
	if len(filters) == 0 {
		return false
	}
	v, ok := filters[VectorFilterKeyLabelsAny]
	if !ok || v == nil {
		return false
	}
	return len(LabelsFromVectorFilterLabelsAnyValue(v)) > 0
}

// FiltersForVectorSearchEngine bản map filter bỏ VectorFilterKeyLabelsAny — dùng khi backend chỉ index payload đơn;
// phần nhãn xử lý post-filter bằng MetadataMatchesVectorSearchFilters.
func FiltersForVectorSearchEngine(filters map[string]interface{}) map[string]interface{} {
	if len(filters) == 0 {
		return nil
	}
	out := make(map[string]interface{})
	for k, v := range filters {
		if k == VectorFilterKeyLabelsAny {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// EmbeddingMetadataHasAnyLabel true nếu embedding metadata có ít nhất một nhãn trong want (đã normalize).
func EmbeddingMetadataHasAnyLabel(meta map[string]interface{}, want []string) bool {
	if len(want) == 0 {
		return true
	}
	normWant := NormalizeLabels(want)
	if len(normWant) == 0 {
		return true
	}
	if meta == nil {
		return false
	}
	prim := ""
	if p, ok := meta[MetadataKeyPrimaryLabel].(string); ok {
		prim = NormalizeLabel(p)
	}
	joined := make(map[string]struct{})
	if j, ok := meta[MetadataKeyLabelsJoined].(string); ok && strings.TrimSpace(j) != "" {
		for _, p := range strings.Split(j, "|") {
			n := NormalizeLabel(p)
			if n != "" {
				joined[n] = struct{}{}
			}
		}
	}
	for _, w := range normWant {
		if w == prim {
			return true
		}
		if _, ok := joined[w]; ok {
			return true
		}
	}
	for _, L := range LabelsFromMetadata(meta) {
		nl := NormalizeLabel(L)
		for _, w := range normWant {
			if nl == w {
				return true
			}
		}
	}
	return false
}

// MetadataMatchesVectorSearchFilters AND các cặp key/value thường (so khớp string không phân biệt hoa thường);
// với VectorFilterKeyLabelsAny thì dùng EmbeddingMetadataHasAnyLabel (OR).
func MetadataMatchesVectorSearchFilters(metadata map[string]interface{}, filters map[string]interface{}) bool {
	if len(filters) == 0 {
		return true
	}
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	for k, v := range filters {
		if k == VectorFilterKeyLabelsAny {
			labs := LabelsFromVectorFilterLabelsAnyValue(v)
			if len(labs) == 0 {
				continue
			}
			if !EmbeddingMetadataHasAnyLabel(metadata, labs) {
				return false
			}
			continue
		}
		mv, ok := metadata[k]
		if !ok || !strings.EqualFold(strings.TrimSpace(fmt.Sprintf("%v", mv)), strings.TrimSpace(fmt.Sprintf("%v", v))) {
			return false
		}
	}
	return true
}

// DataPointHasAnyLabel true nếu dp có ít nhất một nhãn trong want (OR).
func DataPointHasAnyLabel(dp *DataPoint, want []string) bool {
	if len(want) == 0 {
		return true
	}
	have := LabelSetFromDataPoint(dp)
	for _, w := range NormalizeLabels(want) {
		if _, ok := have[w]; ok {
			return true
		}
	}
	return false
}

// LabelSetFromDataPoint tập nhãn (kể cả primary_label nếu không nằm trong list).
func LabelSetFromDataPoint(dp *DataPoint) map[string]struct{} {
	out := make(map[string]struct{})
	if dp == nil || dp.Metadata == nil {
		return out
	}
	for _, L := range LabelsFromMetadata(dp.Metadata) {
		out[L] = struct{}{}
	}
	if p, ok := dp.Metadata[MetadataKeyPrimaryLabel].(string); ok {
		n := NormalizeLabel(p)
		if n != "" {
			out[n] = struct{}{}
		}
	}
	return out
}

// LabelSetsEqual hai tập nhãn đã normalize có cùng phần tử.
func LabelSetsEqual(a, b []string) bool {
	na := NormalizeLabels(a)
	nb := NormalizeLabels(b)
	if len(na) != len(nb) {
		return false
	}
	sa := append([]string(nil), na...)
	sb := append([]string(nil), nb...)
	sort.Strings(sa)
	sort.Strings(sb)
	for i := range sa {
		if sa[i] != sb[i] {
			return false
		}
	}
	return true
}
