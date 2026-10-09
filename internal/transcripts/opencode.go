package transcripts

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (no cgo)
)

// OpenCode keeps its sessions in a SQLite database instead of log files.
//
// FORMAT (checked against a real opencode 1.18 database, 2026-10-08): the
// `message` table has one row per message with time_created in milliseconds
// and a JSON `data` column. Assistant messages carry
//
//	{"role":"assistant","modelID":…,"providerID":…,"path":{"cwd":…},
//	 "cost":…, "tokens":{"input":…,"output":…,"reasoning":…,
//	                      "cache":{"read":…,"write":…}}}
//
// tokens.input does NOT include the cache tokens, and reasoning is separate
// from output. `cost` is what OpenCode itself computed for the message, in
// USD; it is 0 when the model is free or covered by a subscription.
//
// Same privacy contract as the other readers: only ids, times, the working
// directory, the model and the numbers are selected — never the `part` table
// where the prompts and answers live. The database is opened read-only.
//
// Older OpenCode versions stored sessions as JSON files; those are not read.

// AgentOpenCode identifies turns produced by OpenCode.
const AgentOpenCode = "opencode"

// DefaultOpenCodeDB returns the path of OpenCode's database
// ($XDG_DATA_HOME or ~/.local/share, on every OS).
func DefaultOpenCodeDB() (string, error) {
	base := strings.TrimSpace(os.Getenv("XDG_DATA_HOME"))
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "opencode", "opencode.db"), nil
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

const openCodeQuery = `
SELECT m.id, m.session_id, m.time_created,
       COALESCE(json_extract(m.data, '$.modelID'), ''),
       COALESCE(json_extract(m.data, '$.path.cwd'), s.directory, ''),
       COALESCE(json_extract(m.data, '$.tokens.input'), 0),
       COALESCE(json_extract(m.data, '$.tokens.output'), 0),
       COALESCE(json_extract(m.data, '$.tokens.reasoning'), 0),
       COALESCE(json_extract(m.data, '$.tokens.cache.read'), 0),
       COALESCE(json_extract(m.data, '$.tokens.cache.write'), 0),
       COALESCE(json_extract(m.data, '$.cost'), 0)
FROM message m
LEFT JOIN session s ON s.id = m.session_id
WHERE m.time_created >= ?
  AND json_extract(m.data, '$.role') = 'assistant'`

// readOpenCode returns the assistant turns recorded at or after since. Any
// failure (database missing, locked, or a schema this code does not know)
// yields no turns rather than an error: OpenCode is optional extra data.
func readOpenCode(dbPath string, since time.Time) []turn {
	if !fileExists(dbPath) {
		return nil
	}
	dsn := "file:" + filepath.ToSlash(dbPath) + "?mode=ro&_pragma=busy_timeout(3000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil
	}
	defer db.Close()

	sinceMs := int64(0)
	if !since.IsZero() {
		sinceMs = since.UnixMilli()
	}
	rows, err := db.Query(openCodeQuery, sinceMs)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var turns []turn
	interned := map[string]string{}
	intern := func(s string) string {
		if v, ok := interned[s]; ok {
			return v
		}
		interned[s] = s
		return s
	}
	for rows.Next() {
		var (
			id, session, model, cwd                         string
			createdMs                                       int64
			input, output, reasoning, cacheRead, cacheWrite float64
			cost                                            float64
		)
		if err := rows.Scan(&id, &session, &createdMs, &model, &cwd,
			&input, &output, &reasoning, &cacheRead, &cacheWrite, &cost); err != nil {
			continue
		}
		t := turn{
			agent:   AgentOpenCode,
			at:      time.UnixMilli(createdMs),
			cwd:     intern(cwd),
			session: intern(session),
			model:   intern(model),
			tokens: Tokens{
				Input:         int64(input),
				Output:        int64(output + reasoning),
				CacheCreation: int64(cacheWrite),
				CacheRead:     int64(cacheRead),
			},
			key: "opencode|" + id,
		}
		if t.tokens.Total() <= 0 {
			continue // failed or empty request
		}
		if cost > 0 {
			// OpenCode already priced this message with its own model catalog.
			t.cost, t.priced = cost, true
		} else {
			t.price(0)
		}
		turns = append(turns, t)
	}
	return turns
}
