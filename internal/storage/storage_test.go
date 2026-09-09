package storage

import (
	"os"
	"testing"
	"time"

	"karina/internal/domain"
)

func TestAppendAndRead(t *testing.T) {
	dir := t.TempDir()
	st, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)

	pts := []domain.HistoryPoint{
		{At: base, UsedTokens: 100, LimitTokens: 1000},
		{At: base.Add(30 * time.Second), UsedTokens: 200, LimitTokens: 1000},
		{At: base.AddDate(0, 0, -1), UsedTokens: 50, LimitTokens: 1000},
		{At: base.AddDate(0, 0, -3), UsedTokens: 10, LimitTokens: 1000},
	}
	for _, p := range pts {
		if err := st.AppendPoint("openai", p); err != nil {
			t.Fatal(err)
		}
	}

	got, err := st.Points("openai", time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 points got %d", len(got))
	}
	// ascending order
	for i := 1; i < len(got); i++ {
		if got[i].At.Before(got[i-1].At) {
			t.Fatalf("not sorted: %v then %v", got[i-1].At, got[i].At)
		}
	}
	if got[0].UsedTokens != 50 {
		t.Fatalf("first point wrong: %+v", got[0])
	}

	none, err := st.Points("anthropic", base)
	if err != nil || len(none) != 0 {
		t.Fatalf("expected none for missing provider: %d %v", len(none), err)
	}
}

func TestRetentionAndProviderScoped(t *testing.T) {
	dir := t.TempDir()
	st, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	old := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	if err := st.AppendPoint("gemini", domain.HistoryPoint{At: old, UsedTokens: 1}); err != nil {
		t.Fatal(err)
	}
	// Prune older than 30 days ago.
	st.PruneAll(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	got, _ := st.Points("gemini", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	if len(got) != 0 {
		t.Fatalf("expected pruned, got %d", len(got))
	}
	if err := st.DropProvider("gemini"); err != nil {
		t.Fatal(err)
	}
}

func TestCorruptLineIsSkipped(t *testing.T) {
	dir := t.TempDir()
	st, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := st.AppendPoint("deepseek", domain.HistoryPoint{At: now, BalanceTotal: 42}); err != nil {
		t.Fatal(err)
	}
	// Inject a corrupt line directly into the day file.
	day := now.Format("2006-01-02")
	path := dir + "/history/deepseek/" + day + ".jsonl"
	if err := appendString(path, "this is not json\n"); err != nil {
		t.Fatal(err)
	}
	if err := appendString(path, `{"at":"2026-09-09T12:00:00Z","provider":"deepseek"}`+"\n"); err != nil {
		t.Fatal(err)
	}
	got, err := st.Points("deepseek", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 valid points, got %d", len(got))
	}
}

func appendString(path, s string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(s)
	return err
}
