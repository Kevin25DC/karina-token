//go:build windows

package platform

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Notify shows a transient Windows notification-area alert.
//
// It deliberately uses the classic Shell_NotifyIcon "balloon" API instead of
// the modern WinRT toast API (Windows.UI.Notifications). WinRT toasts
// require the calling process to carry a registered AppUserModelID, which is
// normally provided by a Start Menu shortcut created at install time; an
// unpackaged, directly-launched exe like Karina.exe does not have one, and
// calling CreateToastNotifier() fails with "Element not found"
// (HRESULT 0x80070490). Shell_NotifyIcon has no such requirement — and since
// Windows 10, balloon notifications are rendered as ordinary toasts in the
// notification area / Action Center, so the visual result is the same.
func Notify(title, message string) error {
	notifyInitOnce.Do(initNotifyWindow)
	if !notifyReady {
		return errNotifyUnavailable
	}

	titleU, _ := windows.UTF16FromString(truncateRunes(title, 63))
	msgU, _ := windows.UTF16FromString(truncateRunes(message, 255))

	id := atomic.AddUint32(&notifySeq, 1)

	// Two steps, matching how every reliable balloon implementation does it
	// (including .NET's NotifyIcon): first NIM_ADD a plain icon, then
	// NIM_MODIFY with the NIF_INFO fields to trigger the balloon. Setting
	// NIF_INFO already on NIM_ADD is accepted by the API (no error) but
	// silently fails to render the balloon on some Windows builds.
	add := notifyIconData{
		Wnd:             notifyWindow,
		ID:              id,
		Flags:           nifMessage | nifIcon | nifTip,
		CallbackMessage: wmNotifyCallback,
		Icon:            notifyIconHandle,
	}
	add.Size = uint32(unsafe.Sizeof(add))
	copy(add.Tip[:], titleU)

	if res, _, callErr := pShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&add))); res == 0 {
		return callErr
	}

	mod := notifyIconData{
		Wnd:       notifyWindow,
		ID:        id,
		Flags:     nifInfo,
		InfoFlags: niifInfo,
	}
	mod.Size = uint32(unsafe.Sizeof(mod))
	copy(mod.Info[:], msgU)
	copy(mod.InfoTitle[:], titleU)

	if res, _, callErr := pShellNotifyIcon.Call(nimModify, uintptr(unsafe.Pointer(&mod))); res == 0 {
		del := notifyIconData{Wnd: notifyWindow, ID: id}
		del.Size = uint32(unsafe.Sizeof(del))
		pShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&del)))
		return callErr
	}

	// The icon only needs to exist long enough for Explorer to pick up and
	// render the balloon; remove it afterwards so nothing lingers in the
	// notification-area overflow.
	go func() {
		time.Sleep(8 * time.Second)
		del := notifyIconData{Wnd: notifyWindow, ID: id}
		del.Size = uint32(unsafe.Sizeof(del))
		pShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&del)))
	}()
	return nil
}

var errNotifyUnavailable = errors.New("no se pudo preparar la ventana de notificaciones")

// --- Win32 plumbing (mirrors the subset used by getlantern/systray) -------

var (
	notifyUser32   = windows.NewLazySystemDLL("user32.dll")
	notifyShell32  = windows.NewLazySystemDLL("shell32.dll")
	notifyKernel32 = windows.NewLazySystemDLL("kernel32.dll")

	pNotifyRegisterClass   = notifyUser32.NewProc("RegisterClassExW")
	pNotifyCreateWindowEx  = notifyUser32.NewProc("CreateWindowExW")
	pNotifyDefWindowProc   = notifyUser32.NewProc("DefWindowProcW")
	pNotifyLoadIcon        = notifyUser32.NewProc("LoadIconW")
	pShellNotifyIcon       = notifyShell32.NewProc("Shell_NotifyIconW")
	pNotifyGetModuleHandle = notifyKernel32.NewProc("GetModuleHandleW")
)

type notifyWndClassEx struct {
	Size, Style                        uint32
	WndProc                            uintptr
	ClsExtra, WndExtra                 int32
	Instance, Icon, Cursor, Background windows.Handle
	MenuName, ClassName                *uint16
	IconSm                             windows.Handle
}

// notifyIconData mirrors NOTIFYICONDATAW.
// https://learn.microsoft.com/windows/win32/api/shellapi/ns-shellapi-notifyicondataw
type notifyIconData struct {
	Size                       uint32
	Wnd                        windows.Handle
	ID, Flags, CallbackMessage uint32
	Icon                       windows.Handle
	Tip                        [128]uint16
	State, StateMask           uint32
	Info                       [256]uint16
	Timeout, Version           uint32
	InfoTitle                  [64]uint16
	InfoFlags                  uint32
	GuidItem                   windows.GUID
	BalloonIcon                windows.Handle
}

const (
	nimAdd    = 0x00000000
	nimModify = 0x00000001
	nimDelete = 0x00000002

	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004
	nifInfo    = 0x00000010

	niifInfo = 0x00000001

	idiApplication = 32512

	// wmNotifyCallback is the WM_USER-based message Explorer uses to report
	// icon interactions back to our window. Karina does not act on it (no
	// message pump runs), it is only required for Explorer to treat the icon
	// as a normal, fully-registered notify icon capable of showing balloons.
	wmNotifyCallback = 0x0400 + 1 // WM_USER + 1
)

var (
	notifyInitOnce   sync.Once
	notifyWindow     windows.Handle
	notifyIconHandle windows.Handle
	notifyReady      bool
	notifySeq        uint32
)

// initNotifyWindow creates a hidden, message-only window used solely to own
// the transient notify-icon entries. It never needs a message pump: adding
// and removing a notify icon does not require the window to process
// messages.
func initNotifyWindow() {
	instance, _, _ := pNotifyGetModuleHandle.Call(0)

	className, err := windows.UTF16PtrFromString("KarinaNotifyClass")
	if err != nil {
		return
	}
	windowName, err := windows.UTF16PtrFromString("")
	if err != nil {
		return
	}

	wcex := notifyWndClassEx{
		WndProc:   windows.NewCallback(notifyWndProc),
		Instance:  windows.Handle(instance),
		ClassName: className,
	}
	wcex.Size = uint32(unsafe.Sizeof(wcex))
	if res, _, _ := pNotifyRegisterClass.Call(uintptr(unsafe.Pointer(&wcex))); res == 0 {
		return
	}

	const cwUseDefault = 0x80000000
	hwnd, _, _ := pNotifyCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		0,
		cwUseDefault, cwUseDefault, cwUseDefault, cwUseDefault,
		0, 0,
		uintptr(instance),
		0,
	)
	if hwnd == 0 {
		return
	}
	notifyWindow = windows.Handle(hwnd)

	iconH, _, _ := pNotifyLoadIcon.Call(0, idiApplication)
	notifyIconHandle = windows.Handle(iconH)
	notifyReady = true
}

func notifyWndProc(hWnd windows.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	r, _, _ := pNotifyDefWindowProc.Call(uintptr(hWnd), uintptr(msg), wParam, lParam)
	return r
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) > max {
		return string(r[:max])
	}
	return s
}
