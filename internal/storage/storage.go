// Package storage persists local usage history as append-only JSONL files,
// one file per provider per day. Karina writes the observations it
// captures from real provider APIs; it never fabricates history.
package storage

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"karina/internal/domain"
)

const retentionDays = 31

// Store persists HistoryPoint observations.
type Store struct {
	mu  sync.Mutex
	dir string
}

// New creates a Store rooted at dir/history. It ensures the directory tree
// exists and prunes expired day files.
func New(dir string) (*Store, error) {
	s := &Store{dir: filepath.Join(dir, "history")}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return nil, fmt.Errorf("create history dir: %w", err)
	}
	s.PruneAll(time.Now().AddDate(0, 0, -retentionDays))
	return s, nil
}

func (s *Store) providerDir(provider domain.ProviderID) string {
	return filepath.Join(s.dir, string(provider))
}

func (s *Store) dayFile(provider domain.ProviderID, t time.Time) string {
	return filepath.Join(s.providerDir(provider), t.Format("2006-01-02")+".jsonl")
}

// AppendPoint writes a single observation to the day file for its timestamp.
func (s *Store) AppendPoint(provider domain.ProviderID, p domain.HistoryPoint) error {
	p.Provider = provider
	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("encode point: %w", err)
	}
	data = append(data, '\n')

	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.dayFile(provider, p.At)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create day dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open history file: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("append history: %w", err)
	}
	return nil
}

// Points returns all observations for a provider at or after since, ascending.
func (s *Store) Points(provider domain.ProviderID, since time.Time) ([]domain.HistoryPoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := s.providerDir(provider)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list history: %w", err)
	}

	minDay := since.Format("2006-01-02")
	var out []domain.HistoryPoint
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		day := strings.TrimSuffix(e.Name(), ".jsonl")
		if day < minDay {
			continue
		}
		pts, err := readFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		for _, p := range pts {
			if !p.At.Before(since) {
				out = append(out, p)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out, nil
}

func readFile(path string) ([]domain.HistoryPoint, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open history: %w", err)
	}
	defer f.Close()

	var pts []domain.HistoryPoint
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var p domain.HistoryPoint
		if err := json.Unmarshal([]byte(line), &p); err != nil {
			// A single corrupt observation must never break the store.
			continue
		}
		pts = append(pts, p)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read history: %w", err)
	}
	return pts, nil
}

// DropProvider deletes all history for a provider.
func (s *Store) DropProvider(provider domain.ProviderID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.RemoveAll(s.providerDir(provider))
}

// PruneAll removes day files older than olderThan across all providers.
func (s *Store) PruneAll(olderThan time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	providers, _ := os.ReadDir(s.dir)
	cutoff := olderThan.Format("2006-01-02")
	for _, pv := range providers {
		if !pv.IsDir() {
			continue
		}
		files, _ := os.ReadDir(filepath.Join(s.dir, pv.Name()))
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			day := strings.TrimSuffix(f.Name(), ".jsonl")
			if day < cutoff {
				_ = os.Remove(filepath.Join(s.dir, pv.Name(), f.Name()))
			}
		}
	}
}
