//go:build darwin

package platform

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework UserNotifications

#include <stdlib.h>

void karinaShowNotification(const char *title, const char *message);
void karinaRequestNotificationPermission(void);
*/
import "C"

import "unsafe"

// Notify shows a native macOS notification via UNUserNotificationCenter.
//
// The first call triggers the OS "Karina would like to send you
// notifications" permission prompt; until the user allows it, notifications
// are silently dropped (Notify still returns nil — there is no synchronous
// way to know the outcome, and treating "not yet granted" as an error would
// misreport a normal, expected state). This mirrors the same "best-effort"
// contract as the in-app toast: the toast is the guaranteed channel, this is
// bonus. See RequestNotificationPermission to ask earlier than that.
//
// The older NSUserNotificationCenter needs no such prompt, but was found in
// practice to be silently inert for an unsigned, ad-hoc-built app bundle on
// current macOS — no error, the banner just never appears. UNUserNotification
// is the one Apple actually keeps working for third-party apps.
func Notify(title, message string) error {
	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle))
	cMessage := C.CString(message)
	defer C.free(unsafe.Pointer(cMessage))

	C.karinaShowNotification(cTitle, cMessage)
	return nil
}

// RequestNotificationPermission asks the OS for permission to show
// notifications, without delivering one. Call it once at a moment the user
// is actively engaged with the app (e.g. right after onboarding) so the
// system prompt appears in a context they'll recognize, instead of the
// first time silently as a side effect of the polling loop crossing a
// threshold. A no-op on platforms that don't require explicit permission.
func RequestNotificationPermission() {
	C.karinaRequestNotificationPermission()
}
