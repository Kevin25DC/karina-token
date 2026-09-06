//go:build darwin

package platform

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include "tray_darwin.h"
*/
import "C"

import (
	"sync"
	"unsafe"
)

// TrayActions are the callbacks wired from the menu bar menu into the
// application core. They run on the main (AppKit) thread, same as any other
// Cocoa event, so they must not block.
type TrayActions struct {
	OnShow    func()
	OnHide    func()
	OnRefresh func()
	OnWidget  func()
	OnQuit    func()
}

// TrayAvailable reports whether a menu bar icon is supported on this build.
const TrayAvailable = true

var (
	trayMu      sync.Mutex
	trayActive  bool
	currentActs TrayActions
)

// StartTray installs the menu bar status item and menu. Safe to call once;
// the underlying NSStatusItem work is dispatched to the main thread.
func StartTray(icon []byte, actions TrayActions) error {
	trayMu.Lock()
	if trayActive {
		trayMu.Unlock()
		return nil
	}
	currentActs = actions
	trayActive = true
	trayMu.Unlock()

	var ptr *C.uchar
	if len(icon) > 0 {
		ptr = (*C.uchar)(unsafe.Pointer(&icon[0]))
	}
	C.karinaStartTray(ptr, C.int(len(icon)))
	return nil
}

// StopTray removes the menu bar status item.
func StopTray() {
	trayMu.Lock()
	if !trayActive {
		trayMu.Unlock()
		return
	}
	trayActive = false
	trayMu.Unlock()
	C.karinaStopTray()
}

// SetTrayTooltip updates the tooltip shown over the menu bar icon.
func SetTrayTooltip(text string) {
	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))
	C.karinaSetTooltip(cText)
}

//export karinaMenuAction
func karinaMenuAction(action C.int) {
	trayMu.Lock()
	actions := currentActs
	trayMu.Unlock()

	switch int(action) {
	case 0:
		call(actions.OnShow)
	case 1:
		call(actions.OnHide)
	case 2:
		call(actions.OnRefresh)
	case 3:
		call(actions.OnWidget)
	case 4:
		call(actions.OnQuit)
	}
}

func call(fn func()) {
	if fn != nil {
		fn()
	}
}
