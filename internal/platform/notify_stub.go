//go:build !windows

package platform

// Notify is not yet implemented on this platform (Windows only for now).
func Notify(_, _ string) error {
	return nil
}
