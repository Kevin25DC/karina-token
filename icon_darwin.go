//go:build darwin

package main

import _ "embed"

// trayIcon is a plain PNG: getlantern/systray's darwin backend forces the
// image to 16x16pt internally regardless of the source resolution, so no
// multi-size .ico container is needed here (unlike Windows).
//
//go:embed build/darwin/tray-icon.png
var trayIcon []byte
