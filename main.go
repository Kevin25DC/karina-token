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

// trayIcon must be a real .ico (systray's Windows backend loads it via
// LoadImage/IMAGE_ICON, which rejects a plain .png despite the misleading
// "operation completed successfully" it reports on failure).
//
//go:embed build/windows/icon.ico
var trayIcon []byte

const appName = "Karina"
const appVersion = "0.5.1"

func main() {
	log := logging.New(slog.LevelInfo)

	svc := core.New(log)
	if err := svc.Open(core.Options{Autostart: platform.SetStartWithSystem, Notify: platform.Notify}); err != nil {
		log.Error("failed to open service", "error", err.Error())
		return
	}

	app := &App{svc: svc, log: log, icon: trayIcon}

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
