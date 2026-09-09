package main

import (
	"embed"
	"log/slog"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"karina/internal/core"
	"karina/internal/logging"
	"karina/internal/platform"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

const appName = "Karina"
const appVersion = "0.2.0"

func main() {
	log := logging.New(slog.LevelInfo)

	svc := core.New(log)
	if err := svc.Open(core.Options{Autostart: platform.SetStartWithSystem}); err != nil {
		log.Error("failed to open service", "error", err.Error())
		return
	}

	app := &App{svc: svc, log: log, icon: appIcon}

	err := wails.Run(&options.App{
		Title:     appName,
		Width:     1180,
		Height:    780,
		MinWidth:  860,
		MinHeight: 600,
		Frameless: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		Windows: &windows.Options{
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			BackdropType:         windows.Acrylic,
		},
		HideWindowOnClose: platform.TrayAvailable,
		OnStartup:         app.startup,
		OnShutdown:        app.shutdown,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Error("failed to launch application", "error", err.Error())
	}
}
