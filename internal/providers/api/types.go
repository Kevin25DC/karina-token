// Package api contains the provider abstraction: the Provider interface and
// the shared plumbing adapters use (HTTP helpers, error taxonomy, rate-limit
// header parsing). Adapters import this package; higher layers import
// adapters. This layering prevents import cycles.
package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"karina/internal/domain"
)

// Config carries everything an adapter needs for one request cycle.
// APIKey must never be logged or sent anywhere but to the provider.
type Config struct {
	APIKey  string
	BaseURL string
	HTTP    *http.Client
	Logger  *slog.Logger

	// ValidationOnly makes the adapter skip expensive extra lookups so that
	// "Test connection" stays cheap.
	ValidationOnly bool
}

// Provider is the contract every provider adapter implements. The dashboard
// and scheduler only ever see this interface.
type Provider interface {
	// ID returns the stable provider id, e.g. "anthropic".
	ID() string
	// DisplayName returns the human readable name, e.g. "Anthropic Claude".
	DisplayName() string
	// Capabilities reports which real metrics this adapter is able to expose.
	Capabilities() []domain.Capability
	// Refresh queries the provider and returns an honest snapshot. It never
	// fabricates values: unavailable metrics are simply not set.
	Refresh(ctx context.Context, cfg Config) (domain.ProviderState, error)
}

// ErrorKind classifies provider failures so the UI and scheduler can react
// consistently instead of crashing.
type ErrorKind int

const (
	KindNetwork ErrorKind = iota
	KindInvalidCredentials
	KindPermission
	KindPayment
	KindRateLimited
	KindNotFound
	KindServer
	KindTimeout
	KindUnexpected
)

// ProviderError is the shared error taxonomy across adapters.
type ProviderError struct {
	Kind       ErrorKind
	Status     int
	Message    string
	RetryAfter time.Duration
	Wrapped    error
}

func (e *ProviderError) Error() string {
	msg := e.Message
	if msg == "" && e.Wrapped != nil {
		msg = e.Wrapped.Error()
	}
	if e.Status > 0 {
		return fmt.Sprintf("provider http %d: %s", e.Status, msg)
	}
	if msg != "" {
		return msg
	}
	if e.Wrapped != nil {
		return e.Wrapped.Error()
	}
	return e.Kind.String()
}

func (e *ProviderError) Unwrap() error { return e.Wrapped }

func (k ErrorKind) String() string {
	switch k {
	case KindNetwork:
		return "network_error"
	case KindInvalidCredentials:
		return "invalid_credentials"
	case KindPermission:
		return "permission_error"
	case KindPayment:
		return "payment_error"
	case KindRateLimited:
		return "rate_limited"
	case KindNotFound:
		return "not_found"
	case KindServer:
		return "server_error"
	case KindTimeout:
		return "timeout"
	default:
		return "unexpected_error"
	}
}

// NewError builds a *ProviderError.
func NewError(kind ErrorKind, status int, message string, wrapped error) *ProviderError {
	return &ProviderError{Kind: kind, Status: status, Message: message, Wrapped: wrapped}
}

// StateForError maps an error onto a ProviderState while preserving the last
// known good values of the previous state (so the UI degrades gracefully).
func StateForError(p Provider, err error, prev domain.ProviderState) domain.ProviderState {
	st := prev
	st.UpdatedAt = time.Now()
	st.Error = err.Error()

	switch {
	case isKind(err, KindInvalidCredentials):
		st.Status = domain.StatusInvalidCredentials
		st.StatusMsg = "Clave API no válida"
	case isKind(err, KindRateLimited):
		st.Status = domain.StatusRateLimited
		st.StatusMsg = "Límite alcanzado por el proveedor"
		if pe2 := asProviderError(err); pe2 != nil && pe2.RetryAfter > 0 {
			st.StatusMsg = fmt.Sprintf("Límite alcanzado — reintenta en %s", shortDur(pe2.RetryAfter))
		}
	case isKind(err, KindPayment):
		st.Status = domain.StatusError
		st.StatusMsg = "Error de facturación en la cuenta"
	default:
		st.Status = domain.StatusError
		st.StatusMsg = "No se pudo obtener el uso"
	}
	return st
}

func isKind(err error, kind ErrorKind) bool {
	return asProviderError(err) != nil && asProviderError(err).Kind == kind
}

func asProviderError(err error) *ProviderError {
	for err != nil {
		if pe, ok := err.(*ProviderError); ok {
			return pe
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return nil
		}
		err = u.Unwrap()
	}
	return nil
}

func shortDur(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}
