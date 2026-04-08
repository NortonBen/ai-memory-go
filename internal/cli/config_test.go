package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestCreateDefaultConfig_WritesFileUnderHome(t *testing.T) {
	viper.Reset()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	configPath := filepath.Join(tmp, ".ai-memory.yaml")
	_, err := os.Stat(configPath)
	require.Error(t, err)

	createDefaultConfig()

	_, err = os.Stat(configPath)
	require.NoError(t, err)
}

func TestCreateDefaultConfig_IdempotentWhenExists(t *testing.T) {
	viper.Reset()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	configPath := filepath.Join(tmp, ".ai-memory.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("db:\n  datadir: x\n"), 0o600))

	createDefaultConfig()
	// file should still exist and be readable
	b, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.Contains(t, string(b), "datadir")
}

func TestInitConfig_UsesCfgFileWhenSet(t *testing.T) {
	viper.Reset()
	tmp := t.TempDir()
	cfg := filepath.Join(tmp, "custom.yaml")
	require.NoError(t, os.WriteFile(cfg, []byte("db:\n  datadir: mydir\n"), 0o600))

	cfgFile = cfg
	verbose = true
	initConfig()

	require.Equal(t, "mydir", viper.GetString("db.datadir"))
}

