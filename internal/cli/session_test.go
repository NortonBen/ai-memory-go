package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NortonBen/ai-memory-go/internal/sessionid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestSessionCmd_SwitchWritesSessionFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// ensure it doesn't depend on flags
	sessionCmd.SetArgs([]string{})
	sessionCmd.SetOut(os.Stdout)
	sessionCmd.SetErr(os.Stderr)

	sessionCmd.Run(sessionCmd, []string{"switch", "proj-1"})

	b, err := os.ReadFile(filepath.Join(tmp, ".ai-memory", "session.txt"))
	require.NoError(t, err)
	require.Equal(t, "proj-1", string(b))
}

func TestSessionCmd_SwitchGlobalKeyword(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	sessionCmd.Run(sessionCmd, []string{"switch", "global"})

	b, err := os.ReadFile(filepath.Join(tmp, ".ai-memory", "session.txt"))
	require.NoError(t, err)
	require.Equal(t, "global", string(b))
	require.True(t, sessionid.IsGlobalKeyword(string(b)))
}

func TestSessionCmd_SwitchMissingArgDoesNotPanic(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	require.NotPanics(t, func() {
		sessionCmd.Run(sessionCmd, []string{"switch"})
	})
	_, err := os.Stat(filepath.Join(tmp, ".ai-memory", "session.txt"))
	require.Error(t, err)
}

func TestSessionCmd_UnknownSubcommandDoesNotPanic(t *testing.T) {
	require.NotPanics(t, func() {
		sessionCmd.Run(sessionCmd, []string{"wat"})
	})
}

func TestSessionCmd_NoArgsCallsHelp(t *testing.T) {
	// Use a disposable command wrapper to avoid polluting global flags.
	cmd := &cobra.Command{Use: "session"}
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)

	require.NotPanics(t, func() {
		// mimic sessionCmd no-arg behavior
		sessionCmd.Run(cmd, nil)
	})
}

