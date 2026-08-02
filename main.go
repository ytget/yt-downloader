package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	"github.com/ytget/yt-downloader/internal/applog"
)

// Version is set during build via -ldflags "-X main.version=X.Y.Z"
var version = "dev"

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

func main() {
	// Early console logger until settings load configures file output.
	_ = applog.Init(applog.Config{Level: applog.DefaultLevel, FileEnabled: false})
	applog.Info("starting", "version", version)
	fmt.Printf("YT Downloader v%s starting...\n", version)

	app := NewApp(version)

	err := wails.Run(&options.App{
		Title:     "YT Downloader",
		Width:     920,
		Height:    700,
		MinWidth:  720,
		MinHeight: 520,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 18, G: 18, B: 22, A: 255},
		OnStartup:        app.startup,
		OnBeforeClose:    app.beforeClose,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			About: &mac.AboutInfo{
				Title:   "YT Downloader",
				Message: "Download YouTube videos and playlists",
				Icon:    icon,
			},
		},
	})
	if err != nil {
		applog.Error("wails exited", "err", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
