package stats

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
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

// ModelPricing holds per-token USD list prices for a Claude model tier.
// CacheCreation uses the 5-minute cache-write rate (1.25x input); CacheRead is
// the cache-read rate (0.1x input). The Claude CLI usage stream reports a single
// cache_creation_input_tokens figure with no TTL breakdown, so the 5-minute rate
// is used for the estimate (matching behavior from before pricing was model-aware).
type ModelPricing struct {
	Input         float64
	Output        float64
	CacheCreation float64
	CacheRead     float64
}

// Per-token price sets by model tier (list prices, USD per token) for the
// current model generation: Opus 4.5+ at $5/$25, Sonnet 4.x at $3/$15.
// PricingForModel matches by tier substring rather than exact ID so new point
// releases within a tier need no code change. Older Opus (4.0/4.1, listed at
// $15/$75) would be under-estimated, but those aren't current-gen and any
// estimate reconciles to the CLI's actual cost once a result arrives.
var (
	pricingOpus   = ModelPricing{5.00 / 1_000_000, 25.00 / 1_000_000, 6.25 / 1_000_000, 0.50 / 1_000_000}
	pricingSonnet = ModelPricing{3.00 / 1_000_000, 15.00 / 1_000_000, 3.75 / 1_000_000, 0.30 / 1_000_000}
	pricingHaiku  = ModelPricing{1.00 / 1_000_000, 5.00 / 1_000_000, 1.25 / 1_000_000, 0.10 / 1_000_000}
	pricingFable  = ModelPricing{10.00 / 1_000_000, 50.00 / 1_000_000, 12.50 / 1_000_000, 1.00 / 1_000_000}
)

// DefaultPricing is used when the model identifier is empty or unrecognized.
// It mirrors Claude Sonnet rates, preserving the behavior from before pricing
// was made model-aware.
var DefaultPricing = pricingSonnet

// PricingForModel returns the price set for a Claude model identifier (e.g.
// "claude-opus-4-8"), matching by tier substring. Empty or unrecognized
// identifiers fall back to DefaultPricing.
func PricingForModel(model string) ModelPricing {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "opus"):
		return pricingOpus
	case strings.Contains(m, "sonnet"):
		return pricingSonnet
	case strings.Contains(m, "haiku"):
		return pricingHaiku
	case strings.Contains(m, "fable"):
		return pricingFable
	default:
		return DefaultPricing
	}
}

// EstimateCostFromTokens computes estimated cost from token counts using the
// price set for the given model. An empty or unrecognized model uses
// DefaultPricing.
func EstimateCostFromTokens(model string, input, output, cacheCreation, cacheRead int64) float64 {
	p := PricingForModel(model)
	return float64(input)*p.Input +
		float64(output)*p.Output +
		float64(cacheCreation)*p.CacheCreation +
		float64(cacheRead)*p.CacheRead
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

// Snapshot is an immutable, lock-free point-in-time copy of a TokenStats's
// counters. It deliberately carries no mutex, so callers may copy it freely —
// store it in a struct field, pass it by value, or hand it to a fmt verb —
// without copying a lock (which go vet's copylocks check forbids).
type Snapshot struct {
	InputTokens         int64
	OutputTokens        int64
	CacheCreationTokens int64
	CacheReadTokens     int64
	TotalCostUSD        float64
	TotalTokensCount    int64
	TotalElapsedNs      int64
}

// Snapshot returns a consistent point-in-time copy of the stats for reading
// without holding the lock.
func (t *TokenStats) Snapshot() Snapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return Snapshot{
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

	const createProjectStats = `CREATE TABLE IF NOT EXISTS project_stats (
		project_key           TEXT PRIMARY KEY,
		input_tokens          INTEGER DEFAULT 0,
		output_tokens         INTEGER DEFAULT 0,
		cache_creation_tokens INTEGER DEFAULT 0,
		cache_read_tokens     INTEGER DEFAULT 0,
		total_cost            REAL DEFAULT 0,
		total_tokens          INTEGER DEFAULT 0,
		elapsed_ns            INTEGER DEFAULT 0
	)`
	if _, err := db.Exec(createProjectStats); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating project_stats table: %w", err)
	}

	// Prune old checkpoint rows
	if _, err := db.Exec("DELETE FROM checkpoints WHERE timestamp < datetime('now', '-7 days')"); err != nil {
		db.Close()
		return nil, fmt.Errorf("pruning old checkpoints: %w", err)
	}

	return db, nil
}

// SaveProjectStats persists cumulative token stats for a project key.
func SaveProjectStats(db *sql.DB, projectKey string, s *TokenStats) error {
	if db == nil {
		return nil
	}
	snap := s.Snapshot()
	_, err := db.Exec(
		`INSERT OR REPLACE INTO project_stats (project_key, input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens, total_cost, total_tokens, elapsed_ns)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		projectKey, snap.InputTokens, snap.OutputTokens, snap.CacheCreationTokens, snap.CacheReadTokens,
		snap.TotalCostUSD, snap.TotalTokensCount, snap.TotalElapsedNs,
	)
	return err
}

// LoadProjectStats loads cumulative token stats for a project key.
// Returns zeroed stats (not an error) when no row exists or db is nil.
func LoadProjectStats(db *sql.DB, projectKey string) (*TokenStats, error) {
	if db == nil {
		return NewTokenStats(), nil
	}
	row := db.QueryRow(
		`SELECT input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens, total_cost, total_tokens, elapsed_ns
		 FROM project_stats WHERE project_key = ?`, projectKey,
	)
	s := NewTokenStats()
	err := row.Scan(&s.InputTokens, &s.OutputTokens, &s.CacheCreationTokens, &s.CacheReadTokens,
		&s.TotalCostUSD, &s.TotalTokensCount, &s.TotalElapsedNs)
	if err == sql.ErrNoRows {
		return NewTokenStats(), nil
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

// ProjectKey returns a stable key for per-project stats.
// For git repos (both owner and repo non-empty), returns "owner/repo".
// Otherwise falls back to the absolute working directory path.
func ProjectKey(owner, repo string) string {
	if owner != "" && repo != "" {
		return owner + "/" + repo
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return cwd
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
			 WHERE timestamp >= strftime('%Y-%m-%dT%H:%M:%S', 'now', '-60 minutes')
			   AND owner = ? AND repo = ?`,
			owner, repo,
		).Scan(&cost)
		return cost, err
	}

	err := db.QueryRow(
		`SELECT COALESCE(SUM(delta_cost), 0) FROM checkpoints
		 WHERE timestamp >= strftime('%Y-%m-%dT%H:%M:%S', 'now', '-60 minutes')`,
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
				 WHERE timestamp >= strftime('%Y-%m-%dT%H:%M:%S', 'now', '-60 minutes')
				   AND owner = ? AND repo = ?
				 ORDER BY timestamp ASC`
		args = []interface{}{owner, repo}
	} else {
		query = `SELECT delta_cost, timestamp FROM checkpoints
				 WHERE timestamp >= strftime('%Y-%m-%dT%H:%M:%S', 'now', '-60 minutes')
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

