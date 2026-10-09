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
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"karina/internal/pricing"
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
	RequestID string `json:"requestId"`
	Message   *struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage *struct {
			InputTokens              int64 `json:"input_tokens"`
			OutputTokens             int64 `json:"output_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
			// CacheCreation splits the cache writes by lifetime; the two
			// lifetimes are priced differently.
			CacheCreation *struct {
				Ephemeral1h int64 `json:"ephemeral_1h_input_tokens"`
			} `json:"cache_creation"`
		} `json:"usage"`
	} `json:"message"`
}

// Tokens is a token breakdown. It is the measured data; the dollar figures
// reported next to it are an estimate at API list prices (see
// internal/pricing) and are always labelled as such.
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
	// CostUSD is the estimated cost at API list prices.
	CostUSD float64 `json:"cost_usd"`
	// ActiveSeconds is the estimated time worked on the project (see
	// activeSeconds).
	ActiveSeconds int64 `json:"active_seconds"`
	// BillableUSD is ActiveSeconds at the client's hourly rate (0 when the
	// client has none). Filled by ApplyRates.
	BillableUSD float64 `json:"billable_usd"`
	// Client is the client/label the user assigned to this project ("" when
	// none). It is filled in by the caller, not read from the transcripts.
	Client string `json:"client"`
	// ClientInherited is true when Client comes from a folder rule rather
	// than from an assignment made on this project.
	ClientInherited bool `json:"client_inherited"`
	// Models is the per-model split of this project, sorted by total desc.
	Models []ModelUsage `json:"models"`
}

// ClientUsage is the aggregate of every project assigned to one client.
// Name is "" for the projects that have no client.
type ClientUsage struct {
	Name     string  `json:"name"`
	Projects int     `json:"projects"`
	Sessions int     `json:"sessions"`
	Tokens   Tokens  `json:"tokens"`
	CostUSD  float64 `json:"cost_usd"`
	// ActiveSeconds is the sum of the active time of its projects.
	ActiveSeconds int64 `json:"active_seconds"`
	// HourlyRateUSD is what the user charges this client per hour (0 = not
	// set). BillableUSD is ActiveSeconds at that rate and MarginUSD what is
	// left after the estimated AI cost. All three are filled by ApplyRates.
	HourlyRateUSD float64 `json:"hourly_rate_usd"`
	BillableUSD   float64 `json:"billable_usd"`
	MarginUSD     float64 `json:"margin_usd"`
}

// ApplyRates turns hours into money: for every client with an hourly rate it
// computes what the worked time is worth and the margin left after the AI
// cost. Call it after AssignClients.
func (s *Summary) ApplyRates(rates map[string]float64) {
	s.BillableUSD, s.MarginUSD = 0, 0
	for i := range s.Projects {
		p := &s.Projects[i]
		p.BillableUSD = 0
		if rate := rates[p.Client]; p.Client != "" && rate > 0 {
			p.BillableUSD = float64(p.ActiveSeconds) / 3600 * rate
		}
	}
	for i := range s.Clients {
		c := &s.Clients[i]
		c.HourlyRateUSD, c.BillableUSD, c.MarginUSD = 0, 0, 0
		rate := rates[c.Name]
		if c.Name == "" || rate <= 0 {
			continue
		}
		c.HourlyRateUSD = rate
		c.BillableUSD = float64(c.ActiveSeconds) / 3600 * rate
		c.MarginUSD = c.BillableUSD - c.CostUSD
		s.BillableUSD += c.BillableUSD
		s.MarginUSD += c.MarginUSD
	}
}

// ModelUsage is the aggregate for one model id exactly as Claude Code
// recorded it (e.g. "claude-sonnet-5"); Karina does not rename it.
type ModelUsage struct {
	Model string `json:"model"`
	// Turns is the number of assistant responses counted for this model.
	Turns  int    `json:"turns"`
	Tokens Tokens `json:"tokens"`
	// CostUSD is the estimated cost at API list prices; Priced is false when
	// Karina has no price for this model (then CostUSD is 0, not a guess).
	CostUSD float64 `json:"cost_usd"`
	Priced  bool    `json:"priced"`
}

// DayUsage is the aggregate for one calendar day (local time), across every
// project — the shape a simple bar/line chart wants.
type DayUsage struct {
	Date    string  `json:"date"` // YYYY-MM-DD, local time
	Tokens  Tokens  `json:"tokens"`
	CostUSD float64 `json:"cost_usd"`
}

// Summary is the full report for a given time span.
type Summary struct {
	// Available is false when ~/.claude/projects doesn't exist at all on
	// this machine (Claude Code never run here), as opposed to existing
	// but having nothing in the requested span.
	Available bool           `json:"available"`
	Projects  []ProjectUsage `json:"projects"` // sorted by Tokens.Total desc
	Days      []DayUsage     `json:"days"`     // sorted by date asc
	Models    []ModelUsage   `json:"models"`   // sorted by Tokens.Total desc
	Agents    []AgentUsage   `json:"agents"`   // sorted by Tokens.Total desc
	Total     Tokens         `json:"total"`
	// CostUSD is the estimated cost of Total at API list prices (as of
	// PricesAsOf). UnpricedTokens counts the tokens of models Karina has no
	// price for: they are in Total but not in CostUSD.
	CostUSD        float64 `json:"cost_usd"`
	UnpricedTokens int64   `json:"unpriced_tokens"`
	PricesAsOf     string  `json:"prices_as_of"`
	// ActiveSeconds is the active time summed over projects (work done on two
	// projects at once counts for both). IdleGapMinutes is the pause that
	// ends a stretch of work.
	ActiveSeconds  int64 `json:"active_seconds"`
	IdleGapMinutes int   `json:"idle_gap_minutes"`
	// BillableUSD and MarginUSD add up the clients that have an hourly rate
	// (see ApplyRates).
	BillableUSD float64 `json:"billable_usd"`
	MarginUSD   float64 `json:"margin_usd"`
	// Clients and ClientNames are filled in by AssignClients.
	Clients     []ClientUsage `json:"clients"`      // sorted by CostUSD desc
	ClientNames []string      `json:"client_names"` // every known client, sorted
	// ClientFolders are the folder rules in effect, sorted by path.
	ClientFolders []FolderRule `json:"client_folders"`
}

// FolderRule assigns every project under a folder to a client. Projects is
// how many projects of the current span the rule applies to.
type FolderRule struct {
	Path     string `json:"path"`
	Client   string `json:"client"`
	Projects int    `json:"projects"`
}

// normPath makes paths comparable: forward slashes, no trailing slash, and
// case-insensitive (Windows and the default macOS filesystem both are).
func normPath(p string) string {
	p = strings.ReplaceAll(p, `\`, "/")
	return strings.ToLower(strings.TrimRight(p, "/"))
}

// under reports whether project path p (normalized) is folder f or inside it.
func under(p, f string) bool {
	return f != "" && (p == f || strings.HasPrefix(p, f+"/"))
}

// AssignClients labels each project with its client and builds the
// per-client totals. byPath maps an exact project path to a client; byFolder
// maps a parent folder to a client for everything under it. An exact
// assignment wins over a folder, and the deepest matching folder wins over
// its parents.
func (s *Summary) AssignClients(byPath, byFolder map[string]string) {
	names := map[string]struct{}{}
	for _, name := range byPath {
		if name != "" {
			names[name] = struct{}{}
		}
	}
	rules := make([]FolderRule, 0, len(byFolder))
	for folder, name := range byFolder {
		if name == "" || normPath(folder) == "" {
			continue
		}
		names[name] = struct{}{}
		rules = append(rules, FolderRule{Path: folder, Client: name})
	}
	// Deepest folder first, so the first match is the most specific one.
	sort.Slice(rules, func(i, j int) bool {
		ni, nj := normPath(rules[i].Path), normPath(rules[j].Path)
		if len(ni) != len(nj) {
			return len(ni) > len(nj)
		}
		return ni < nj
	})
	exact := make(map[string]string, len(byPath))
	for path, name := range byPath {
		exact[normPath(path)] = name
	}
	s.ClientNames = make([]string, 0, len(names))
	for name := range names {
		s.ClientNames = append(s.ClientNames, name)
	}
	sort.Strings(s.ClientNames)

	clients := map[string]*ClientUsage{}
	for i := range s.Projects {
		p := &s.Projects[i]
		np := normPath(p.Path)
		p.Client, p.ClientInherited = exact[np], false
		if p.Client == "" {
			for r := range rules {
				if under(np, normPath(rules[r].Path)) {
					p.Client, p.ClientInherited = rules[r].Client, true
					rules[r].Projects++
					break
				}
			}
		}
		c, ok := clients[p.Client]
		if !ok {
			c = &ClientUsage{Name: p.Client}
			clients[p.Client] = c
		}
		c.Projects++
		c.Sessions += p.Sessions
		c.Tokens.add(p.Tokens)
		c.CostUSD += p.CostUSD
		c.ActiveSeconds += p.ActiveSeconds
	}
	s.Clients = make([]ClientUsage, 0, len(clients))
	for _, c := range clients {
		s.Clients = append(s.Clients, *c)
	}
	sort.Slice(s.Clients, func(i, j int) bool {
		if s.Clients[i].CostUSD != s.Clients[j].CostUSD {
			return s.Clients[i].CostUSD > s.Clients[j].CostUSD
		}
		return s.Clients[i].Name < s.Clients[j].Name
	})

	sort.Slice(rules, func(i, j int) bool { return normPath(rules[i].Path) < normPath(rules[j].Path) })
	s.ClientFolders = rules
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

// Detected reports whether a coding agent has been used on this machine:
// one of the log directories holds at least one session file. It stops at
// the first match, so it is cheap enough to call on startup.
func Detected(root string, r Range) bool {
	if fileExists(r.OpenCodeDB) {
		return true
	}
	sources, _ := existingSources(root, r)
	found := false
	for _, src := range sources {
		src := src
		_ = filepath.WalkDir(src.root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !d.IsDir() && src.match(path) {
				found = true
				return filepath.SkipAll
			}
			return nil
		})
		if found {
			break
		}
	}
	return found
}

// Scan walks every *.jsonl file under root (including the subagents/
// subdirectories Claude Code nests forked-agent transcripts in) and
// aggregates token usage for assistant turns at or after since.
func Scan(root string, since time.Time) (Summary, error) {
	return ScanRange(root, Range{Since: since})
}

// DefaultIdleGap is the pause after which work on a project is considered
// to have stopped when measuring active time.
const DefaultIdleGap = 10 * time.Minute

// Range selects what a scan covers and how active time is measured.
type Range struct {
	// Since is the inclusive start; Until the exclusive end (zero = no end).
	Since, Until time.Time
	// IdleGap is the longest pause between two responses that still counts
	// as working time (zero = DefaultIdleGap).
	IdleGap time.Duration
	// CodexRoot and GeminiRoot add the local logs of those coding agents to
	// the scan (see agents.go). Empty means that agent is not read.
	CodexRoot  string
	GeminiRoot string
	// OpenCodeDB adds OpenCode's SQLite database (see opencode.go).
	OpenCodeDB string
}

// ScanRange is Scan for an arbitrary time range. root is Claude Code's
// projects directory; r may add other agents' log directories.
func ScanRange(root string, r Range) (Summary, error) {
	since := r.Since
	idleGap := r.IdleGap
	if idleGap <= 0 {
		idleGap = DefaultIdleGap
	}
	sources, err := existingSources(root, r)
	if err != nil {
		return Summary{}, err
	}
	hasOpenCode := fileExists(r.OpenCodeDB)
	if len(sources) == 0 && !hasOpenCode {
		return Summary{Available: false}, nil
	}

	a := &aggregates{
		projects:      map[string]*ProjectUsage{},
		days:          map[string]*DayUsage{},
		sessions:      map[string]map[string]struct{}{},
		models:        map[string]*ModelUsage{},
		projectModels: map[string]map[string]*ModelUsage{},
		projectTimes:  map[string][]int64{},
		agents:        map[string]*AgentUsage{},
	}

	// A file last written before `since` cannot hold a turn inside the span,
	// so it is not even opened. That is what keeps "Hoy" fast on machines
	// with months of history.
	type job struct {
		path  string
		parse func(string) []turn
	}
	var jobs []job
	seen := map[string]struct{}{}
	for _, src := range sources {
		src := src
		walkErr := filepath.WalkDir(src.root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				// Skip unreadable entries rather than failing the whole scan.
				return nil
			}
			if d.IsDir() || !src.match(path) {
				return nil
			}
			seen[path] = struct{}{}
			if fi, err := d.Info(); err == nil && !fi.ModTime().Before(since) {
				jobs = append(jobs, job{path, src.parse})
			}
			return nil
		})
		if walkErr != nil {
			return Summary{}, walkErr
		}
	}
	cache.prune(seen)

	// Parse files in parallel; unchanged files come straight from the cache.
	perFile := make([][]turn, len(jobs))
	workers := runtime.NumCPU()
	if workers > 8 {
		workers = 8
	}
	var wg sync.WaitGroup
	next := make(chan int)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				perFile[i] = cache.turns(jobs[i].path, jobs[i].parse)
			}
		}()
	}
	for i := range jobs {
		next <- i
	}
	close(next)
	wg.Wait()
	if hasOpenCode {
		perFile = append(perFile, readOpenCode(r.OpenCodeDB, since))
	}

	// The same response can also sit in more than one file (resumed or
	// forked sessions copy their history), so it is counted once overall.
	counted := map[string]struct{}{}
	for _, turns := range perFile {
		for i := range turns {
			t := &turns[i]
			if t.at.Before(since) || (!r.Until.IsZero() && !t.at.Before(r.Until)) {
				continue
			}
			if t.key != "" {
				if _, dup := counted[t.key]; dup {
					continue
				}
				counted[t.key] = struct{}{}
			}
			a.add(t)
		}
	}

	summary := Summary{
		Available:      true,
		Total:          a.total,
		Models:         sortedModels(a.models),
		Agents:         sortedAgents(a.agents),
		CostUSD:        a.cost,
		UnpricedTokens: a.unpriced,
		PricesAsOf:     pricing.AsOf,
		IdleGapMinutes: int(idleGap / time.Minute),
	}
	for path, p := range a.projects {
		p.Sessions = len(a.sessions[path])
		p.Models = sortedModels(a.projectModels[path])
		p.ActiveSeconds = activeSeconds(a.projectTimes[path], int64(idleGap/time.Second))
		summary.ActiveSeconds += p.ActiveSeconds
		summary.Projects = append(summary.Projects, *p)
	}
	sort.Slice(summary.Projects, func(i, j int) bool {
		return summary.Projects[i].Tokens.Total() > summary.Projects[j].Tokens.Total()
	})
	for _, d := range a.days {
		summary.Days = append(summary.Days, *d)
	}
	sort.Slice(summary.Days, func(i, j int) bool { return summary.Days[i].Date < summary.Days[j].Date })

	return summary, nil
}

// activeSeconds estimates working time from the moments (unix seconds) a
// project got a response: consecutive responses no further apart than gap
// belong to the same stretch of work and the time between them counts; a
// longer silence is a break and does not. Sessions running in parallel on
// the same project are merged first, so their time is not counted twice.
func activeSeconds(times []int64, gap int64) int64 {
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	var total int64
	for i := 1; i < len(times); i++ {
		if d := times[i] - times[i-1]; d <= gap {
			total += d
		}
	}
	return total
}

// turn is one assistant response: the only data kept from a transcript.
type turn struct {
	at      time.Time
	cwd     string
	session string
	model   string
	tokens  Tokens
	// key identifies the API response (message id + request id). Claude Code
	// writes one line per content block of a response, each repeating the
	// same usage, so turns are de-duplicated on it. Empty when unknown.
	key string
	// cost is the estimated USD cost at API list prices; priced is false
	// when there is no price for the model.
	cost   float64
	priced bool
	// agent is the coding agent that produced the turn (AgentClaude, ...).
	agent string
}

// fileCache remembers the parsed turns of each transcript, keyed by path and
// invalidated by size/mtime. Transcripts are append-only and only the live
// session changes, so after the first scan almost nothing is re-read.
type fileCache struct {
	mu    sync.Mutex
	files map[string]cachedFile
}

type cachedFile struct {
	size  int64
	mod   time.Time
	turns []turn
}

var cache = &fileCache{files: map[string]cachedFile{}}

func (c *fileCache) turns(path string, parse func(string) []turn) []turn {
	fi, err := os.Stat(path)
	if err != nil {
		return nil
	}
	c.mu.Lock()
	hit, ok := c.files[path]
	c.mu.Unlock()
	if ok && hit.size == fi.Size() && hit.mod.Equal(fi.ModTime()) {
		return hit.turns
	}
	turns := parse(path)
	c.mu.Lock()
	c.files[path] = cachedFile{size: fi.Size(), mod: fi.ModTime(), turns: turns}
	c.mu.Unlock()
	return turns
}

// prune forgets files that no longer exist on disk.
func (c *fileCache) prune(seen map[string]struct{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for path := range c.files {
		if _, ok := seen[path]; !ok {
			delete(c.files, path)
		}
	}
}

var (
	assistantMark = []byte(`"assistant"`)
	usageMark     = []byte(`"usage"`)
)

// parseClaude reads one Claude Code transcript line by line and returns its
// assistant turns. Errors reading an individual file are swallowed
// (best-effort, like a corrupt or concurrently-rotated log line shouldn't
// fail the whole report) — the JSONL is written by Claude Code itself while
// sessions are live, so a torn last line is expected.
func parseClaude(path string) []turn {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Transcript lines can carry multi-megabyte "thinking" signatures; the
	// default 64KiB token buffer truncates those, so raise it well past
	// anything real (tool outputs included) but still bounded.
	scanner.Buffer(make([]byte, 0, 64*1024), 32*1024*1024)

	var turns []turn
	byKey := map[string]int{} // response key -> index in turns
	// cwd, session and model repeat on every line: share one copy per file.
	interned := map[string]string{}
	intern := func(s string) string {
		if v, ok := interned[s]; ok {
			return v
		}
		interned[s] = s
		return s
	}

	for scanner.Scan() {
		line := scanner.Bytes()
		// Most of a transcript is user turns and tool results, often huge.
		// Only lines that can be an assistant turn with usage are decoded.
		if !bytes.Contains(line, assistantMark) || !bytes.Contains(line, usageMark) {
			continue
		}
		var rec record
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		if rec.Type != "assistant" || rec.Message == nil || rec.Message.Usage == nil {
			continue
		}
		ts, err := parseTimestamp(rec.Timestamp)
		if err != nil {
			continue
		}
		u := rec.Message.Usage
		t := turn{
			agent:   AgentClaude,
			at:      ts,
			cwd:     intern(rec.CWD),
			session: intern(rec.SessionID),
			model:   intern(rec.Message.Model),
			tokens: Tokens{
				Input:         u.InputTokens,
				Output:        u.OutputTokens,
				CacheCreation: u.CacheCreationInputTokens,
				CacheRead:     u.CacheReadInputTokens,
			},
		}
		if price, ok := pricing.For(t.model); ok {
			var write1h int64
			if u.CacheCreation != nil {
				write1h = u.CacheCreation.Ephemeral1h
			}
			if write1h > t.tokens.CacheCreation {
				write1h = t.tokens.CacheCreation
			}
			t.priced = true
			t.cost = price.Cost(t.tokens.Input, t.tokens.Output,
				t.tokens.CacheCreation-write1h, write1h, t.tokens.CacheRead)
		}

		if rec.Message.ID != "" {
			t.key = rec.Message.ID + "|" + rec.RequestID
			// Repeated line of a response already seen: keep the most
			// complete usage (the last block carries the final output count).
			if i, dup := byKey[t.key]; dup {
				if t.tokens.Total() > turns[i].tokens.Total() {
					t.at = turns[i].at
					turns[i] = t
				}
				continue
			}
			byKey[t.key] = len(turns)
		}
		turns = append(turns, t)
	}
	return turns
}

// add folds one turn into the running aggregates.
func (a *aggregates) add(t *turn) {
	tk := t.tokens

	projectPath := t.cwd
	if projectPath == "" {
		projectPath = "desconocido"
	}
	p, ok := a.projects[projectPath]
	if !ok {
		p = &ProjectUsage{Path: projectPath, Label: filepath.Base(projectPath)}
		a.projects[projectPath] = p
		a.sessions[projectPath] = map[string]struct{}{}
		a.projectModels[projectPath] = map[string]*ModelUsage{}
	}
	p.Tokens.add(tk)
	p.CostUSD += t.cost
	a.projectTimes[projectPath] = append(a.projectTimes[projectPath], t.at.Unix())
	if t.session != "" {
		// Session ids are only unique within one agent.
		a.sessions[projectPath][t.agent+"|"+t.session] = struct{}{}
	}

	ag, ok := a.agents[t.agent]
	if !ok {
		ag = &AgentUsage{Agent: t.agent, Name: agentName(t.agent)}
		a.agents[t.agent] = ag
	}
	ag.Turns++
	ag.Tokens.add(tk)
	ag.CostUSD += t.cost
	if !t.priced {
		ag.UnpricedTokens += tk.Total()
	}

	dayKey := t.at.Local().Format("2006-01-02")
	day, ok := a.days[dayKey]
	if !ok {
		day = &DayUsage{Date: dayKey}
		a.days[dayKey] = day
	}
	day.Tokens.add(tk)
	day.CostUSD += t.cost

	model := t.model
	if model == "" {
		model = "desconocido"
	}
	addModel(a.models, model, t)
	addModel(a.projectModels[projectPath], model, t)

	a.total.add(tk)
	a.cost += t.cost
	if !t.priced {
		a.unpriced += tk.Total()
	}
}

// aggregates is the running state of one Scan.
type aggregates struct {
	projects      map[string]*ProjectUsage
	days          map[string]*DayUsage
	sessions      map[string]map[string]struct{} // project path -> set of session ids
	models        map[string]*ModelUsage
	projectModels map[string]map[string]*ModelUsage // project path -> model -> usage
	projectTimes  map[string][]int64                // project path -> response times (unix s)
	agents        map[string]*AgentUsage            // agent id -> usage
	total         Tokens
	cost          float64
	unpriced      int64 // tokens of models without a known price
}

func addModel(models map[string]*ModelUsage, model string, t *turn) {
	m, ok := models[model]
	if !ok {
		m = &ModelUsage{Model: model, Priced: t.priced}
		models[model] = m
	}
	m.Turns++
	m.Tokens.add(t.tokens)
	m.CostUSD += t.cost
}

// sortedModels returns the models that actually consumed tokens, biggest
// first. Entries with no tokens (e.g. Claude Code's "<synthetic>" local
// messages) are dropped: they are not model usage.
func sortedModels(models map[string]*ModelUsage) []ModelUsage {
	out := make([]ModelUsage, 0, len(models))
	for _, m := range models {
		if m.Tokens.Total() > 0 {
			out = append(out, *m)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		ti, tj := out[i].Tokens.Total(), out[j].Tokens.Total()
		if ti != tj {
			return ti > tj
		}
		return out[i].Model < out[j].Model
	})
	return out
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
