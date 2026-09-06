//go:build !windows && !darwin

package platform

// Notify is not yet implemented on this platform (Windows and macOS only,
// for now).
func Notify(_, _ string) error {
	return nil
}

// RequestNotificationPermission is a no-op: this platform either needs no
// explicit permission or notifications aren't implemented yet.
func RequestNotificationPermission() {}
