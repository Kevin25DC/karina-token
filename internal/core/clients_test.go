package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"karina/internal/credentials"
	"karina/internal/domain"
)

func TestClientsAndReport(t *testing.T) {
	credentials.SetBackend(newMemoryBackend())
	root := t.TempDir()
	now := time.Now().UTC().Format(time.RFC3339)
	line := func(cwd string) string {
		return `{"type":"assistant","timestamp":"` + now + `","cwd":"` + cwd + `","sessionId":"s",` +
			`"message":{"model":"claude-sonnet-5","usage":{"input_tokens":1000000,"output_tokens":0}}}`
	}
	for name, cwd := range map[string]string{"a": "/work/a", "b": "/work/b"} {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "s.jsonl"), []byte(line(cwd)+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	dataDir := t.TempDir()
	s := New(nil)
	if err := s.Open(Options{DataDir: dataDir, SkipManualAuto: true, TranscriptsDir: root}); err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(s.Close)

	if err := s.SetProjectClient("/work/a", "  Acme  "); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSubscriptionPrice(100); err != nil {
		t.Fatal(err)
	}
	if got := s.Config().SubscriptionMonthlyUSD; got != 100 {
		t.Fatalf("subscription price = %v", got)
	}

	summary, err := s.ClaudeCodeUsage(domain.Span7d)
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Clients) != 2 || len(summary.ClientNames) != 1 || summary.ClientNames[0] != "Acme" {
		t.Fatalf("clients = %+v names = %v", summary.Clients, summary.ClientNames)
	}
	// 1M Sonnet 5 input tokens = $2 per project.
	if summary.CostUSD != 4 {
		t.Fatalf("cost = %v", summary.CostUSD)
	}

	data, err := s.ExportClientReportCSV("this_month")
	if err != nil {
		t.Fatal(err)
	}
	csv := string(data)
	for _, want := range []string{"Acme,a,/work/a,1,0.00,1000000", "Sin cliente,b,/work/b", "Acme,TOTAL CLIENTE", "TOTAL,,,0,0.00,2000000,0,0,0,2000000,4.00"} {
		if !strings.Contains(csv, want) {
			t.Fatalf("report missing %q:\n%s", want, csv)
		}
	}
	if _, err := s.ExportClientReportCSV("siempre"); err == nil {
		t.Fatal("unknown period must fail")
	}
	// Nothing happened last month.
	if report, err := s.ClientReport("last_month"); err != nil || len(report.Summary.Projects) != 0 {
		t.Fatalf("last month = %+v err=%v", report.Summary.Projects, err)
	}

	// Budgets: Acme has spent $2 this month.
	var alerts []string
	unsubscribe := s.Subscribe(func(e Event) {
		if e.Kind == EventThreshold {
			alerts = append(alerts, e.Message)
		}
	})
	defer unsubscribe()
	if err := s.SetClientBudget("Acme", 2.5); err != nil {
		t.Fatal(err)
	}
	budgets, err := s.ClientBudgets()
	if err != nil || len(budgets) != 1 || budgets[0].SpentUSD != 2 || budgets[0].Percent != 80 {
		t.Fatalf("budgets = %+v err=%v", budgets, err)
	}
	s.checkBudgets()
	s.checkBudgets() // the same level is announced only once
	if len(alerts) != 1 || !strings.Contains(alerts[0], "80%") {
		t.Fatalf("alerts after 80%% = %v", alerts)
	}
	if err := s.SetClientBudget("Acme", 1); err != nil {
		t.Fatal(err)
	}
	s.checkBudgets()
	if len(alerts) != 2 || !strings.Contains(alerts[1], "superado") {
		t.Fatalf("alerts after 100%% = %v", alerts)
	}
	if err := s.SetClientBudget("Acme", 0); err != nil {
		t.Fatal(err)
	}
	if budgets, _ = s.ClientBudgets(); len(budgets) != 0 {
		t.Fatalf("budget not removed: %+v", budgets)
	}

	// A folder rule covers the projects that have no assignment of their own.
	if err := s.SetFolderClient("/work/", "Globex"); err != nil {
		t.Fatal(err)
	}
	summary, _ = s.ClaudeCodeUsage(domain.Span7d)
	for _, p := range summary.Projects {
		want, inherited := "Acme", false
		if p.Path == "/work/b" {
			want, inherited = "Globex", true
		}
		if p.Client != want || p.ClientInherited != inherited {
			t.Fatalf("%s = %q inherited=%v", p.Path, p.Client, p.ClientInherited)
		}
	}
	if len(summary.ClientFolders) != 1 || summary.ClientFolders[0].Projects != 1 {
		t.Fatalf("folders = %+v", summary.ClientFolders)
	}
	// Re-typing the same folder replaces the rule; an empty client removes it.
	if err := s.SetFolderClient(`\WORK`, ""); err != nil {
		t.Fatal(err)
	}
	if summary, _ = s.ClaudeCodeUsage(domain.Span7d); len(summary.ClientFolders) != 0 {
		t.Fatalf("rule not removed: %+v", summary.ClientFolders)
	}

	// The subscription has its own pace: 5 min by default, never under 2.
	if got := s.Config().SubscriptionIntervalSeconds; got != 300 {
		t.Fatalf("default subscription interval = %d", got)
	}
	if err := s.SetSubscriptionInterval(15); err != nil {
		t.Fatal(err)
	}
	if got := s.Config().SubscriptionIntervalSeconds; got != 120 {
		t.Fatalf("clamped subscription interval = %d", got)
	}

	// The assignment survives a restart and can be cleared.
	s2 := New(nil)
	if err := s2.Open(Options{DataDir: dataDir, SkipManualAuto: true, TranscriptsDir: root}); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(s2.Close)
	if err := s2.SetProjectClient("/work/a", ""); err != nil {
		t.Fatal(err)
	}
	summary, _ = s2.ClaudeCodeUsage(domain.Span7d)
	if len(summary.ClientNames) != 0 || len(summary.Clients) != 1 {
		t.Fatalf("after clear: %+v", summary.Clients)
	}
}
