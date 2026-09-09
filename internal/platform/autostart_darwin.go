//go:build darwin

package platform

import (
	"fmt"
	"os"
	"path/filepath"
)

// SetStartWithSystem creates or removes a per-user LaunchAgent that starts
// Karina at login.
func SetStartWithSystem(enabled bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "com.karina.app.plist")
	if !enabled {
		_ = os.Remove(path)
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>com.karina.app</string>
  <key>ProgramArguments</key><array><string>%s</string></array>
  <key>RunAtLoad</key><true/>
</dict></plist>
`, exe)
	if err := os.WriteFile(path, []byte(plist), 0o644); err != nil {
		return err
	}
	return nil
}
