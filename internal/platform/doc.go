// Package platform holds OS-specific integrations: launching Karina at user
// logon, the system tray/menu bar icon (Windows and macOS; Linux pending),
// and native notifications (Windows and macOS; Linux pending). Only safe,
// per-user mechanisms are used; nothing here requires administrator
// privileges.
package platform
