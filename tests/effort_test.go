package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudosai/ralph-go/internal/config"
)

// ============================================================================
// Tests: effort resolution
//
// The claude CLI's effort levels are low/medium/high/xhigh/max. Ralph knows
// which one is in force from two sources, because the stream-json output it
// parses never names one:
//
//	ResolveEffort     — --effort, else `effortLevel` in the settings chain.
//	                    Available immediately, at startup.
//	TranscriptEffort  — the level the CLI recorded on the session's assistant
//	                    records. Ground truth, but only once the session is
//	                    under way.
//
// Both are display-only; neither changes what is passed to `claude --effort`.
// ============================================================================

// writeSettings writes a .claude/settings file with the given raw JSON body,
// creating the parent directories.
func writeSettings(t *testing.T, path, body string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

// isolateSettings points both ends of the settings chain at temp directories:
// the working directory (for .claude/settings*.json) and HOME (for the global
// one). Without this, the developer's own ~/.claude/settings.json leaks in and
// the "nothing configured" cases pass or fail by accident.
func isolateSettings(t *testing.T) {
	t.Helper()

	t.Chdir(t.TempDir())
	t.Setenv("HOME", t.TempDir())
}

func TestResolveEffort_FlagWins(t *testing.T) {
	isolateSettings(t)
	writeSettings(t, filepath.Join(".claude", "settings.json"), `{"effortLevel":"low"}`)

	// --effort goes on the command line, so it beats any settings file — the
	// settings value must not be reported as the level in force.
	if got := config.ResolveEffort("max"); got != "max" {
		t.Errorf("ResolveEffort(\"max\") = %q, want \"max\" (the flag outranks settings)", got)
	}
}

func TestResolveEffort_ReadsProjectSettings(t *testing.T) {
	isolateSettings(t)
	writeSettings(t, filepath.Join(".claude", "settings.json"), `{"effortLevel":"high"}`)

	if got := config.ResolveEffort(""); got != "high" {
		t.Errorf("ResolveEffort(\"\") = %q, want \"high\" from .claude/settings.json", got)
	}
}

func TestResolveEffort_LocalSettingsOutrankShared(t *testing.T) {
	isolateSettings(t)
	writeSettings(t, filepath.Join(".claude", "settings.json"), `{"effortLevel":"high"}`)
	writeSettings(t, filepath.Join(".claude", "settings.local.json"), `{"effortLevel":"xhigh"}`)

	// Same precedence the claude CLI applies: local overrides shared.
	if got := config.ResolveEffort(""); got != "xhigh" {
		t.Errorf("ResolveEffort(\"\") = %q, want \"xhigh\" from settings.local.json", got)
	}
}

func TestResolveEffort_ProjectSettingsOutrankUser(t *testing.T) {
	isolateSettings(t)
	writeSettings(t, filepath.Join(".claude", "settings.json"), `{"effortLevel":"low"}`)
	writeSettings(t, filepath.Join(os.Getenv("HOME"), ".claude", "settings.json"), `{"effortLevel":"max"}`)

	// The whole chain has to be walked in the CLI's order, not just "first file
	// that exists": a repo that pins an effort level overrides the user's global
	// one, so reading ~/.claude first would report a level the CLI is not using.
	if got := config.ResolveEffort(""); got != "low" {
		t.Errorf("ResolveEffort(\"\") = %q, want \"low\" — project settings outrank ~/.claude", got)
	}
}

func TestResolveEffort_FallsBackToUserSettings(t *testing.T) {
	isolateSettings(t)
	writeSettings(t, filepath.Join(os.Getenv("HOME"), ".claude", "settings.json"), `{"effortLevel":"medium"}`)

	if got := config.ResolveEffort(""); got != "medium" {
		t.Errorf("ResolveEffort(\"\") = %q, want \"medium\" from ~/.claude/settings.json", got)
	}
}

func TestResolveEffort_SkipsFilesWithoutTheKey(t *testing.T) {
	isolateSettings(t)
	// A settings file that exists but configures something else must not stop
	// the walk — the next source down still applies.
	writeSettings(t, filepath.Join(".claude", "settings.json"), `{"model":"claude-opus-4-8"}`)
	writeSettings(t, filepath.Join(os.Getenv("HOME"), ".claude", "settings.json"), `{"effortLevel":"low"}`)

	if got := config.ResolveEffort(""); got != "low" {
		t.Errorf("ResolveEffort(\"\") = %q, want \"low\" — a file without effortLevel should not end the walk", got)
	}
}

func TestResolveEffort_SkipsMalformedSettings(t *testing.T) {
	isolateSettings(t)
	// Half-written or hand-broken JSON is not fatal.
	writeSettings(t, filepath.Join(".claude", "settings.json"), `{"effortLevel": `)
	writeSettings(t, filepath.Join(os.Getenv("HOME"), ".claude", "settings.json"), `{"effortLevel":"max"}`)

	if got := config.ResolveEffort(""); got != "max" {
		t.Errorf("ResolveEffort(\"\") = %q, want \"max\" — malformed JSON should be skipped, not fatal", got)
	}
}

func TestResolveEffort_EmptyWhenNothingConfigured(t *testing.T) {
	isolateSettings(t)

	// Nothing configures a level, so the CLI picks for itself and Ralph must
	// say so rather than invent one. The TUI renders this as "-".
	if got := config.ResolveEffort(""); got != "" {
		t.Errorf("ResolveEffort(\"\") = %q, want \"\" when no source configures a level", got)
	}
}

// writeTranscript writes a session transcript at the path the claude CLI uses,
// under a project directory whose name does not matter — TranscriptEffort finds
// it by session ID.
func writeTranscript(t *testing.T, sessionID, body string) {
	t.Helper()

	dir := filepath.Join(os.Getenv("HOME"), ".claude", "projects", "-tmp-project")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", dir, err)
	}
	path := filepath.Join(dir, sessionID+".jsonl")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

const testSessionID = "a50b7b11-7b8d-44a7-bdf5-0433d84f3fb1"

func TestTranscriptEffort_ReadsRecordedLevel(t *testing.T) {
	isolateSettings(t)
	writeTranscript(t, testSessionID, `{"type":"user","message":{"role":"user"}}
{"type":"assistant","effort":"xhigh","message":{"model":"claude-opus-5"}}
`)

	if got := config.TranscriptEffort(testSessionID); got != "xhigh" {
		t.Errorf("TranscriptEffort = %q, want \"xhigh\" from the assistant record", got)
	}
}

func TestTranscriptEffort_LastRecordWins(t *testing.T) {
	isolateSettings(t)
	// A --resume'd session keeps earlier iterations' records in the same file;
	// the newest level is the one in force.
	writeTranscript(t, testSessionID, `{"type":"assistant","effort":"low"}
{"type":"assistant","effort":"max"}
`)

	if got := config.TranscriptEffort(testSessionID); got != "max" {
		t.Errorf("TranscriptEffort = %q, want \"max\" — the newest record wins", got)
	}
}

func TestTranscriptEffort_ToleratesHalfWrittenTail(t *testing.T) {
	isolateSettings(t)
	// The CLI appends to this file while Ralph reads it, so the final record
	// can be truncated mid-write. Everything decoded before it still counts.
	writeTranscript(t, testSessionID, `{"type":"assistant","effort":"high"}
{"type":"assistant","effo`)

	if got := config.TranscriptEffort(testSessionID); got != "high" {
		t.Errorf("TranscriptEffort = %q, want \"high\" — a truncated trailing record should not discard earlier ones", got)
	}
}

func TestTranscriptEffort_EmptyBeforeFirstAssistantRecord(t *testing.T) {
	isolateSettings(t)
	// Only assistant records carry the level, so early in a session there is
	// nothing to read yet. The caller retries rather than treating this as
	// "no effort configured".
	writeTranscript(t, testSessionID, `{"type":"user","message":{"role":"user"}}
`)

	if got := config.TranscriptEffort(testSessionID); got != "" {
		t.Errorf("TranscriptEffort = %q, want \"\" before the first assistant record", got)
	}
}

func TestTranscriptEffort_EmptyWhenNoTranscript(t *testing.T) {
	isolateSettings(t)

	if got := config.TranscriptEffort(testSessionID); got != "" {
		t.Errorf("TranscriptEffort = %q, want \"\" when no transcript exists for the session", got)
	}
}

func TestTranscriptEffort_RejectsNonSessionIDs(t *testing.T) {
	isolateSettings(t)
	writeTranscript(t, testSessionID, `{"type":"assistant","effort":"max"}`+"\n")

	// The ID is interpolated into a glob and a path, so anything not shaped
	// like the CLI's UUID is refused outright — a wildcard must not match the
	// real transcript, and a traversal must not reach outside the projects dir.
	for _, id := range []string{"", "*", "../../etc/passwd", "a50b7b11/../../x"} {
		if got := config.TranscriptEffort(id); got != "" {
			t.Errorf("TranscriptEffort(%q) = %q, want \"\" — non-UUID session IDs must be refused", id, got)
		}
	}
}
