package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfigPathFromHome(t *testing.T) {
	assert.Equal(t, "/tmp/home/.tokilake/tokiame.json", defaultConfigPathFromHome("/tmp/home"))
	assert.Equal(t, "", defaultConfigPathFromHome(""))
}

func TestResolveConfigPathPrefersExplicitFlag(t *testing.T) {
	t.Setenv("TOKIAME_CONFIG", "/tmp/from-env.json")
	assert.Equal(t, "/tmp/from-flag.json", resolveConfigPath("/tmp/from-flag.json"))
}

func TestResolveConfigPathFallsBackToEnv(t *testing.T) {
	t.Setenv("TOKIAME_CONFIG", "/tmp/from-env.json")
	assert.Equal(t, "/tmp/from-env.json", resolveConfigPath(""))
}

func TestResolveConfigPathUsesDefaultHomeLocation(t *testing.T) {
	t.Setenv("TOKIAME_CONFIG", "")

	homeDir := t.TempDir()
	configPath := filepath.Join(homeDir, ".tokilake", "tokiame.json")
	err := os.MkdirAll(filepath.Dir(configPath), 0o755)
	assert.NoError(t, err)
	err = os.WriteFile(configPath, []byte("{}"), 0o644)
	assert.NoError(t, err)

	oldHome := os.Getenv("HOME")
	t.Cleanup(func() {
		if oldHome == "" {
			_ = os.Unsetenv("HOME")
			return
		}
		_ = os.Setenv("HOME", oldHome)
	})
	assert.NoError(t, os.Setenv("HOME", homeDir))

	assert.Equal(t, configPath, resolveConfigPath(""))
}
