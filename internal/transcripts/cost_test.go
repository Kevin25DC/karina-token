package transcripts

import (
	"math"
	"path/filepath"
	"testing"
	"time"
)

func responseLine(cwd, id, ts string, output int64) string {
	return `{"type":"assistant","timestamp":"` + ts + `","cwd":"` + cwd + `","sessionId":"s1","requestId":"req_` + id + `",` +
		`"message":{"id":"msg_` + id + `","model":"claude-sonnet-5","usage":{` +
		`"input_tokens":1000000,"output_tokens":` + itoa(output) + `,` +
		`"cache_creation_input_tokens":1000000,"cache_read_input_tokens":1000000,` +
		`"cache_creation":{"ephemeral_5m_input_tokens":0,"ephemeral_1h_input_tokens":1000000}}}}`
}

func TestScanCountsEachResponseOnceAndPricesIt(t *testing.T) {
	root := t.TempDir()
	// One response written as three lines (one per content block); the last
	// carries the final output count. A resumed session repeats it elsewhere.
	writeFile(t, filepath.Join(root, "-a", "s1.jsonl"), []string{
		responseLine("/a", "1", "2026-09-01T10:00:00.000Z", 10),
		responseLine("/a", "1", "2026-09-01T10:00:01.000Z", 10),
		responseLine("/a", "1", "2026-09-01T10:00:02.000Z", 1000000),
		responseLine("/a", "2", "2026-09-01T11:00:00.000Z", 0),
	})
	writeFile(t, filepath.Join(root, "-a", "s1-resumed.jsonl"), []string{
		responseLine("/a", "1", "2026-09-01T10:00:02.000Z", 1000000),
	})

	summary, err := Scan(root, time.Time{})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if got := summary.Models[0].Turns; got != 2 {
		t.Fatalf("turns = %d, want 2", got)
	}
	if got := summary.Total.Output; got != 1000000 {
		t.Fatalf("output = %d, want the final count once", got)
	}
	// Sonnet 5 per 1M: input 2, output 10, 1h cache write 4, cache read 0.2.
	// Response 1: 2+10+4+0.2 = 16.2; response 2: 2+0+4+0.2 = 6.2.
	if math.Abs(summary.CostUSD-22.4) > 1e-6 {
		t.Fatalf("cost = %v, want 22.4", summary.CostUSD)
	}
	if summary.UnpricedTokens != 0 || !summary.Models[0].Priced {
		t.Fatalf("unpriced = %d priced = %v", summary.UnpricedTokens, summary.Models[0].Priced)
	}
}

func TestActiveTimeAndRange(t *testing.T) {
	root := t.TempDir()
	// Two sessions on /a overlap; /b has a long break in the middle.
	writeFile(t, filepath.Join(root, "-a", "s1.jsonl"), []string{
		assistantLine("/a", "s1", "2026-09-01T10:00:00.000Z", 1, 0, 0, 0),
		assistantLine("/a", "s1", "2026-09-01T10:08:00.000Z", 1, 0, 0, 0),
	})
	writeFile(t, filepath.Join(root, "-a", "s2.jsonl"), []string{
		assistantLine("/a", "s2", "2026-09-01T10:04:00.000Z", 1, 0, 0, 0),
		assistantLine("/a", "s2", "2026-09-01T10:12:00.000Z", 1, 0, 0, 0),
	})
	writeFile(t, filepath.Join(root, "-b", "s3.jsonl"), []string{
		assistantLine("/b", "s3", "2026-09-01T10:00:00.000Z", 1, 0, 0, 0),
		assistantLine("/b", "s3", "2026-09-01T10:05:00.000Z", 1, 0, 0, 0),
		assistantLine("/b", "s3", "2026-09-01T11:00:00.000Z", 1, 0, 0, 0), // after a break
		assistantLine("/b", "s3", "2026-09-02T09:00:00.000Z", 1, 0, 0, 0), // outside Until
	})

	until, _ := time.Parse(time.RFC3339, "2026-09-02T00:00:00Z")
	summary, err := ScanRange(root, Range{Until: until})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int64{}
	for _, p := range summary.Projects {
		got[p.Path] = p.ActiveSeconds
	}
	// /a: 10:00 -> 10:12 merged across sessions = 12 min. /b: 5 min.
	if got["/a"] != 12*60 || got["/b"] != 5*60 {
		t.Fatalf("active = %v", got)
	}
	if summary.ActiveSeconds != 17*60 || summary.IdleGapMinutes != 10 {
		t.Fatalf("total = %d gap = %d", summary.ActiveSeconds, summary.IdleGapMinutes)
	}
	if summary.Total.Input != 7 {
		t.Fatalf("Until not applied: input = %d", summary.Total.Input)
	}

	// A longer idle gap bridges the break on /b.
	summary, _ = ScanRange(root, Range{Until: until, IdleGap: time.Hour})
	for _, p := range summary.Projects {
		if p.Path == "/b" && p.ActiveSeconds != 60*60 {
			t.Fatalf("/b with 1h gap = %d", p.ActiveSeconds)
		}
	}
}

func TestApplyRates(t *testing.T) {
	s := Summary{Projects: []ProjectUsage{
		{Path: "/a", ActiveSeconds: 2 * 3600, CostUSD: 10},
		{Path: "/b", ActiveSeconds: 3600, CostUSD: 5},
		{Path: "/c", ActiveSeconds: 3600, CostUSD: 1},
	}}
	s.AssignClients(map[string]string{"/a": "Acme", "/b": "Globex"}, nil)
	s.ApplyRates(map[string]float64{"Acme": 40})

	byName := map[string]ClientUsage{}
	for _, c := range s.Clients {
		byName[c.Name] = c
	}
	if a := byName["Acme"]; a.BillableUSD != 80 || a.MarginUSD != 70 || a.HourlyRateUSD != 40 {
		t.Fatalf("acme = %+v", a)
	}
	if g := byName["Globex"]; g.BillableUSD != 0 || g.MarginUSD != 0 {
		t.Fatalf("client without rate must not be valued: %+v", g)
	}
	if s.BillableUSD != 80 || s.MarginUSD != 70 || s.Projects[0].BillableUSD != 80 {
		t.Fatalf("totals = %v / %v", s.BillableUSD, s.MarginUSD)
	}
}

func TestAssignClientsByFolder(t *testing.T) {
	s := Summary{Projects: []ProjectUsage{
		{Path: `C:\Work\Acme\web`, CostUSD: 1},
		{Path: `C:\Work\Acme\legacy\api`, CostUSD: 2},
		{Path: `C:\Work\Acme\interno`, CostUSD: 4},
		{Path: `C:\Work\AcmeOtro\x`, CostUSD: 8},
	}}
	s.AssignClients(
		map[string]string{`C:\Work\Acme\interno`: "Propio"},
		map[string]string{`c:/work/acme/`: "Acme", `C:\Work\Acme\legacy`: "Acme Legacy"},
	)

	want := []struct {
		client    string
		inherited bool
	}{{"Acme", true}, {"Acme Legacy", true}, {"Propio", false}, {"", false}}
	for i, w := range want {
		if p := s.Projects[i]; p.Client != w.client || p.ClientInherited != w.inherited {
			t.Fatalf("project %d = %q inherited=%v, want %q %v", i, p.Client, p.ClientInherited, w.client, w.inherited)
		}
	}
	if len(s.ClientFolders) != 2 || s.ClientFolders[0].Projects != 1 || s.ClientFolders[1].Projects != 1 {
		t.Fatalf("folders = %+v", s.ClientFolders)
	}
	if len(s.ClientNames) != 3 {
		t.Fatalf("names = %v", s.ClientNames)
	}
}

func TestAssignClients(t *testing.T) {
	s := Summary{Projects: []ProjectUsage{
		{Path: "/a", Sessions: 2, CostUSD: 3, Tokens: Tokens{Input: 10}},
		{Path: "/b", Sessions: 1, CostUSD: 5, Tokens: Tokens{Input: 20}},
		{Path: "/c", Sessions: 1, CostUSD: 1},
	}}
	s.AssignClients(map[string]string{"/a": "Acme", "/b": "Acme", "/gone": "Otro"}, nil)

	if len(s.ClientNames) != 2 || s.ClientNames[0] != "Acme" || s.ClientNames[1] != "Otro" {
		t.Fatalf("client names = %v", s.ClientNames)
	}
	if len(s.Clients) != 2 {
		t.Fatalf("clients = %+v", s.Clients)
	}
	acme := s.Clients[0]
	if acme.Name != "Acme" || acme.Projects != 2 || acme.Sessions != 3 || acme.CostUSD != 8 || acme.Tokens.Input != 30 {
		t.Fatalf("acme = %+v", acme)
	}
	if s.Clients[1].Name != "" || s.Projects[2].Client != "" || s.Projects[0].Client != "Acme" {
		t.Fatalf("unassigned = %+v", s.Clients[1])
	}
}
