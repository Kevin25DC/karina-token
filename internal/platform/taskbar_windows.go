//go:build windows

package platform

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The widget should float on the desktop without looking like an open
// application. On Windows that is the WS_EX_TOOLWINDOW extended style: a tool
// window has no taskbar button and does not appear in Alt+Tab.

const (
	gwlExStyle     = ^uintptr(19) // GWL_EXSTYLE = -20
	wsExToolWindow = 0x00000080
	wsExAppWindow  = 0x00040000
	swHide         = 0
	swShowNA       = 8
	// wailsWindowClass is the window class Wails v2 registers for the app
	// window; the tray library owns other windows in the same process.
	wailsWindowClass = "wailsWindow"
)

var (
	user32               = windows.NewLazySystemDLL("user32.dll")
	gdi32                = windows.NewLazySystemDLL("gdi32.dll")
	procGetWindowLongPtr = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtr = user32.NewProc("SetWindowLongPtrW")
	procShowWindow       = user32.NewProc("ShowWindow")
	procSetWindowRgn     = user32.NewProc("SetWindowRgn")
	procGetDpiForWindow  = user32.NewProc("GetDpiForWindow")
	procCreateRoundRect  = gdi32.NewProc("CreateRoundRectRgn")
)

// SetWindowShape clips the application window to a rounded rectangle of the
// given size (in logical pixels, as passed to the window-size call), so that
// nothing of the window shows outside the widget's rounded corners. The
// window is a plain rectangle with a translucent backdrop: without the clip,
// that backdrop is visible as grey wedges in the corners.
//
// With squareTop the top corners stay square (widget docked to the screen
// edge). It reports whether the window was found.
func SetWindowShape(width, height, radius int, squareTop bool) bool {
	hwnd := appWindow()
	if hwnd == 0 {
		return false
	}
	scale := func(v int) uintptr {
		dpi, _, _ := procGetDpiForWindow.Call(uintptr(hwnd))
		if dpi == 0 {
			dpi = 96
		}
		// Round up: a region slightly larger than the window clips nothing
		// extra, one slightly smaller would cut the content.
		return (uintptr(v)*dpi + 95) / 96
	}
	w, h, d := scale(width), scale(height), scale(radius*2)
	top := uintptr(0)
	if squareTop {
		// Start the rounded rectangle above the window so its top corners
		// fall outside and the visible top edge is straight.
		top = ^(d - 1) // -d as a two's-complement int
	}
	rgn, _, _ := procCreateRoundRect.Call(0, top, w+1, h+1, d, d)
	if rgn == 0 {
		return false
	}
	// On success the system owns the region; it must not be deleted here.
	procSetWindowRgn.Call(uintptr(hwnd), rgn, 1)
	return true
}

// ClearWindowShape gives the window back its normal rectangular shape.
func ClearWindowShape() {
	if hwnd := appWindow(); hwnd != 0 {
		procSetWindowRgn.Call(uintptr(hwnd), 0, 1)
	}
}

// appWindow returns the main window of this process, or 0 if not found.
func appWindow() windows.HWND {
	pid := windows.GetCurrentProcessId()
	var found windows.HWND
	cb := syscall.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
		var owner uint32
		if _, err := windows.GetWindowThreadProcessId(hwnd, &owner); err != nil || owner != pid {
			return 1 // keep enumerating
		}
		class := make([]uint16, 64)
		n, err := windows.GetClassName(hwnd, &class[0], int32(len(class)))
		if err != nil || windows.UTF16ToString(class[:n]) != wailsWindowClass {
			return 1
		}
		found = hwnd
		return 0 // stop
	})
	// EnumWindows reports an error when the callback stops it early; the
	// result is in found either way.
	_ = windows.EnumWindows(cb, unsafe.Pointer(nil))
	return found
}

// SetTaskbarVisible shows or hides the application's button in the taskbar
// (and its entry in Alt+Tab) without closing the window. It reports whether
// the window was found.
func SetTaskbarVisible(visible bool) bool {
	hwnd := appWindow()
	if hwnd == 0 {
		return false
	}
	style, _, _ := procGetWindowLongPtr.Call(uintptr(hwnd), gwlExStyle)
	want := style
	if visible {
		want = (style &^ wsExToolWindow) | wsExAppWindow
	} else {
		want = (style &^ wsExAppWindow) | wsExToolWindow
	}
	if want == style {
		return true
	}
	// The taskbar only notices the change if the window is hidden while the
	// style is swapped.
	wasVisible := windows.IsWindowVisible(hwnd)
	if wasVisible {
		procShowWindow.Call(uintptr(hwnd), swHide)
	}
	procSetWindowLongPtr.Call(uintptr(hwnd), gwlExStyle, want)
	if wasVisible {
		procShowWindow.Call(uintptr(hwnd), swShowNA)
	}
	return true
}
