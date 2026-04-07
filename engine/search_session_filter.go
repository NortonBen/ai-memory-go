package engine

import (
	"strings"

	"github.com/NortonBen/ai-memory-go/schema"
)

func effectiveSearchSessionID(sessionID string) string {
	s := strings.TrimSpace(sessionID)
	if s == "" {
		return "default"
	}
	return s
}

// dataPointVisibleForSearch allows the active session’s rows plus unscoped (empty session_id) “global” memories.
// Named sessions such as "default" and "test" never match each other.
func dataPointVisibleForSearch(dp *schema.DataPoint, querySessionID string) bool {
	if dp == nil {
		return false
	}
	if dp.SessionID == "" {
		return true
	}
	return dp.SessionID == effectiveSearchSessionID(querySessionID)
}
