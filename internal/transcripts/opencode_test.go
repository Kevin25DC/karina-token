package transcripts

import (
	"database/sql"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// newOpenCodeDB builds a database with the tables and columns Karina reads,
// as found in a real opencode 1.18 installation.
func newOpenCodeDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "opencode.db")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	at := func(s string) int64 {
		ts, _ := time.Parse(time.RFC3339, s)
		return ts.UnixMilli()
	}
	stmts := []string{
		`CREATE TABLE session (id text PRIMARY KEY, project_id text NOT NULL, directory text NOT NULL)`,
		`CREATE TABLE message (id text PRIMARY KEY, session_id text NOT NULL, time_created integer NOT NULL, time_updated integer NOT NULL, data text NOT NULL)`,
		`INSERT INTO session VALUES ('ses_1', 'p', '/work/app')`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatal(err)
		}
	}
	insert := func(id, when, data string) {
		if _, err := db.Exec(`INSERT INTO message VALUES (?, 'ses_1', ?, ?, ?)`, id, at(when), at(when), data); err != nil {
			t.Fatal(err)
		}
	}
	insert("msg_user", "2026-09-01T10:00:00Z", `{"role":"user","time":{"created":1}}`)
	// Priced by OpenCode itself.
	insert("msg_a", "2026-09-01T10:01:00Z", `{"role":"assistant","modelID":"gpt-5","providerID":"openai","path":{"cwd":"/work/app","root":"/work/app"},"cost":0.25,"tokens":{"input":1000,"output":100,"reasoning":20,"total":1620,"cache":{"read":400,"write":100}}}`)
	// Cost 0 (subscription): falls back to Karina's price table; no cwd in the
	// message, so the session directory is used.
	insert("msg_b", "2026-09-01T10:05:00Z", `{"role":"assistant","modelID":"claude-sonnet-5","providerID":"anthropic","cost":0,"tokens":{"input":1000000,"output":0,"reasoning":0,"cache":{"read":0,"write":0}}}`)
	// Failed request: no tokens.
	insert("msg_err", "2026-09-01T10:06:00Z", `{"role":"assistant","modelID":"gpt-5","cost":0,"error":{"name":"x"},"tokens":{"input":0,"output":0,"reasoning":0,"cache":{"read":0,"write":0}}}`)
	// Before the range.
	insert("msg_old", "2026-08-01T10:00:00Z", `{"role":"assistant","modelID":"gpt-5","cost":9,"tokens":{"input":5,"output":5,"reasoning":0,"cache":{"read":0,"write":0}}}`)
	return path
}

func TestScanOpenCodeDatabase(t *testing.T) {
	db := newOpenCodeDB(t)
	since, _ := time.Parse(time.RFC3339, "2026-09-01T00:00:00Z")

	s, err := ScanRange(t.TempDir(), Range{OpenCodeDB: db, Since: since})
	if err != nil {
		t.Fatal(err)
	}
	if !s.Available || len(s.Agents) != 1 || s.Agents[0].Agent != AgentOpenCode || s.Agents[0].Turns != 2 {
		t.Fatalf("agents = %+v", s.Agents)
	}
	if len(s.Projects) != 1 || s.Projects[0].Path != "/work/app" || s.Projects[0].Sessions != 1 {
		t.Fatalf("projects = %+v", s.Projects)
	}
	// msg_a: input 1000, output 100+20 reasoning, cache read 400, write 100.
	want := Tokens{Input: 1001000, Output: 120, CacheCreation: 100, CacheRead: 400}
	if s.Total != want {
		t.Fatalf("total = %+v", s.Total)
	}
	// $0.25 from OpenCode + $2 for 1M Sonnet 5 input tokens.
	if math.Abs(s.CostUSD-2.25) > 1e-9 || s.UnpricedTokens != 0 {
		t.Fatalf("cost = %v unpriced = %d", s.CostUSD, s.UnpricedTokens)
	}
	if s.Projects[0].ActiveSeconds != 240 {
		t.Fatalf("active = %d", s.Projects[0].ActiveSeconds)
	}
	if !Detected(t.TempDir(), Range{OpenCodeDB: db}) {
		t.Fatal("OpenCode not detected")
	}
}

func TestOpenCodeUnknownSchemaIsIgnored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.db")
	if err := os.WriteFile(path, []byte("not a database"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := ScanRange(t.TempDir(), Range{OpenCodeDB: path})
	if err != nil || len(s.Agents) != 0 {
		t.Fatalf("summary = %+v err = %v", s.Agents, err)
	}
}
