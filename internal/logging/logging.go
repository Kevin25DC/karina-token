// Package logging provides a small helper to build structured slog loggers.
// It never logs credentials; callers must not pass secrets to it.
package logging

import (
	"log/slog"
	"os"
)

// New returns a structured logger writing to stderr in a human-friendly,
// key=value text format.
func New(level slog.Level) *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}
