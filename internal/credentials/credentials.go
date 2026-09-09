// Package credentials stores provider API keys in the operating system
// secure store (Windows Credential Manager, macOS Keychain, Linux Secret
// Service). Keys are never persisted to disk by Karina itself and never
// logged.
package credentials

import (
	"errors"
	"strings"

	"github.com/zalando/go-keyring"
)

// Service is the keyring service/application name used for all entries.
const Service = "Karina"

// Backend abstracts the underlying OS credential store so it can be swapped
// in tests.
type Backend interface {
	Set(service, account, secret string) error
	Get(service, account string) (string, error)
	Delete(service, account string) error
}

type keyringBackend struct{}

func (keyringBackend) Set(service, account, secret string) error {
	return keyring.Set(service, account, secret)
}
func (keyringBackend) Get(service, account string) (string, error) {
	return keyring.Get(service, account)
}
func (keyringBackend) Delete(service, account string) error {
	return keyring.Delete(service, account)
}

var defaultBackend Backend = keyringBackend{}

// ErrNotFound is returned when no secret exists for an account.
var ErrNotFound = errors.New("credential not found")

// Store manages credentials scoped by provider id.
type Store struct {
	backend Backend
	service string
}

// NewStore returns a Store using the OS secure store.
func NewStore() *Store { return &Store{backend: defaultBackend, service: Service} }

// SetBackend overrides the backend (used by tests).
func SetBackend(b Backend) { defaultBackend = b }

// Save stores the secret for the given account (provider id).
func (s *Store) Save(account, secret string) error {
	if strings.TrimSpace(secret) == "" {
		return errors.New("empty secret")
	}
	if err := s.backend.Set(s.service, account, secret); err != nil {
		return err
	}
	return nil
}

// Get returns the stored secret for an account.
func (s *Store) Get(account string) (string, error) {
	secret, err := s.backend.Get(s.service, account)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", ErrNotFound
		}
		return "", err
	}
	return secret, nil
}

// Has reports whether a secret exists for an account.
func (s *Store) Has(account string) bool {
	_, err := s.Get(account)
	return err == nil
}

// Delete removes the secret for an account.
func (s *Store) Delete(account string) error {
	err := s.backend.Delete(s.service, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

// Preview returns a redacted preview of a key for safe display, e.g.
// "sk-…123". It must never reveal the full secret.
func Preview(secret string) string {
	if secret == "" {
		return ""
	}
	if len(secret) <= 8 {
		return "••••"
	}
	if i := strings.Index(secret, "-"); i >= 0 {
		return secret[:i+1] + "…" + secret[len(secret)-3:]
	}
	return secret[:3] + "…" + secret[len(secret)-3:]
}
