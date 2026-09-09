//go:build windows

package platform

import (
	"sync"

	"github.com/getlantern/systray"
)

// TrayActions are the callbacks wired from the system tray menu into the
// application core. They run on the systray goroutine and must be safe to
// call from there.
type TrayActions struct {
	OnShow    func()
	OnHide    func()
	OnRefresh func()
	OnWidget  func()
	OnQuit    func()
}

// TrayAvailable reports whether a system tray is supported on this build.
const TrayAvailable = true

var (
	trayMu     sync.Mutex
	trayActive bool
)

// StartTray installs the system tray icon and menu. It is asynchronous; the
// icon appears shortly after the call. Returns an error only for immediate
// setup problems (the platform may also not support a tray at all).
func StartTray(icon []byte, actions TrayActions) error {
	trayMu.Lock()
	if trayActive {
		trayMu.Unlock()
		return nil
	}
	trayActive = true
	trayMu.Unlock()

	go systray.Run(
		func() {
			if len(icon) > 0 {
				systray.SetIcon(icon)
			}
			systray.SetTitle("Karina")
			systray.SetTooltip("Karina · monitor de uso de IA")

			open := systray.AddMenuItem("Abrir Karina", "Mostrar la ventana principal")
			hide := systray.AddMenuItem("Ocultar a la bandeja", "Ocultar la ventana")
			systray.AddSeparator()
			refresh := systray.AddMenuItem("Actualizar ahora", "Consultar a los proveedores")
			widget := systray.AddMenuItem("Modo widget (mini)", "Ventana compacta fija con las barras de consumo")
			systray.AddSeparator()
			quit := systray.AddMenuItem("Salir", "Cerrar Karina")

			go func() {
				for {
					select {
					case <-open.ClickedCh:
						call(actions.OnShow)
					case <-hide.ClickedCh:
						call(actions.OnHide)
					case <-refresh.ClickedCh:
						call(actions.OnRefresh)
					case <-widget.ClickedCh:
						call(actions.OnWidget)
					case <-quit.ClickedCh:
						call(actions.OnQuit)
					}
				}
			}()
		},
		func() {},
	)
	return nil
}

// StopTray removes the tray icon and releases resources.
func StopTray() {
	trayMu.Lock()
	if !trayActive {
		trayMu.Unlock()
		return
	}
	trayActive = false
	trayMu.Unlock()
	systray.Quit()
}

// SetTrayTooltip updates the tooltip shown over the tray icon.
func SetTrayTooltip(text string) {
	systray.SetTooltip(text)
}

func call(fn func()) {
	if fn != nil {
		fn()
	}
}
