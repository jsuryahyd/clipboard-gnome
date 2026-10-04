package compat

import (
	"fmt"
	"os"
	"path/filepath"
)

// InitPlatformCompat applies system compatibility workarounds for WebKitGTK and Wayland.
// This keeps the application resilient against graphics driver issues, WebKit regressions,
// and corrupted Fontconfig cache files on Linux desktops.
func InitPlatformCompat() {
	CleanFontconfigCacheIfCorrupted()
	SetupWebKitEnvironment()
	EnsureWaylandDisplay()
}

// CleanFontconfigCacheIfCorrupted detects and removes circular or problematic symlinks
// in ~/.cache/fontconfig. Corrupted symlinks cause WebKitWebProcess to peg CPU at 100%
// and hang indefinitely on Zorin OS / Ubuntu.
func CleanFontconfigCacheIfCorrupted() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	cacheDir := filepath.Join(home, ".cache", "fontconfig")
	CleanFontconfigDir(cacheDir)
}

// CleanFontconfigDir removes symlink entries from the specified fontconfig cache directory.
func CleanFontconfigDir(cacheDir string) {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return
	}
	hasSymlinks := false
	for _, entry := range entries {
		info, err := entry.Info()
		if err == nil && (info.Mode()&os.ModeSymlink != 0) {
			hasSymlinks = true
			break
		}
	}
	if hasSymlinks {
		for _, entry := range entries {
			info, err := entry.Info()
			if err == nil && (info.Mode()&os.ModeSymlink != 0) {
				_ = os.Remove(filepath.Join(cacheDir, entry.Name()))
			}
		}
	}
}

// SetupWebKitEnvironment configures environment variables to avoid WebKitGTK 2.52+
// Wayland rendering hangs and DMA-BUF driver conflicts.
func SetupWebKitEnvironment() {
	if os.Getenv("WEBKIT_DISABLE_DMABUF_RENDERER") == "" {
		_ = os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")
	}
	if os.Getenv("WEBKIT_DISABLE_COMPOSITING_MODE") == "" {
		_ = os.Setenv("WEBKIT_DISABLE_COMPOSITING_MODE", "1")
	}
}

// EnsureWaylandDisplay auto-detects active Wayland session sockets when the application
// is launched from non-login shells or subagent environments.
func EnsureWaylandDisplay() {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
		if runtimeDir == "" {
			runtimeDir = fmt.Sprintf("/run/user/%d", os.Getuid())
		}
		if _, err := os.Stat(filepath.Join(runtimeDir, "wayland-0")); err == nil {
			_ = os.Setenv("WAYLAND_DISPLAY", "wayland-0")
			if os.Getenv("GDK_BACKEND") == "" {
				_ = os.Setenv("GDK_BACKEND", "wayland")
			}
		}
	} else if os.Getenv("GDK_BACKEND") == "" {
		_ = os.Setenv("GDK_BACKEND", "wayland")
	}
}

// GetDesktopExecCmd wraps a binary path with the required WebKit compatibility flags
// for desktop launcher and autostart files.
func GetDesktopExecCmd(execPath string, hidden bool) string {
	cmd := fmt.Sprintf("env WEBKIT_DISABLE_DMABUF_RENDERER=1 WEBKIT_DISABLE_COMPOSITING_MODE=1 %s", execPath)
	if hidden {
		cmd += " --hidden"
	}
	return cmd
}
