package stats

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// TokenStats tracks token usage and costs.
// All mutating methods are protected by a sync.RWMutex for concurrent access
// from the processLoopOutput goroutine (writer) and the BubbleTea TUI goroutine (reader).
type TokenStats struct {
	mu                  sync.RWMutex `json:"-"`
	InputTokens         int64        `json:"input_tokens"`
	OutputTokens        int64        `json:"output_tokens"`
	CacheCreationTokens int64        `json:"cache_creation_tokens"`
	CacheReadTokens     int64        `json:"cache_read_tokens"`
	TotalCostUSD        float64      `json:"total_cost"`
	TotalTokensCount    int64        `json:"total_tokens"`
	TotalElapsedNs      int64        `json:"elapsed_ns"`
}

// NewTokenStats creates a new empty TokenStats instance
func NewTokenStats() *TokenStats {
	return &TokenStats{
		InputTokens:         0,
		OutputTokens:        0,
		CacheCreationTokens: 0,
		CacheReadTokens:     0,
		TotalCostUSD:        0.0,
		TotalTokensCount:    0,
		TotalElapsedNs:      0,
	}
}

// AddUsage adds token usage counts to the stats
func (t *TokenStats) AddUsage(input, output, cacheCreation, cacheRead int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.InputTokens += input
	t.OutputTokens += output
	t.CacheCreationTokens += cacheCreation
	t.CacheReadTokens += cacheRead
	t.TotalTokensCount = t.InputTokens + t.OutputTokens + t.CacheCreationTokens + t.CacheReadTokens
}

// Pricing constants for Claude Sonnet 4 (per token)
const (
	PriceInputPerToken         = 3.00 / 1_000_000
	PriceOutputPerToken        = 15.00 / 1_000_000
	PriceCacheCreationPerToken = 3.75 / 1_000_000
	PriceCacheReadPerToken     = 0.30 / 1_000_000
)

// EstimateCostFromTokens computes estimated cost from token counts using hardcoded pricing
func EstimateCostFromTokens(input, output, cacheCreation, cacheRead int64) float64 {
	return float64(input)*PriceInputPerToken +
		float64(output)*PriceOutputPerToken +
		float64(cacheCreation)*PriceCacheCreationPerToken +
		float64(cacheRead)*PriceCacheReadPerToken
}

// AddCost adds cost to the total cost
func (t *TokenStats) AddCost(costUSD float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.TotalCostUSD += costUSD
}

// ReconcileCost replaces an estimated cost delta with the actual cost.
// It subtracts the estimated amount and adds the actual amount.
func (t *TokenStats) ReconcileCost(estimatedDelta, actualCost float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.TotalCostUSD -= estimatedDelta
	t.TotalCostUSD += actualCost
}

// TotalTokens returns the sum of all token counts
func (t *TokenStats) TotalTokens() int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.InputTokens + t.OutputTokens + t.CacheCreationTokens + t.CacheReadTokens
}

// SetTotalElapsedNs sets the total elapsed time in nanoseconds (thread-safe)
func (t *TokenStats) SetTotalElapsedNs(ns int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.TotalElapsedNs = ns
}

// Snapshot returns a consistent point-in-time copy of the stats for reading.
// The returned value has a fresh (zero) mutex and can be read without locking.
func (t *TokenStats) Snapshot() TokenStats {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return TokenStats{
		InputTokens:         t.InputTokens,
		OutputTokens:        t.OutputTokens,
		CacheCreationTokens: t.CacheCreationTokens,
		CacheReadTokens:     t.CacheReadTokens,
		TotalCostUSD:        t.TotalCostUSD,
		TotalTokensCount:    t.InputTokens + t.OutputTokens + t.CacheCreationTokens + t.CacheReadTokens,
		TotalElapsedNs:      t.TotalElapsedNs,
	}
}

// FormatTokens formats a token count into a human-readable string
// e.g., 36870000 → "36.87m", 300000 → "300k", 1500 → "1.5k", 42 → "42"
func FormatTokens(count int64) string {
	switch {
	case count >= 1_000_000:
		return fmt.Sprintf("%.2fm", float64(count)/1_000_000)
	case count >= 1_000:
		val := float64(count) / 1_000
		if val == float64(int64(val)) {
			return fmt.Sprintf("%dk", int64(val))
		}
		return fmt.Sprintf("%.1fk", val)
	default:
		return fmt.Sprintf("%d", count)
	}
}

// Save persists the stats to a JSON file at the given path
func (t *TokenStats) Save(path string) error {
	// Take a consistent snapshot under lock, then do I/O outside the lock
	t.mu.Lock()
	t.TotalTokensCount = t.InputTokens + t.OutputTokens + t.CacheCreationTokens + t.CacheReadTokens
	snap := TokenStats{
		InputTokens:         t.InputTokens,
		OutputTokens:        t.OutputTokens,
		CacheCreationTokens: t.CacheCreationTokens,
		CacheReadTokens:     t.CacheReadTokens,
		TotalCostUSD:        t.TotalCostUSD,
		TotalTokensCount:    t.TotalTokensCount,
		TotalElapsedNs:      t.TotalElapsedNs,
	}
	t.mu.Unlock()

	data, err := json.MarshalIndent(&snap, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// LoadTokenStats loads stats from a JSON file, returns empty stats if file doesn't exist
func LoadTokenStats(path string) (*TokenStats, error) {
	// If file doesn't exist, return empty stats
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return NewTokenStats(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var stats TokenStats
	if err := json.Unmarshal(data, &stats); err != nil {
		return nil, err
	}

	return &stats, nil
}

// GenerateSessionID returns a 6-char lowercase hex string from crypto/rand.
func GenerateSessionID() (string, error) {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// GetGitContext returns owner, repo, and branch from the current git context.
// Returns empty strings on any error (graceful fallback for non-git dirs).
func GetGitContext() (owner, repo, branch string) {
	// Get remote URL
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err == nil {
		remote := strings.TrimSpace(string(out))
		owner, repo = parseGitRemote(remote)
	}

	// Get branch
	out, err = exec.Command("git", "branch", "--show-current").Output()
	if err == nil {
		branch = strings.TrimSpace(string(out))
	}
	return
}

// parseGitRemote extracts owner/repo from an HTTPS or SSH git URL.
func parseGitRemote(remote string) (owner, repo string) {
	// Strip trailing .git
	remote = strings.TrimSuffix(remote, ".git")

	// SSH: git@github.com:owner/repo
	if strings.Contains(remote, ":") && strings.HasPrefix(remote, "git@") {
		parts := strings.SplitN(remote, ":", 2)
		if len(parts) == 2 {
			segments := strings.Split(parts[1], "/")
			if len(segments) >= 2 {
				return segments[len(segments)-2], segments[len(segments)-1]
			}
		}
		return "", ""
	}

	// HTTPS: https://github.com/owner/repo
	parts := strings.Split(remote, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2], parts[len(parts)-1]
	}
	return "", ""
}

// GetLatestCommitTitle returns the title of the latest git commit, or empty string on error.
func GetLatestCommitTitle() string {
	out, err := exec.Command("git", "log", "-1", "--format=%s").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// InitDB opens (or creates) the SQLite database at path, sets WAL mode,
// creates tables if needed, prunes old checkpoints, and returns the *sql.DB.
func InitDB(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("creating db directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// SQLite only supports one concurrent writer. Limiting to a single open
	// connection lets Go's pool serialize writes instead of hitting SQLITE_BUSY.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}

	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("setting busy timeout: %w", err)
	}

	const createCheckpoints = `CREATE TABLE IF NOT EXISTS checkpoints (
		id                  INTEGER PRIMARY KEY AUTOINCREMENT,
		loop_id             TEXT NOT NULL,
		session_id          TEXT NOT NULL,
		owner               TEXT,
		repo                TEXT,
		branch              TEXT,
		delta_cost          REAL NOT NULL,
		delta_input_tokens  INTEGER,
		delta_output_tokens INTEGER,
		delta_cache_creation INTEGER,
		delta_cache_read    INTEGER,
		timestamp           TEXT NOT NULL
	)`
	if _, err := db.Exec(createCheckpoints); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating checkpoints table: %w", err)
	}

	const createIndex = `CREATE INDEX IF NOT EXISTS idx_checkpoints_ts ON checkpoints(timestamp)`
	if _, err := db.Exec(createIndex); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating timestamp index: %w", err)
	}

	const createLoopStats = `CREATE TABLE IF NOT EXISTS loop_stats (
		loop_id               TEXT PRIMARY KEY,
		session_id            TEXT NOT NULL,
		owner                 TEXT,
		repo                  TEXT,
		branch                TEXT,
		description           TEXT,
		total_cost            REAL,
		input_tokens          INTEGER,
		output_tokens         INTEGER,
		cache_creation_tokens INTEGER,
		cache_read_tokens     INTEGER,
		total_tokens          INTEGER,
		start_time            TEXT,
		finish_time           TEXT
	)`
	if _, err := db.Exec(createLoopStats); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating loop_stats table: %w", err)
	}

	// Prune old checkpoint rows
	if _, err := db.Exec("DELETE FROM checkpoints WHERE timestamp < datetime('now', '-7 days')"); err != nil {
		db.Close()
		return nil, fmt.Errorf("pruning old checkpoints: %w", err)
	}

	return db, nil
}

// CheckpointParams holds parameters for a checkpoint row insert.
type CheckpointParams struct {
	LoopID            string
	SessionID         string
	Owner             string
	Repo              string
	Branch            string
	DeltaCost         float64
	DeltaInputTokens  int64
	DeltaOutputTokens int64
	DeltaCacheCreation int64
	DeltaCacheRead    int64
	Timestamp         string
}

// FlushCheckpoint inserts a checkpoint row into the database.
// No-op if db is nil.
func FlushCheckpoint(db *sql.DB, p CheckpointParams) error {
	if db == nil {
		return nil
	}
	_, err := db.Exec(
		`INSERT INTO checkpoints (loop_id, session_id, owner, repo, branch, delta_cost, delta_input_tokens, delta_output_tokens, delta_cache_creation, delta_cache_read, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.LoopID, p.SessionID, p.Owner, p.Repo, p.Branch,
		p.DeltaCost, p.DeltaInputTokens, p.DeltaOutputTokens, p.DeltaCacheCreation, p.DeltaCacheRead,
		p.Timestamp,
	)
	return err
}

// LoopStatsParams holds parameters for a loop_stats row insert.
type LoopStatsParams struct {
	LoopID              string
	SessionID           string
	Owner               string
	Repo                string
	Branch              string
	Description         string
	TotalCost           float64
	InputTokens         int64
	OutputTokens        int64
	CacheCreationTokens int64
	CacheReadTokens     int64
	TotalTokens         int64
	StartTime           string
	FinishTime          string
}

// WriteLoopStats inserts or replaces a loop_stats row.
// No-op if db is nil.
func WriteLoopStats(db *sql.DB, p LoopStatsParams) error {
	if db == nil {
		return nil
	}
	_, err := db.Exec(
		`INSERT OR REPLACE INTO loop_stats (loop_id, session_id, owner, repo, branch, description, total_cost, input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens, total_tokens, start_time, finish_time)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.LoopID, p.SessionID, p.Owner, p.Repo, p.Branch, p.Description,
		p.TotalCost, p.InputTokens, p.OutputTokens, p.CacheCreationTokens, p.CacheReadTokens, p.TotalTokens,
		p.StartTime, p.FinishTime,
	)
	return err
}

// QueryRollingHourCost returns the sum of delta_cost for the rolling 60-minute window.
// If owner and repo are non-empty, the query is scoped to that project.
// Returns (0, nil) if db is nil.
func QueryRollingHourCost(db *sql.DB, owner, repo string) (float64, error) {
	if db == nil {
		return 0, nil
	}

	var cost float64
	if owner != "" && repo != "" {
		err := db.QueryRow(
			`SELECT COALESCE(SUM(delta_cost), 0) FROM checkpoints
			 WHERE timestamp >= datetime('now', '-60 minutes')
			   AND owner = ? AND repo = ?`,
			owner, repo,
		).Scan(&cost)
		return cost, err
	}

	err := db.QueryRow(
		`SELECT COALESCE(SUM(delta_cost), 0) FROM checkpoints
		 WHERE timestamp >= datetime('now', '-60 minutes')`,
	).Scan(&cost)
	return cost, err
}

// QueryRollingWakeTime returns the earliest time at which the rolling 60-minute window
// cost sum will drop below limit. It walks checkpoints oldest-first, subtracting each
// row's delta_cost from the total. When the remaining cost drops below limit, the wake
// time is that row's timestamp + 60 minutes. If no single row's aging-out is sufficient,
// returns time.Now().Add(60 * time.Minute) as a fallback.
// Returns (time.Time{}, nil) if db is nil.
func QueryRollingWakeTime(db *sql.DB, owner, repo string, limit float64) (time.Time, error) {
	if db == nil {
		return time.Time{}, nil
	}

	var query string
	var args []interface{}
	if owner != "" && repo != "" {
		query = `SELECT delta_cost, timestamp FROM checkpoints
				 WHERE timestamp >= datetime('now', '-60 minutes')
				   AND owner = ? AND repo = ?
				 ORDER BY timestamp ASC`
		args = []interface{}{owner, repo}
	} else {
		query = `SELECT delta_cost, timestamp FROM checkpoints
				 WHERE timestamp >= datetime('now', '-60 minutes')
				 ORDER BY timestamp ASC`
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return time.Time{}, err
	}
	defer rows.Close()

	type checkpoint struct {
		deltaCost float64
		timestamp string
	}

	var checkpoints []checkpoint
	var totalCost float64
	for rows.Next() {
		var cp checkpoint
		if err := rows.Scan(&cp.deltaCost, &cp.timestamp); err != nil {
			return time.Time{}, err
		}
		totalCost += cp.deltaCost
		checkpoints = append(checkpoints, cp)
	}
	if err := rows.Err(); err != nil {
		return time.Time{}, err
	}

	// Walk oldest to newest, subtracting each row's cost
	for _, cp := range checkpoints {
		totalCost -= cp.deltaCost
		if totalCost < limit {
			ts, err := time.Parse(time.RFC3339, cp.timestamp)
			if err != nil {
				return time.Time{}, fmt.Errorf("failed to parse checkpoint timestamp %q: %w", cp.timestamp, err)
			}
			return ts.Add(60 * time.Minute), nil
		}
	}

	// Fallback: no single row's aging-out is sufficient
	return time.Now().UTC().Add(60 * time.Minute), nil
}

// ExportSessionTSV returns a TSV-formatted string of loop_stats rows for the given session.
// Returns empty string if no rows found.
func ExportSessionTSV(db *sql.DB, sessionID string) (string, error) {
	if db == nil {
		return "", nil
	}

	rows, err := db.Query(
		`SELECT loop_id, session_id, owner, repo, branch, description, total_cost,
		        input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
		        total_tokens, start_time, finish_time
		 FROM loop_stats WHERE session_id = ? ORDER BY loop_id`,
		sessionID,
	)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var b strings.Builder
	header := "loop_id\tsession_id\towner\trepo\tbranch\tdescription\ttotal_cost\tinput_tokens\toutput_tokens\tcache_creation_tokens\tcache_read_tokens\ttotal_tokens\tstart_time\tfinish_time"
	hasRows := false

	for rows.Next() {
		if !hasRows {
			b.WriteString(header)
			b.WriteByte('\n')
			hasRows = true
		}
		var loopID, sessID, owner, repo, branch, desc, startTime, finishTime string
		var totalCost float64
		var input, output, cacheCreation, cacheRead, total int64
		if err := rows.Scan(&loopID, &sessID, &owner, &repo, &branch, &desc, &totalCost,
			&input, &output, &cacheCreation, &cacheRead, &total, &startTime, &finishTime); err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%s\t%s\t%.6f\t%d\t%d\t%d\t%d\t%d\t%s\t%s\n",
			loopID, sessID, owner, repo, branch, desc, totalCost,
			input, output, cacheCreation, cacheRead, total, startTime, finishTime)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}

	return b.String(), nil
}
