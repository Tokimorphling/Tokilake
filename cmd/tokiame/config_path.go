package main

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultTokiameConfigDir  = ".tokilake"
	defaultTokiameConfigFile = "tokiame.json"
)

func defaultConfigPathFromHome(homeDir string) string {
	homeDir = strings.TrimSpace(homeDir)
	if homeDir == "" {
		return ""
	}
	return filepath.Join(homeDir, defaultTokiameConfigDir, defaultTokiameConfigFile)
}

func resolveConfigPath(explicitPath string) string {
	explicitPath = strings.TrimSpace(explicitPath)
	if explicitPath != "" {
		return explicitPath
	}

	envPath := strings.TrimSpace(os.Getenv("TOKIAME_CONFIG"))
	if envPath != "" {
		return envPath
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	defaultPath := defaultConfigPathFromHome(homeDir)
	if defaultPath == "" {
		return ""
	}
	if _, err = os.Stat(defaultPath); err == nil {
		return defaultPath
	}
	return ""
}
