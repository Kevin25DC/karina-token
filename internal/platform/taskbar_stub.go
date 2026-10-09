//go:build !windows

package platform

// SetTaskbarVisible is only implemented on Windows; elsewhere the window
// keeps its normal taskbar/Dock presence.
func SetTaskbarVisible(bool) bool { return false }

// SetWindowShape is only implemented on Windows.
func SetWindowShape(_, _, _ int, _ bool) bool { return false }

// ClearWindowShape is only implemented on Windows.
func ClearWindowShape() {}
