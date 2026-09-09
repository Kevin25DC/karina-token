//go:build !windows

package platform

import "fmt"

// TrayActions are the callbacks wired from the system tray menu.
type TrayActions struct {
	OnShow    func()
	OnHide    func()
	OnRefresh func()
	OnWidget  func()
	OnQuit    func()
}

// TrayAvailable reports whether a system tray is supported on this build.
const TrayAvailable = false

// StartTray is not yet implemented on this platform (Windows only for now).
func StartTray(_ []byte, _ TrayActions) error {
	return fmt.Errorf("bandeja del sistema no disponible aún en esta plataforma")
}

// StopTray is a no-op on unsupported platforms.
func StopTray() {}

// SetTrayTooltip is a no-op on unsupported platforms.
func SetTrayTooltip(string) {}
