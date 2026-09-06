package transcripts

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeFile(t *testing.T, path string, lines []string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assistantLine(cwd, sessionID, timestamp string, input, output, cacheCreate, cacheRead int64) string {
	return `{"type":"assistant","timestamp":"` + timestamp + `","cwd":"` + cwd + `","sessionId":"` + sessionID + `",` +
		`"message":{"model":"claude-sonnet-5","usage":{` +
		`"input_tokens":` + itoa(input) + `,"output_tokens":` + itoa(output) + `,` +
		`"cache_creation_input_tokens":` + itoa(cacheCreate) + `,"cache_read_input_tokens":` + itoa(cacheRead) + `}}}`
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

func TestScanMissingRootIsUnavailable(t *testing.T) {
	dir := t.TempDir()
	summary, err := Scan(filepath.Join(dir, "does-not-exist"), time.Time{})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if summary.Available {
		t.Fatal("expected Available = false for a missing root")
	}
}

func TestScanAggregatesByProjectAndDay(t *testing.T) {
	root := t.TempDir()

	writeFile(t, filepath.Join(root, "-Users-kevin-app-a", "session1.jsonl"), []string{
		`{"type":"mode","sessionId":"s1"}`, // non-usage line, should be ignored
		assistantLine("/Users/kevin/app-a", "s1", "2026-09-01T10:00:00.000Z", 100, 50, 10, 5),
		`{"type":"user","sessionId":"s1"}`, // no usage, ignored
		assistantLine("/Users/kevin/app-a", "s1", "2026-09-01T11:00:00.000Z", 200, 80, 0, 30),
		"not even json", // corrupt line, must not abort the scan
	})
	writeFile(t, filepath.Join(root, "-Users-kevin-app-b", "session2.jsonl"), []string{
		assistantLine("/Users/kevin/app-b", "s2", "2026-09-02T09:00:00.000Z", 5000, 1000, 0, 0),
	})
	// A subagent transcript nested one level deeper, same project as app-a.
	writeFile(t, filepath.Join(root, "-Users-kevin-app-a", "session1", "subagents", "agent1.jsonl"), []string{
		assistantLine("/Users/kevin/app-a", "s1-sub", "2026-09-01T10:30:00.000Z", 40, 20, 0, 0),
	})

	summary, err := Scan(root, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if !summary.Available {
		t.Fatal("expected Available = true")
	}

	wantAppA := Tokens{Input: 100 + 200 + 40, Output: 50 + 80 + 20, CacheCreation: 10, CacheRead: 5 + 30}
	wantAppB := Tokens{Input: 5000, Output: 1000}

	if len(summary.Projects) != 2 {
		t.Fatalf("got %d projects, want 2: %+v", len(summary.Projects), summary.Projects)
	}
	// Sorted by total tokens desc: app-b (6000) before app-a (~415).
	if summary.Projects[0].Path != "/Users/kevin/app-b" {
		t.Fatalf("projects[0] = %q, want app-b first (higher total)", summary.Projects[0].Path)
	}
	if summary.Projects[0].Tokens != wantAppB {
		t.Fatalf("app-b tokens = %+v, want %+v", summary.Projects[0].Tokens, wantAppB)
	}
	appA := summary.Projects[1]
	if appA.Path != "/Users/kevin/app-a" || appA.Tokens != wantAppA {
		t.Fatalf("app-a = %+v, want path=/Users/kevin/app-a tokens=%+v", appA, wantAppA)
	}
	if appA.Sessions != 2 {
		t.Fatalf("app-a sessions = %d, want 2 (s1 + s1-sub)", appA.Sessions)
	}

	if len(summary.Days) != 2 {
		t.Fatalf("got %d days, want 2: %+v", len(summary.Days), summary.Days)
	}
	if summary.Days[0].Date != "2026-09-01" || summary.Days[1].Date != "2026-09-02" {
		t.Fatalf("days out of order: %+v", summary.Days)
	}

	wantTotal := Tokens{}
	wantTotal.add(wantAppA)
	wantTotal.add(wantAppB)
	if summary.Total != wantTotal {
		t.Fatalf("total = %+v, want %+v", summary.Total, wantTotal)
	}
}

func TestScanFiltersBySince(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "-Users-kevin-app", "s.jsonl"), []string{
		assistantLine("/Users/kevin/app", "s1", "2026-01-01T00:00:00.000Z", 1000, 1000, 0, 0),
		assistantLine("/Users/kevin/app", "s1", "2026-09-01T00:00:00.000Z", 7, 3, 0, 0),
	})

	summary, err := Scan(root, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if summary.Total.Total() != 10 {
		t.Fatalf("total = %+v, want only the September record (10 tokens)", summary.Total)
	}
}

func TestScanEmptyDirectoryIsAvailableWithNoData(t *testing.T) {
	root := t.TempDir()
	summary, err := Scan(root, time.Time{})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if !summary.Available {
		t.Fatal("an existing-but-empty directory should still be Available")
	}
	if len(summary.Projects) != 0 || summary.Total.Total() != 0 {
		t.Fatalf("expected no data, got %+v", summary)
	}
}
