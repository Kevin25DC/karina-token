package transcripts

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Fixtures below follow the session formats defined in the tools' own source
// (see agents.go); they are not captures of real sessions.

func codexTokenCount(ts string, in, cached, out, total int64) string {
	usage := `{"input_tokens":` + itoa(in) + `,"cached_input_tokens":` + itoa(cached) +
		`,"output_tokens":` + itoa(out) + `,"reasoning_output_tokens":0,"total_tokens":` + itoa(total) + `}`
	return `{"timestamp":"` + ts + `","type":"event_msg","payload":{"type":"token_count","info":{` +
		`"total_token_usage":` + usage + `,"last_token_usage":` + usage + `,"model_context_window":272000},"rate_limits":null}}`
}

func writeCodexSession(t *testing.T, codexRoot, sub string) {
	t.Helper()
	writeFile(t, filepath.Join(codexRoot, sub, "2026", "09", "01", "rollout-2026-09-01T10-00-00-sess-1.jsonl"), []string{
		`{"timestamp":"2026-09-01T10:00:00.000Z","type":"session_meta","payload":{"id":"sess-1","timestamp":"2026-09-01T10:00:00.000Z","cwd":"/work/app","originator":"codex_cli_rs","cli_version":"0.150.0"}}`,
		`{"timestamp":"2026-09-01T10:00:01.000Z","type":"turn_context","payload":{"cwd":"/work/app","approval_policy":"on-request","model":"gpt-5-codex"}}`,
		`{"timestamp":"2026-09-01T10:00:02.000Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"secret prompt"}]}}`,
		codexTokenCount("2026-09-01T10:00:05.000Z", 1000, 400, 100, 1100),
		codexTokenCount("2026-09-01T10:00:06.000Z", 1000, 400, 100, 1100), // same snapshot again
		`{"timestamp":"2026-09-01T10:00:07.000Z","type":"event_msg","payload":{"type":"token_count","info":null,"rate_limits":null}}`,
		codexTokenCount("2026-09-01T10:03:00.000Z", 2500, 1000, 300, 2800),
	})
}

func TestScanCodexSessions(t *testing.T) {
	claude := t.TempDir()
	codex := t.TempDir()
	writeCodexSession(t, codex, "sessions")
	// The archived copy of the same rollout must not count twice.
	writeCodexSession(t, codex, "archived_sessions")
	// Config files in the Codex home are not sessions.
	writeFile(t, filepath.Join(codex, "history.jsonl"), []string{codexTokenCount("2026-09-01T10:00:05.000Z", 9, 0, 9, 18)})

	s, err := ScanRange(claude, Range{CodexRoot: codex})
	if err != nil {
		t.Fatal(err)
	}
	if !s.Available || len(s.Agents) != 1 || s.Agents[0].Agent != AgentCodex || s.Agents[0].Turns != 2 {
		t.Fatalf("agents = %+v", s.Agents)
	}
	// Cached tokens are inside Codex's input_tokens: (1000-400)+(1500-600).
	if s.Total.Input != 1500 || s.Total.CacheRead != 1000 || s.Total.Output != 300 {
		t.Fatalf("total = %+v", s.Total)
	}
	if len(s.Projects) != 1 || s.Projects[0].Path != "/work/app" || s.Projects[0].Sessions != 1 {
		t.Fatalf("projects = %+v", s.Projects)
	}
	if s.Projects[0].ActiveSeconds != 175 {
		t.Fatalf("active = %d", s.Projects[0].ActiveSeconds)
	}
	if len(s.Models) != 1 || s.Models[0].Model != "gpt-5-codex" {
		t.Fatalf("models = %+v", s.Models)
	}
}

const geminiTurn = `{"id":"m2","timestamp":"2026-09-01T10:01:00.000Z","type":"gemini","content":"secret answer","model":"gemini-2.5-pro",` +
	`"tokens":{"input":500,"output":50,"cached":200,"thoughts":30,"tool":10,"total":590}}`

func TestScanGeminiSessions(t *testing.T) {
	claude := t.TempDir()
	home := t.TempDir()
	tmp := filepath.Join(home, "tmp")

	// Current format: JSONL, project path in the .project_root marker.
	app := filepath.Join(tmp, "app")
	writeFile(t, filepath.Join(app, "chats", "session-2026-09-01T10-00-g1g1g1g1.jsonl"), []string{
		`{"sessionId":"g1","projectHash":"abc","startTime":"2026-09-01T10:00:00.000Z","lastUpdated":"2026-09-01T10:00:00.000Z","kind":"main"}`,
		`{"id":"m1","timestamp":"2026-09-01T10:00:30.000Z","type":"user","content":"secret prompt"}`,
		geminiTurn,
		`{"$set":{"lastUpdated":"2026-09-01T10:01:00.000Z"}}`,
		geminiTurn, // written again by a patch: counted once
		`{"$rewindTo":"m1"}`,
	})
	if err := os.WriteFile(filepath.Join(app, ".project_root"), []byte("/work/app\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Legacy format: one JSON object, project resolved through projects.json.
	writeFile(t, filepath.Join(tmp, "old-site", "chats", "session-2026-08-30T09-00-legacy.json"), []string{
		`{"sessionId":"g0","projectHash":"def","startTime":"2026-08-30T09:00:00.000Z","lastUpdated":"2026-08-30T09:05:00.000Z","messages":[` +
			`{"id":"a","timestamp":"2026-08-30T09:00:00.000Z","type":"user","content":"x"},` +
			`{"id":"b","timestamp":"2026-08-30T09:01:00.000Z","type":"gemini","content":"y","model":"gemini-2.5-flash","tokens":{"input":100,"output":20,"cached":0,"total":120}}]}`,
	})
	writeFile(t, filepath.Join(home, "projects.json"), []string{`{"projects":{"/work/old-site":"old-site"}}`})
	// Other files in the temp dir are not sessions.
	writeFile(t, filepath.Join(app, "logs.json"), []string{`[{"tokens":{"input":999}}]`})

	s, err := ScanRange(claude, Range{GeminiRoot: tmp})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Agents) != 1 || s.Agents[0].Agent != AgentGemini || s.Agents[0].Turns != 2 {
		t.Fatalf("agents = %+v", s.Agents)
	}
	byPath := map[string]ProjectUsage{}
	for _, p := range s.Projects {
		byPath[p.Path] = p
	}
	// input 500 includes 200 cached; tool tokens are input, thoughts output.
	if got := byPath["/work/app"].Tokens; got.Input != 310 || got.CacheRead != 200 || got.Output != 80 {
		t.Fatalf("/work/app = %+v", got)
	}
	if got := byPath["/work/old-site"].Tokens; got.Input != 100 || got.Output != 20 {
		t.Fatalf("legacy project = %+v (projects %v)", got, s.Projects)
	}
}

func TestScanMergesAgentsPerProject(t *testing.T) {
	claude := t.TempDir()
	codex := t.TempDir()
	writeFile(t, filepath.Join(claude, "-work-app", "c1.jsonl"), []string{
		assistantLine("/work/app", "sess-1", "2026-09-01T10:02:00.000Z", 10, 5, 0, 0),
	})
	writeCodexSession(t, codex, "sessions")

	s, err := ScanRange(claude, Range{CodexRoot: codex, Since: time.Time{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Projects) != 1 || len(s.Agents) != 2 {
		t.Fatalf("projects = %d agents = %+v", len(s.Projects), s.Agents)
	}
	// Same session id in two agents is still two sessions.
	if s.Projects[0].Sessions != 2 {
		t.Fatalf("sessions = %d", s.Projects[0].Sessions)
	}
	// One stretch of work across both agents: 10:00:05 -> 10:03:00.
	if s.Projects[0].ActiveSeconds != 175 {
		t.Fatalf("active = %d", s.Projects[0].ActiveSeconds)
	}
	if !Detected(claude, Range{}) || !Detected(t.TempDir(), Range{CodexRoot: codex}) || Detected(t.TempDir(), Range{}) {
		t.Fatal("Detected is wrong")
	}
}
