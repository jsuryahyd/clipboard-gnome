package compat

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSetupWebKitEnvironment(t *testing.T) {
	_ = os.Unsetenv("WEBKIT_DISABLE_DMABUF_RENDERER")
	_ = os.Unsetenv("WEBKIT_DISABLE_COMPOSITING_MODE")

	SetupWebKitEnvironment()

	if os.Getenv("WEBKIT_DISABLE_DMABUF_RENDERER") != "1" {
		t.Errorf("Expected WEBKIT_DISABLE_DMABUF_RENDERER=1, got %q", os.Getenv("WEBKIT_DISABLE_DMABUF_RENDERER"))
	}
	if os.Getenv("WEBKIT_DISABLE_COMPOSITING_MODE") != "1" {
		t.Errorf("Expected WEBKIT_DISABLE_COMPOSITING_MODE=1, got %q", os.Getenv("WEBKIT_DISABLE_COMPOSITING_MODE"))
	}
}

func TestCleanFontconfigDir(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fontconfig_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create regular cache file
	regularFile := filepath.Join(tempDir, "regular.cache-9")
	if err := os.WriteFile(regularFile, []byte("cache_data"), 0644); err != nil {
		t.Fatalf("Failed to create regular file: %v", err)
	}

	// Create problematic symlink
	symlinkFile := filepath.Join(tempDir, "symlink.cache-10")
	if err := os.Symlink(regularFile, symlinkFile); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	CleanFontconfigDir(tempDir)

	// Regular file should still exist
	if _, err := os.Stat(regularFile); os.IsNotExist(err) {
		t.Errorf("Regular cache file should not have been removed")
	}

	// Symlink should be removed
	if _, err := os.Lstat(symlinkFile); !os.IsNotExist(err) {
		t.Errorf("Symlink cache file should have been removed")
	}
}

func TestGetDesktopExecCmd(t *testing.T) {
	execPath := "/usr/local/bin/clipboard-gnome"
	
	cmdNormal := GetDesktopExecCmd(execPath, false)
	expectedNormal := "env WEBKIT_DISABLE_DMABUF_RENDERER=1 WEBKIT_DISABLE_COMPOSITING_MODE=1 /usr/local/bin/clipboard-gnome"
	if cmdNormal != expectedNormal {
		t.Errorf("Expected %q, got %q", expectedNormal, cmdNormal)
	}

	cmdHidden := GetDesktopExecCmd(execPath, true)
	expectedHidden := "env WEBKIT_DISABLE_DMABUF_RENDERER=1 WEBKIT_DISABLE_COMPOSITING_MODE=1 /usr/local/bin/clipboard-gnome --hidden"
	if cmdHidden != expectedHidden {
		t.Errorf("Expected %q, got %q", expectedHidden, cmdHidden)
	}
}
