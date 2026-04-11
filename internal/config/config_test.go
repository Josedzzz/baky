package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfig(t *testing.T) {
	// Create a temporary directory for the test config
	tmpDir, err := os.MkdirTemp("", "baky-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Override configPath
	oldConfigPath := configPath
	configPath = filepath.Join(tmpDir, "config.json")
	defer func() { configPath = oldConfigPath }()

	// Test SaveNasPath and GetNasPath
	testNas := "/tmp/nas"
	if err := SaveNasPath(testNas); err != nil {
		t.Errorf("SaveNasPath failed: %v", err)
	}

	nas, err := GetNasPath()
	if err != nil {
		t.Errorf("GetNasPath failed: %v", err)
	}
	if nas != testNas {
		t.Errorf("Expected NAS path %s, got %s", testNas, nas)
	}

	// Test AddPath and GetPaths
	testPath := "/home/user/docs"
	if err := AddPath(testPath); err != nil {
		t.Errorf("AddPath failed: %v", err)
	}

	paths, err := GetPaths()
	if err != nil {
		t.Errorf("GetPaths failed: %v", err)
	}
	if len(paths) != 1 {
		t.Errorf("Expected 1 path, got %d", len(paths))
	}
	if paths[0].Path != testPath {
		t.Errorf("Expected path %s, got %s", testPath, paths[0].Path)
	}

	// Test LogBackup
	if err := LogBackup(testPath, true, "Success"); err != nil {
		t.Errorf("LogBackup failed: %v", err)
	}

	hist, err := GetHistory()
	if err != nil {
		t.Errorf("GetHistory failed: %v", err)
	}
	if len(hist) != 1 {
		t.Errorf("Expected 1 history event, got %d", len(hist))
	}
	if hist[0].Path != testPath || hist[0].Result != "success" {
		t.Errorf("History entry mismatch")
	}
}
