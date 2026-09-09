//go:build linux

package platform

import (
	"fmt"
	"os"
	"path/filepath"
)

// SetStartWithSystem creates or removes an XDG autostart desktop entry.
func SetStartWithSystem(enabled bool) error {
	cfg := os.Getenv("XDG_CONFIG_HOME")
	if cfg == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		cfg = filepath.Join(home, ".config")
	}
	dir := filepath.Join(cfg, "autostart")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "karina.desktop")
	if !enabled {
		_ = os.Remove(path)
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	entry := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=Karina\nExec=%s\nX-GNOME-Autostart-enabled=true\n", exe)
	if err := os.WriteFile(path, []byte(entry), 0o644); err != nil {
		return err
	}
	return nil
}
