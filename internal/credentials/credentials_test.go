package credentials

import (
	"errors"
	"strings"
	"testing"
)

// memoryBackend is a test-only in-memory implementation.
type memoryBackend struct {
	m map[string]map[string]string
}

func newMemory() *memoryBackend { return &memoryBackend{m: map[string]map[string]string{}} }

func (b *memoryBackend) Set(service, account, secret string) error {
	if b.m[service] == nil {
		b.m[service] = map[string]string{}
	}
	b.m[service][account] = secret
	return nil
}

func (b *memoryBackend) Get(service, account string) (string, error) {
	s, ok := b.m[service][account]
	if !ok {
		return "", ErrNotFound
	}
	return s, nil
}

func (b *memoryBackend) Delete(service, account string) error {
	if _, ok := b.m[service][account]; !ok {
		return ErrNotFound
	}
	delete(b.m[service], account)
	return nil
}

func TestStoreLifecycle(t *testing.T) {
	SetBackend(newMemory())
	defer SetBackend(keyringBackend{})

	s := NewStore()
	if s.Has("openai") {
		t.Fatal("should not have key")
	}
	if _, err := s.Get("openai"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound got %v", err)
	}
	if err := s.Save("openai", "sk-secret123"); err != nil {
		t.Fatal(err)
	}
	if !s.Has("openai") {
		t.Fatal("should have key now")
	}
	got, err := s.Get("openai")
	if err != nil || got != "sk-secret123" {
		t.Fatalf("got %q err %v", got, err)
	}
	if err := s.Delete("openai"); err != nil {
		t.Fatal(err)
	}
	if s.Has("openai") {
		t.Fatal("should be gone")
	}
}

func TestSaveRejectsEmpty(t *testing.T) {
	SetBackend(newMemory())
	defer SetBackend(keyringBackend{})

	if err := NewStore().Save("x", "  "); err == nil {
		t.Fatal("empty secret must be rejected")
	}
}

func TestPreview(t *testing.T) {
	if Preview("sk-abcdefgh123") != "sk-…123" {
		t.Fatalf("unexpected preview: %q", Preview("sk-abcdefgh123"))
	}
	if strings.Contains(Preview("sk-abcdefgh1234567890"), "abcdefgh") {
		t.Fatal("preview leaks secret")
	}
	if Preview("ab") != "••••" {
		t.Fatalf("short preview: %q", Preview("ab"))
	}
}
