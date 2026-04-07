package cli

import (
	"testing"

	"github.com/NortonBen/ai-memory-go/internal/sessionid"
	"github.com/stretchr/testify/require"
)

func TestGetSessionRawAndID_FromFlag(t *testing.T) {
	prev := session
	t.Cleanup(func() { session = prev })

	session = "my-session"
	require.Equal(t, "my-session", getSessionRaw())
	require.Equal(t, "my-session", GetSessionRaw())
	require.Equal(t, "my-session", GetSessionID())
}

func TestGetSessionID_GlobalKeywordMapsToDefault(t *testing.T) {
	prev := session
	t.Cleanup(func() { session = prev })

	session = "global"
	require.Equal(t, sessionid.DefaultName, GetSessionID())
}

