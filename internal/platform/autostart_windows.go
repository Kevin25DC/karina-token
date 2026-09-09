//go:build windows

package platform

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

const runValueName = "Karina"

// SetStartWithSystem registers or removes the per-user auto-start entry in
// the Windows registry (HKCU\...\Run). No admin rights are required.
func SetStartWithSystem(enabled bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open run key: %w", err)
	}
	defer k.Close()

	if !enabled {
		if err := k.DeleteValue(runValueName); err != nil {
			if errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) {
				return nil
			}
			return fmt.Errorf("delete autostart value: %w", err)
		}
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	if err := k.SetStringValue(runValueName, `"`+exe+`"`); err != nil {
		return fmt.Errorf("set autostart value: %w", err)
	}
	return nil
}
