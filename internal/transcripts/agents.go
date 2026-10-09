package transcripts

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"karina/internal/pricing"
)

// Other coding agents keep local session logs too. Reading them puts their
// usage and working time next to Claude Code's, per project and per client.
//
// The same privacy contract applies: only timestamps, the working directory,
// session/message ids, the model name and the numeric token counts are
// decoded. Prompts, answers and tool output never leave the JSON decoder.
//
// FORMATS (taken from each tool's own source code, 2026-10-08 — neither tool
// was installed on the machine this was written on, so they are covered by
// fixtures built from those definitions, not by real session files):
//
//   - Codex CLI (openai/codex, codex-rs/protocol): $CODEX_HOME (default
//     ~/.codex) holds sessions/ and archived_sessions/ with rollout *.jsonl
//     files. Each line is {"timestamp","type","payload"}. "session_meta"
//     carries id and cwd; "turn_context" carries cwd and model; "event_msg"
//     with payload.type "token_count" carries payload.info with the
//     cumulative total_token_usage and the last_token_usage of the request.
//     input_tokens INCLUDES cached_input_tokens; output_tokens includes the
//     reasoning tokens.
//   - Gemini CLI (google-gemini/gemini-cli, chatRecordingService): the global
//     dir ~/.gemini/tmp/<project>/chats/ holds session-*.jsonl (older
//     versions: one session-*.json object). Records of type "gemini" carry
//     model and tokens {input, output, cached, thoughts, tool, total}. The
//     project path is in <project>/.project_root (or the projects.json
//     registry next to tmp/).
const (
	AgentClaude = "claude"
	AgentCodex  = "codex"
	AgentGemini = "gemini"
)

// AgentUsage is the aggregate for one coding agent.
type AgentUsage struct {
	Agent string `json:"agent"`
	Name  string `json:"name"`
	// Turns is the number of model responses counted.
	Turns   int     `json:"turns"`
	Tokens  Tokens  `json:"tokens"`
	CostUSD float64 `json:"cost_usd"`
	// UnpricedTokens are tokens of models without a known price: they are in
	// Tokens but not in CostUSD.
	UnpricedTokens int64 `json:"unpriced_tokens"`
}

func agentName(agent string) string {
	switch agent {
	case AgentClaude:
		return "Claude Code"
	case AgentCodex:
		return "Codex CLI"
	case AgentGemini:
		return "Gemini CLI"
	case AgentOpenCode:
		return "OpenCode"
	}
	return agent
}

func sortedAgents(agents map[string]*AgentUsage) []AgentUsage {
	out := make([]AgentUsage, 0, len(agents))
	for _, a := range agents {
		out = append(out, *a)
	}
	sort.Slice(out, func(i, j int) bool {
		ti, tj := out[i].Tokens.Total(), out[j].Tokens.Total()
		if ti != tj {
			return ti > tj
		}
		return out[i].Agent < out[j].Agent
	})
	return out
}

// DefaultCodexRoot returns $CODEX_HOME, or ~/.codex.
func DefaultCodexRoot() (string, error) {
	if home := strings.TrimSpace(os.Getenv("CODEX_HOME")); home != "" {
		return home, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex"), nil
}

// DefaultGeminiRoot returns ~/.gemini/tmp, where Gemini CLI keeps one folder
// per project.
func DefaultGeminiRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".gemini", "tmp"), nil
}

// source is one directory of session logs and how to read it.
type source struct {
	root  string
	match func(path string) bool
	parse func(path string) []turn
}

// existingSources returns the log directories that exist on this machine.
func existingSources(claudeRoot string, r Range) ([]source, error) {
	isDir := func(p string) (bool, error) {
		if p == "" {
			return false, nil
		}
		info, err := os.Stat(p)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return false, nil
			}
			return false, err
		}
		return info.IsDir(), nil
	}
	jsonl := func(path string) bool { return strings.HasSuffix(path, ".jsonl") }

	var out []source
	ok, err := isDir(claudeRoot)
	if err != nil {
		return nil, err
	}
	if ok {
		out = append(out, source{root: claudeRoot, match: jsonl, parse: parseClaude})
	}
	// Codex: only its two session folders; the rest of ~/.codex is config.
	for _, sub := range []string{"sessions", "archived_sessions"} {
		if r.CodexRoot == "" {
			break
		}
		dir := filepath.Join(r.CodexRoot, sub)
		if ok, _ := isDir(dir); ok {
			out = append(out, source{root: dir, match: jsonl, parse: parseCodex})
		}
	}
	if ok, _ := isDir(r.GeminiRoot); ok {
		out = append(out, source{root: r.GeminiRoot, match: isGeminiChat, parse: parseGemini})
	}
	return out, nil
}

// newScanner returns a line scanner sized for session logs, whose lines can
// carry multi-megabyte payloads.
func newScanner(f *os.File) *bufio.Scanner {
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 0, 64*1024), 32*1024*1024)
	return s
}

// price fills in the estimated cost of a turn. cacheWrite1h is the part of
// tokens.CacheCreation written with the one-hour lifetime.
func (t *turn) price(cacheWrite1h int64) {
	p, ok := pricing.For(t.model)
	if !ok {
		return
	}
	if cacheWrite1h > t.tokens.CacheCreation {
		cacheWrite1h = t.tokens.CacheCreation
	}
	t.priced = true
	t.cost = p.Cost(t.tokens.Input, t.tokens.Output,
		t.tokens.CacheCreation-cacheWrite1h, cacheWrite1h, t.tokens.CacheRead)
}

// ---------------------------------------------------------------- Codex CLI

type codexUsage struct {
	Input     int64 `json:"input_tokens"`
	Cached    int64 `json:"cached_input_tokens"`
	Output    int64 `json:"output_tokens"`
	Reasoning int64 `json:"reasoning_output_tokens"`
	Total     int64 `json:"total_tokens"`
}

func (u codexUsage) minus(o codexUsage) codexUsage {
	return codexUsage{
		Input:     u.Input - o.Input,
		Cached:    u.Cached - o.Cached,
		Output:    u.Output - o.Output,
		Reasoning: u.Reasoning - o.Reasoning,
		Total:     u.Total - o.Total,
	}
}

func (u codexUsage) valid() bool {
	return u.Input >= 0 && u.Cached >= 0 && u.Output >= 0 && u.Input+u.Output > 0
}

type codexLine struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Payload   struct {
		// session_meta
		ID string `json:"id"`
		// session_meta and turn_context
		CWD string `json:"cwd"`
		// turn_context
		Model string `json:"model"`
		// event_msg
		Type string `json:"type"`
		Info *struct {
			Total *codexUsage `json:"total_token_usage"`
			Last  *codexUsage `json:"last_token_usage"`
		} `json:"info"`
	} `json:"payload"`
}

var (
	codexTokenMark   = []byte(`"token_count"`)
	codexContextMark = []byte(`"turn_context"`)
	codexMetaMark    = []byte(`"session_meta"`)
)

// parseCodex reads one Codex CLI rollout file. Usage arrives as cumulative
// snapshots, so each turn is the growth since the previous snapshot; that
// also makes repeated snapshots harmless.
func parseCodex(path string) []turn {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var (
		turns   []turn
		session string
		cwd     string
		model   string
		prev    *codexUsage
	)
	scanner := newScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if !bytes.Contains(line, codexTokenMark) && !bytes.Contains(line, codexContextMark) && !bytes.Contains(line, codexMetaMark) {
			continue
		}
		var rec codexLine
		if json.Unmarshal(line, &rec) != nil {
			continue
		}
		switch rec.Type {
		case "session_meta":
			if rec.Payload.ID != "" {
				session = rec.Payload.ID
			}
			if rec.Payload.CWD != "" && cwd == "" {
				cwd = rec.Payload.CWD
			}
		case "turn_context":
			if rec.Payload.CWD != "" {
				cwd = rec.Payload.CWD
			}
			if rec.Payload.Model != "" {
				model = rec.Payload.Model
			}
		case "event_msg":
			if rec.Payload.Type != "token_count" || rec.Payload.Info == nil || rec.Payload.Info.Total == nil {
				continue
			}
			total := *rec.Payload.Info.Total
			var delta codexUsage
			switch {
			case prev != nil && total == *prev:
				continue // same snapshot again
			case prev != nil && total.minus(*prev).valid():
				delta = total.minus(*prev)
			case rec.Payload.Info.Last != nil && rec.Payload.Info.Last.valid():
				// First snapshot of the file, or the totals went backwards
				// (a forked session starts from its parent's totals): only
				// the last request is known to belong here.
				delta = *rec.Payload.Info.Last
			default:
				prev = &total
				continue
			}
			prev = &total

			ts, err := parseTimestamp(rec.Timestamp)
			if err != nil {
				continue
			}
			cached := delta.Cached
			if cached > delta.Input {
				cached = delta.Input
			}
			t := turn{
				agent:   AgentCodex,
				at:      ts,
				cwd:     cwd,
				session: session,
				model:   model,
				// Codex counts cached tokens inside input_tokens.
				tokens: Tokens{Input: delta.Input - cached, Output: delta.Output, CacheRead: cached},
				// sessions/ and archived_sessions/ can hold the same rollout.
				key: "codex|" + session + "|" + rec.Timestamp + "|" + strconv.FormatInt(total.Total, 10),
			}
			t.price(0)
			turns = append(turns, t)
		}
	}
	return turns
}

// --------------------------------------------------------------- Gemini CLI

type geminiMessage struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Model     string `json:"model"`
	Tokens    *struct {
		Input    int64 `json:"input"`
		Output   int64 `json:"output"`
		Cached   int64 `json:"cached"`
		Thoughts int64 `json:"thoughts"`
		Tool     int64 `json:"tool"`
	} `json:"tokens"`
}

// geminiRecord is one JSONL line (or the whole legacy .json file): either a
// message, the session header, or a "$set" metadata update. Legacy
// checkpoints and files carry the full message list.
type geminiRecord struct {
	geminiMessage
	SessionID string          `json:"sessionId"`
	Messages  []geminiMessage `json:"messages"`
	Set       *struct {
		SessionID string          `json:"sessionId"`
		Messages  []geminiMessage `json:"messages"`
	} `json:"$set"`
}

// isGeminiChat matches the session files under <project>/chats/.
func isGeminiChat(path string) bool {
	if !strings.HasSuffix(path, ".jsonl") && !strings.HasSuffix(path, ".json") {
		return false
	}
	return strings.Contains(filepath.ToSlash(path), "/chats/")
}

// parseGemini reads one Gemini CLI session file.
func parseGemini(path string) []turn {
	cwd := geminiProjectPath(path)
	session := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(path), ".jsonl"), ".json")

	var turns []turn
	byID := map[string]int{}
	add := func(m geminiMessage) {
		if m.Type != "gemini" || m.Tokens == nil {
			return
		}
		ts, err := parseTimestamp(m.Timestamp)
		if err != nil {
			return
		}
		tk := m.Tokens
		cached := tk.Cached
		if cached > tk.Input {
			cached = tk.Input
		}
		t := turn{
			agent:   AgentGemini,
			at:      ts,
			cwd:     cwd,
			session: session,
			model:   m.Model,
			// Gemini counts cached tokens inside the prompt; tool-use prompt
			// tokens are input and thinking tokens are billed as output.
			tokens: Tokens{
				Input:     tk.Input - cached + tk.Tool,
				Output:    tk.Output + tk.Thoughts,
				CacheRead: cached,
			},
		}
		if t.tokens.Total() <= 0 {
			return
		}
		t.price(0)
		if m.ID != "" {
			t.key = "gemini|" + session + "|" + m.ID
			// A message can be written again (patches, checkpoints): keep
			// the most complete count.
			if i, dup := byID[m.ID]; dup {
				if t.tokens.Total() > turns[i].tokens.Total() {
					turns[i] = t
				}
				return
			}
			byID[m.ID] = len(turns)
		}
		turns = append(turns, t)
	}
	handle := func(rec *geminiRecord) {
		if rec.SessionID != "" {
			session = rec.SessionID
		}
		if rec.Set != nil {
			if rec.Set.SessionID != "" {
				session = rec.Set.SessionID
			}
			for _, m := range rec.Set.Messages {
				add(m)
			}
		}
		for _, m := range rec.Messages {
			add(m)
		}
		add(rec.geminiMessage)
	}

	if strings.HasSuffix(path, ".json") {
		// Legacy format: the whole conversation as one JSON object.
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var rec geminiRecord
		if json.Unmarshal(data, &rec) == nil {
			handle(&rec)
		}
		return turns
	}

	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	scanner := newScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		// Only the session header and records with token counts matter.
		if !bytes.Contains(line, []byte(`"tokens"`)) && !bytes.Contains(line, []byte(`"sessionId"`)) {
			continue
		}
		var rec geminiRecord
		if json.Unmarshal(line, &rec) == nil {
			handle(&rec)
		}
	}
	return turns
}

// geminiProjectPath finds the project a session file belongs to. Gemini CLI
// names the folder with a short id and writes the real path in a
// .project_root marker; the projects.json registry maps path -> id too.
func geminiProjectPath(sessionFile string) string {
	// <tmp>/<project>/chats/[<parent session>/]file
	dir := filepath.Dir(sessionFile)
	for filepath.Base(dir) != "chats" {
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
	projectDir := filepath.Dir(dir)
	if data, err := os.ReadFile(filepath.Join(projectDir, ".project_root")); err == nil {
		if p := strings.TrimSpace(string(data)); p != "" {
			return p
		}
	}
	id := filepath.Base(projectDir)
	registry := filepath.Join(filepath.Dir(filepath.Dir(projectDir)), "projects.json")
	if data, err := os.ReadFile(registry); err == nil {
		var reg struct {
			Projects map[string]string `json:"projects"`
		}
		if json.Unmarshal(data, &reg) == nil {
			for path, slug := range reg.Projects {
				if slug == id {
					return path
				}
			}
		}
	}
	// Unknown project: keep the sessions together under the folder id.
	return "gemini:" + id
}
