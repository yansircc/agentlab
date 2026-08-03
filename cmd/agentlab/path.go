package main

import (
	"os"
	"path/filepath"
)

func defaultRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".agentlab"
	}
	return filepath.Join(home, ".agentlab")
}
