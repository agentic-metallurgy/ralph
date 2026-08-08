package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// claudeSettings is the subset of a .claude/settings*.json file Ralph reads.
type claudeSettings struct {
	EffortLevel string `json:"effortLevel"`
}

// transcriptRecord is the subset of a session transcript record Ralph reads.
type transcriptRecord struct {
	Effort string `json:"effort"`
}

// effortSettingsPaths returns the settings files that can define an effort
// level, highest precedence first: project-local, then project-shared, then
// user-global — the order the claude CLI itself resolves them in. Project paths
// are relative to the working directory, which is where Ralph runs claude.
//
// Enterprise managed settings are deliberately not consulted: they outrank even
// --effort, so a machine under managed policy can run at a level neither this
// function nor Ralph's own flag decides.
func effortSettingsPaths() []string {
	paths := []string{
		filepath.Join(".claude", "settings.local.json"),
		filepath.Join(".claude", "settings.json"),
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".claude", "settings.json"))
	}
	return paths
}

// ResolveEffort reports the effort level the claude CLI will actually run at,
// for display only — the value passed to `claude --effort` stays Config.Effort,
// so resolving here never changes what the loop executes.
//
// A non-empty --effort wins outright, since Ralph puts it on the command line.
// Otherwise the CLI falls back to `effortLevel` in its settings chain, which
// Ralph has to read itself: unlike the model, which the stream reports on the
// system/init line, the effort level never appears in the stream-json output,
// so these files are the only source. Returns "" when nothing configures one
// and the CLI picks for itself.
func ResolveEffort(flagEffort string) string {
	if flagEffort != "" {
		return flagEffort
	}
	for _, path := range effortSettingsPaths() {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var settings claudeSettings
		if err := json.Unmarshal(data, &settings); err != nil {
			// A malformed or half-written settings file is not fatal; the rest
			// of the chain still applies.
			continue
		}
		if settings.EffortLevel != "" {
			return settings.EffortLevel
		}
	}
	return ""
}

// isSessionID reports whether id is shaped like the UUID the claude CLI hands
// out, which TranscriptEffort interpolates into a glob and a path. Rejecting
// anything else keeps a surprising session id out of both.
func isSessionID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F', r == '-':
		default:
			return false
		}
	}
	return true
}

// TranscriptEffort reports the effort level the claude CLI itself recorded for
// a session, or "" if it cannot be determined (yet).
//
// This is ground truth where ResolveEffort is only a good guess: the CLI writes
// the level it actually resolved onto every assistant record of the session
// transcript, so it accounts for precedence Ralph does not reimplement —
// notably enterprise managed settings, which outrank even --effort. The cost is
// a dependency on an undocumented internal file format, so every failure here
// is silent and simply leaves the resolved-from-settings value in place.
//
// It returns "" until the CLI has written its first assistant record, which
// lands a few seconds into an iteration; callers are expected to retry.
func TranscriptEffort(sessionID string) string {
	if !isSessionID(sessionID) {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	// Session IDs are unique, so globbing across the project directories finds
	// the transcript without reproducing the CLI's cwd→directory-name mangling.
	matches, err := filepath.Glob(filepath.Join(home, ".claude", "projects", "*", sessionID+".jsonl"))
	if err != nil || len(matches) == 0 {
		return ""
	}
	file, err := os.Open(matches[0])
	if err != nil {
		return ""
	}
	defer file.Close()

	// Records are newline-delimited JSON, but a single one can be megabytes
	// (a large tool result), so decode the file as a stream of values rather
	// than scanning it by line.
	//
	// The last record wins: a --resume'd session keeps the earlier iterations'
	// records in the same file, and the newest is the one in force.
	var effort string
	decoder := json.NewDecoder(file)
	for {
		var record transcriptRecord
		if err := decoder.Decode(&record); err != nil {
			break // EOF, or a half-written trailing record — keep what we have.
		}
		if record.Effort != "" {
			effort = record.Effort
		}
	}
	return effort
}
