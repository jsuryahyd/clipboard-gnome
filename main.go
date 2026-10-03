package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed all:frontend
var assets embed.FS

//go:embed assets/appicon.png
var icon []byte

func init() {
	cleanFontconfigCacheIfCorrupted()

	// WebKitGTK 2.52+ on Wayland / Mesa can cause WebKitWebProcess hangs or blank
	// rendering with accelerated compositing / DMA-BUF. Disabling DMA-BUF renderer
	// and compositing mode ensures reliable rendering on Wayland.
	if os.Getenv("WEBKIT_DISABLE_DMABUF_RENDERER") == "" {
		_ = os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")
	}
	if os.Getenv("WEBKIT_DISABLE_COMPOSITING_MODE") == "" {
		_ = os.Setenv("WEBKIT_DISABLE_COMPOSITING_MODE", "1")
	}

	// Auto-detect Wayland environment if running from non-login or subshell environment
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

// cleanFontconfigCacheIfCorrupted detects and removes circular or problematic symlinks
// in ~/.cache/fontconfig, which causes WebKitWebProcess to peg CPU at 100% and hang
// on Zorin OS / Ubuntu.
func cleanFontconfigCacheIfCorrupted() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	cacheDir := filepath.Join(home, ".cache", "fontconfig")
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

func main() {
	app := NewApp()

	// Handle single-instance lock, version checks, command-line flags
	instanceMgr := InitSingleInstance(app, os.Args[1:])
	defer instanceMgr.Close()

	app.startHidden = instanceMgr.StartHidden

	err := wails.Run(&options.App{
		Title:             "Clipboard-Gnome",
		Width:             420,
		Height:            640,
		MinWidth:          360,
		MinHeight:         480,
		DisableResize:     false,
		Frameless:         true,
		AlwaysOnTop:       true,
		HideWindowOnClose: true,
		StartHidden:       instanceMgr.StartHidden,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 15, G: 23, B: 42, A: 255}, // Solid slate-900 background matching --bg-main
		OnStartup:        app.startup,
		OnDomReady:       app.domReady,
		OnShutdown: func(ctx context.Context) {
			instanceMgr.Close()
			app.shutdown(ctx)
		},
		Linux: &linux.Options{
			Icon:                icon,
			WindowIsTranslucent: false,
			WebviewGpuPolicy:    linux.WebviewGpuPolicyNever,
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		log.Fatalf("Error starting Clipboard-Gnome Wails application: %v", err)
	}
}
