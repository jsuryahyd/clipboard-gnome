package main

import (
	"context"
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed all:frontend
var assets embed.FS

//go:embed assets/appicon.png
var icon []byte

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
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0}, // Transparent background for rounded CSS container
		OnStartup:        app.startup,
		OnDomReady:       app.domReady,
		OnShutdown: func(ctx context.Context) {
			instanceMgr.Close()
			app.shutdown(ctx)
		},
		Linux: &linux.Options{
			Icon:                icon,
			WindowIsTranslucent: true,
			WebviewGpuPolicy:    linux.WebviewGpuPolicyAlways,
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		log.Fatalf("Error starting Clipboard-Gnome Wails application: %v", err)
	}
}
