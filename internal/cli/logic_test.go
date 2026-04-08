package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseLabelsCSV(t *testing.T) {
	require.Nil(t, parseLabelsCSV(""))
	require.Nil(t, parseLabelsCSV("   "))
	require.Equal(t, []string{"a", "b", "c"}, parseLabelsCSV("a, b, ,c"))
}

func TestResolveDeleteSessionArg(t *testing.T) {
	prev := session
	t.Cleanup(func() { session = prev })
	session = "from-flag"

	require.Equal(t, "x", resolveDeleteSessionArg(" x ", false))
	require.Equal(t, "from-flag", resolveDeleteSessionArg("", true))
	require.Equal(t, "", resolveDeleteSessionArg("", false))
}

func TestCommandArgsValidators(t *testing.T) {
	require.NoError(t, searchCmd.Args(searchCmd, []string{"q"}))
	require.Error(t, searchCmd.Args(searchCmd, []string{}))
	require.Error(t, searchCmd.Args(searchCmd, []string{"a", "b"}))

	require.NoError(t, thinkCmd.Args(thinkCmd, []string{"q"}))
	require.Error(t, thinkCmd.Args(thinkCmd, []string{}))

	require.NoError(t, requestCmd.Args(requestCmd, []string{"hello"}))
	require.Error(t, requestCmd.Args(requestCmd, []string{}))
}

func TestGetSessionRaw_FromSessionFileFallback(t *testing.T) {
	prev := session
	t.Cleanup(func() { session = prev })
	session = "default"

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, ".ai-memory"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, ".ai-memory", "session.txt"), []byte("from-file\n"), 0o644))

	require.Equal(t, "from-file", getSessionRaw())
}

