// Package transcripts reads local Claude Code session transcripts
// (~/.claude/projects/**/*.jsonl) to report *real* token usage per project
// and per day — the one number none of the providers' APIs can give Karina
// for a normal (non-Admin) key or for the Claude subscription: what a
// specific project actually cost you.
//
// Privacy contract, mirroring the one credentials.go documents for API
// keys: this package only ever reads timestamp, cwd, session id, model and
// the numeric usage block from each JSONL record. It never parses,
// stores or exposes message content, thinking blocks, or tool
// inputs/outputs — those fields are simply absent from the struct these
// records are decoded into, so they never make it past the JSON decoder.
package transcripts

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// record is the narrow shape pulled out of each JSONL line. Any other
// field present in the real record (message content, thinking, tool
// inputs/outputs, etc.) is simply not represented here and is discarded by
// the JSON decoder — it never reaches Go memory as structured data.
type record struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	CWD       string `json:"cwd"`
	SessionID string `json:"sessionId"`
	Message   *struct {
		Model string `json:"model"`
		Usage *struct {
			InputTokens              int64 `json:"input_tokens"`
			OutputTokens             int64 `json:"output_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// Tokens is a token breakdown, kept separate from dollar cost: cache
// creation/read tokens are priced very differently per model and Karina
// has no reliable, current price list to convert them — showing the raw
// numbers stays honest to the "real data, never invented" rule.
type Tokens struct {
	Input         int64 `json:"input_tokens"`
	Output        int64 `json:"output_tokens"`
	CacheCreation int64 `json:"cache_creation_tokens"`
	CacheRead     int64 `json:"cache_read_tokens"`
}

// Total sums every bucket. Useful for sorting/ranking; not a token price.
func (t Tokens) Total() int64 {
	return t.Input + t.Output + t.CacheCreation + t.CacheRead
}

func (t *Tokens) add(o Tokens) {
	t.Input += o.Input
	t.Output += o.Output
	t.CacheCreation += o.CacheCreation
	t.CacheRead += o.CacheRead
}

// ProjectUsage is the aggregate for one working directory (one local repo
// or folder Claude Code was run from).
type ProjectUsage struct {
	Path     string `json:"path"`
	Label    string `json:"label"`
	Sessions int    `json:"sessions"`
	Tokens   Tokens `json:"tokens"`
}

// DayUsage is the aggregate for one calendar day (local time), across every
// project — the shape a simple bar/line chart wants.
type DayUsage struct {
	Date   string `json:"date"` // YYYY-MM-DD, local time
	Tokens Tokens `json:"tokens"`
}

// Summary is the full report for a given time span.
type Summary struct {
	// Available is false when ~/.claude/projects doesn't exist at all on
	// this machine (Claude Code never run here), as opposed to existing
	// but having nothing in the requested span.
	Available bool           `json:"available"`
	Projects  []ProjectUsage `json:"projects"` // sorted by Tokens.Total desc
	Days      []DayUsage     `json:"days"`     // sorted by date asc
	Total     Tokens         `json:"total"`
}

// DefaultRoot returns ~/.claude/projects, the directory Claude Code itself
// writes transcripts to.
func DefaultRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "projects"), nil
}

// Scan walks every *.jsonl file under root (including the subagents/
// subdirectories Claude Code nests forked-agent transcripts in) and
// aggregates token usage for assistant turns at or after since.
func Scan(root string, since time.Time) (Summary, error) {
	info, err := os.Stat(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Summary{Available: false}, nil
		}
		return Summary{}, err
	}
	if !info.IsDir() {
		return Summary{Available: false}, nil
	}

	projects := map[string]*ProjectUsage{}
	days := map[string]*DayUsage{}
	sessions := map[string]map[string]struct{}{} // project path -> set of session ids
	var total Tokens

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Skip unreadable entries rather than failing the whole scan.
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		scanFile(path, since, projects, days, sessions, &total)
		return nil
	})
	if walkErr != nil {
		return Summary{}, walkErr
	}

	summary := Summary{Available: true, Total: total}
	for path, p := range projects {
		p.Sessions = len(sessions[path])
		summary.Projects = append(summary.Projects, *p)
	}
	sort.Slice(summary.Projects, func(i, j int) bool {
		return summary.Projects[i].Tokens.Total() > summary.Projects[j].Tokens.Total()
	})
	for _, d := range days {
		summary.Days = append(summary.Days, *d)
	}
	sort.Slice(summary.Days, func(i, j int) bool { return summary.Days[i].Date < summary.Days[j].Date })

	return summary, nil
}

// scanFile reads one transcript file line by line, folding matching
// records into the running aggregates. Errors reading an individual file
// are swallowed (best-effort, like a corrupt or concurrently-rotated log
// line shouldn't fail the whole report) — the JSONL is written by Claude
// Code itself while sessions are live, so a torn last line is expected.
func scanFile(
	path string,
	since time.Time,
	projects map[string]*ProjectUsage,
	days map[string]*DayUsage,
	sessions map[string]map[string]struct{},
	total *Tokens,
) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Transcript lines can carry multi-megabyte "thinking" signatures; the
	// default 64KiB token buffer truncates those, so raise it well past
	// anything real (tool outputs included) but still bounded.
	scanner.Buffer(make([]byte, 0, 64*1024), 32*1024*1024)

	for scanner.Scan() {
		var rec record
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		if rec.Type != "assistant" || rec.Message == nil || rec.Message.Usage == nil {
			continue
		}
		ts, err := parseTimestamp(rec.Timestamp)
		if err != nil || ts.Before(since) {
			continue
		}

		u := rec.Message.Usage
		tk := Tokens{
			Input:         u.InputTokens,
			Output:        u.OutputTokens,
			CacheCreation: u.CacheCreationInputTokens,
			CacheRead:     u.CacheReadInputTokens,
		}

		projectPath := rec.CWD
		if projectPath == "" {
			projectPath = "desconocido"
		}
		p, ok := projects[projectPath]
		if !ok {
			p = &ProjectUsage{Path: projectPath, Label: filepath.Base(projectPath)}
			projects[projectPath] = p
			sessions[projectPath] = map[string]struct{}{}
		}
		p.Tokens.add(tk)
		if rec.SessionID != "" {
			sessions[projectPath][rec.SessionID] = struct{}{}
		}

		dayKey := ts.Local().Format("2006-01-02")
		day, ok := days[dayKey]
		if !ok {
			day = &DayUsage{Date: dayKey}
			days[dayKey] = day
		}
		day.Tokens.add(tk)

		total.add(tk)
	}
}

func parseTimestamp(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, errors.New("empty timestamp")
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("unrecognized timestamp format")
}
