//go:build windows

package main

import _ "embed"

// trayIcon must be a real .ico (systray's Windows backend loads it via
// LoadImage/IMAGE_ICON, which rejects a plain .png despite the misleading
// "operation completed successfully" it reports on failure).
//
//go:embed build/windows/icon.ico
var trayIcon []byte
