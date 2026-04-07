// Package sessionid chuẩn hóa session cho CLI, HTTP view và engine.
//
// Khái niệm:
//   - "default": tên session có tên trong DB (session_id = "default"), là ngữ cảnh mặc định khi không chỉ định.
//     Khác với “global” — không phải cùng một thứ.
//   - Global / shared: bản ghi có session_id rỗng (NULL/""), được search của mọi session có tên gộp thêm
//     (IncludeGlobalSession / dataPointVisibleForSearch).
//
// Từ khóa người dùng (không phân biệt hoa thường) cho global khi add: global, shared, _
package sessionid

import (
	"fmt"
	"strings"
)

// DefaultName is the normal named session used when nothing is configured.
const DefaultName = "default"

var globalKeywords = map[string]struct{}{
	"global": {},
	"shared": {},
	"_":      {},
}

// IsGlobalKeyword reports whether s is a user-facing alias for unscoped (empty session_id) storage.
func IsGlobalKeyword(s string) bool {
	_, ok := globalKeywords[strings.ToLower(strings.TrimSpace(s))]
	return ok
}

// ForDataPointAdd interprets user/CLI/API input for StoreDataPoint.session_id.
// If useGlobal is true, caller should use engine.WithGlobalSession() and ignore sessionID.
// Otherwise sessionID is the value to pass to WithSessionID (never empty — defaults to DefaultName).
func ForDataPointAdd(raw string) (sessionID string, useGlobal bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return DefaultName, false
	}
	if IsGlobalKeyword(s) {
		return "", true
	}
	return s, false
}

// ForEngineContext is the session key for Search, Think, Request chat history, and related scoping.
// Global keywords are not stored as a literal session name: they map to DefaultName so history/search
// stay on a normal namespace while global memories (empty session_id) are still merged into retrieval.
func ForEngineContext(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return DefaultName
	}
	if IsGlobalKeyword(s) {
		return DefaultName
	}
	return s
}

// ListFilter parses ?session= for listing datapoints/vectors: only unscoped rows vs named session.
// If raw is empty, unscopedOnly is false and sessionID is empty (no session filter — all rows).
func ListFilter(raw string) (sessionID string, unscopedOnly bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", false
	}
	if IsGlobalKeyword(s) {
		return "", true
	}
	return s, false
}

// ForBulkDeleteAll parses CLI/API input for wiping all memory tied to a session.
// Global keywords → delete only unscoped rows (empty session_id). Otherwise returns the exact session name.
func ForBulkDeleteAll(raw string) (unscoped bool, namedSession string, err error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return false, "", fmt.Errorf("session is required")
	}
	if IsGlobalKeyword(s) {
		return true, "", nil
	}
	return false, s, nil
}
