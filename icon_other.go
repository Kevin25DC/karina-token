//go:build !windows && !darwin

package main

// trayIcon is unused on platforms without a tray implementation yet (see
// internal/platform/tray_stub.go).
var trayIcon []byte
