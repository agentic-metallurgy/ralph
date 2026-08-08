package tests

// ============================================================================
// BDD Test Suite: Tmux Status Bar Content (Task 8)
//
// User goal: while ralph is running inside tmux, the status bar shows repo name,
// branch and the stats of the CURRENT LOOP ITERATION — progress, tokens burned
// and time spent in this iteration — so the user can judge how the running
// iteration is going at a glance without looking at the TUI.
//
// Cumulative session figures deliberately do NOT live here: they stay in the TUI
// footer ("Total Tokens" / "Total Time"), so the two surfaces complement each
// other instead of duplicating.
//
// updateTmuxStatusBar() is called on every tick and formats the content as:
//
//	[repo | branch | loop: N/M, tokens: 45k, elapsed: HH:MM:SS]
//
// Both `tokens:` and `elapsed:` reset when a new iteration begins
// (tui.SendLoopStarted), which is what makes them per-loop rather than session
// totals.
//
// During hibernation the loop field carries the countdown and the token/elapsed
// fields are omitted entirely — no dangling labels:
//
//	[repo | branch | loop: RATE LIMITED 💤 MM:SS]
//
// These tests inject a FakeStatusBarForTest, trigger a tick, and assert the
// content passed to the fake bar matches expectations.
// ============================================================================

import (
	"strings"
	"testing"
	"time"

	"github.com/cloudosai/ralph-go/internal/tui"
)

// --- Helpers ---

// setupModelWithFakeBar creates a ready model with a fake tmux status bar.
// Returns the model and the fake bar so tests can inspect LastContent after a tick.
func setupModelWithFakeBar(current, total int) (tui.Model, *tui.FakeStatusBarForTest) {
	m, _ := setupReadyModelWithLoop(current, total)
	fakeBar := &tui.FakeStatusBarForTest{}
	m.SetTmuxStatusBar(fakeBar)
	return m, fakeBar
}

// triggerTick sends a tick message to the model.
func triggerTick(m tui.Model) tui.Model {
	m, _ = updateModel(m, tui.TickMsgForTest())
	return m
}

// --- Scenario 1: Normal state shows loop progress ---

// TestBDD_TmuxStatusBar_LoopProgressShownOnTick
//
// Given: a model with loop at #2/5 and a fake tmux status bar
// When: a tick occurs
// Then: the tmux bar content includes the loop progress "#2/5"
func TestBDD_TmuxStatusBar_LoopProgressShownOnTick(t *testing.T) {
	m, fakeBar := setupModelWithFakeBar(2, 5)

	if fakeBar.LastContent != "" {
		t.Fatal("Precondition: tmux bar should be empty before tick")
	}

	// When: a tick occurs
	triggerTick(m)

	// Then: loop progress shown
	if !strings.Contains(fakeBar.LastContent, "2/5") {
		t.Errorf("Expected 2/5 in tmux bar, got: %q", fakeBar.LastContent)
	}
}

// --- Scenario 2: Default state shows #0/0 ---

// TestBDD_TmuxStatusBar_DefaultProgressShownBeforeLoopSet
//
// Given: a fresh model with no loop progress set and a fake tmux bar
// When: a tick occurs
// Then: the tmux bar content includes "#0/0"
func TestBDD_TmuxStatusBar_DefaultProgressShownBeforeLoopSet(t *testing.T) {
	m := setupReadyModel()
	fakeBar := &tui.FakeStatusBarForTest{}
	m.SetTmuxStatusBar(fakeBar)

	// When: a tick occurs
	triggerTick(m)

	// Then: default loop progress shown
	if !strings.Contains(fakeBar.LastContent, "0/0") {
		t.Errorf("Expected 0/0 in tmux bar before loop is set, got: %q", fakeBar.LastContent)
	}
}

// --- Scenario 3: Repo and branch shown ---

// TestBDD_TmuxStatusBar_RepoBranchShownOnTick
//
// Given: a model with git context set (repo="myrepo", branch="feat-x")
// When: a tick occurs
// Then: the tmux bar content includes the repo and branch names
func TestBDD_TmuxStatusBar_RepoBranchShownOnTick(t *testing.T) {
	m, fakeBar := setupModelWithFakeBar(1, 3)

	// Given: git context set
	m.SetGitContext("myrepo", "feat-x")

	// When: tick
	triggerTick(m)

	// Then: repo and branch shown
	if !strings.Contains(fakeBar.LastContent, "myrepo") {
		t.Errorf("Expected 'myrepo' in tmux bar, got: %q", fakeBar.LastContent)
	}
	if !strings.Contains(fakeBar.LastContent, "feat-x") {
		t.Errorf("Expected 'feat-x' in tmux bar, got: %q", fakeBar.LastContent)
	}
}

// --- Scenario 4: Elapsed time is per-loop, not per-session ---

// TestBDD_TmuxStatusBar_ElapsedTimeIsPerLoopNotSession
//
// Given: a model that has been running for 65s inside its first loop iteration
// When: a new loop iteration starts and time keeps advancing (mocked clock)
// Then: the bar's "elapsed:" field restarts from 00:00:00 and counts from the
// loop start, while the cumulative session time keeps running in the TUI footer
func TestBDD_TmuxStatusBar_ElapsedTimeIsPerLoopNotSession(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	now := baseTime
	tui.SetTimeNowForTest(func() time.Time { return now })
	defer tui.SetTimeNowForTest(time.Now)

	// Given: a model created at baseTime (session start == first loop start)
	m, fakeBar := setupModelWithFakeBar(1, 3)

	// And: 65 seconds pass inside the first iteration
	now = baseTime.Add(65 * time.Second)
	m = triggerTick(m)

	if !strings.Contains(fakeBar.LastContent, "elapsed: 00:01:05") {
		t.Fatalf("Precondition: expected 'elapsed: 00:01:05' after 65s in the first loop, got: %q", fakeBar.LastContent)
	}

	// When: a new loop iteration starts, with no wall-clock time passing
	m, _ = sendTuiMsg(m, tui.SendLoopStarted())
	m = triggerTick(m)

	// Then: the bar's elapsed field resets to zero for the new iteration...
	if !strings.Contains(fakeBar.LastContent, "elapsed: 00:00:00") {
		t.Errorf("Expected 'elapsed: 00:00:00' immediately after a new loop started, got: %q", fakeBar.LastContent)
	}
	if strings.Contains(fakeBar.LastContent, "00:01:05") {
		t.Errorf("Bar still shows 00:01:05 after the loop restarted — it is reporting session time, not loop time: %q", fakeBar.LastContent)
	}

	// ...even though the session clock never stopped: the footer still shows 65s.
	if !viewContains(m, "00:01:05") {
		t.Errorf("Expected the TUI footer to still show cumulative session time 00:01:05, view:\n%s", m.View())
	}

	// When: 30 more seconds pass inside the second iteration (session total 1m35s)
	now = now.Add(30 * time.Second)
	m = triggerTick(m)

	// Then: the bar counts from the loop start (30s), not the session start (1m35s)
	if !strings.Contains(fakeBar.LastContent, "elapsed: 00:00:30") {
		t.Errorf("Expected 'elapsed: 00:00:30' (time since the loop started), got: %q", fakeBar.LastContent)
	}
	if strings.Contains(fakeBar.LastContent, "00:01:35") {
		t.Errorf("Bar shows session elapsed 00:01:35 instead of loop elapsed 00:00:30: %q", fakeBar.LastContent)
	}
	if !viewContains(m, "00:01:35") {
		t.Errorf("Expected the TUI footer to show cumulative session time 00:01:35, view:\n%s", m.View())
	}
}

// --- Scenario 5: Token count is per-loop and resets on a new loop ---

// TestBDD_TmuxStatusBar_LoopTokensShownAndResetOnNewLoop
//
// Given: a model whose current iteration has consumed 45,000 tokens
// When: a tick occurs, and later a new loop iteration starts
// Then: the bar shows "tokens: 45k", then resets to "tokens: 0"
func TestBDD_TmuxStatusBar_LoopTokensShownAndResetOnNewLoop(t *testing.T) {
	m, fakeBar := setupModelWithFakeBar(2, 5)

	// Given: the current iteration reports 45,000 tokens
	m, _ = sendTuiMsg(m, tui.SendLoopStatsUpdate(45000))

	// When: tick
	m = triggerTick(m)

	// Then: the per-loop token count reaches the bar in humanised form
	if !strings.Contains(fakeBar.LastContent, "tokens: 45k") {
		t.Errorf("Expected 'tokens: 45k' in tmux bar, got: %q", fakeBar.LastContent)
	}

	// When: a new loop iteration starts and another tick occurs
	m, _ = sendTuiMsg(m, tui.SendLoopStarted())
	m = triggerTick(m)

	// Then: the token count resets for the new iteration
	if !strings.Contains(fakeBar.LastContent, "tokens: 0") {
		t.Errorf("Expected 'tokens: 0' after a new loop started, got: %q", fakeBar.LastContent)
	}
	if strings.Contains(fakeBar.LastContent, "45k") {
		t.Errorf("Bar still shows the previous iteration's 45k tokens — tokens are cumulative, not per-loop: %q", fakeBar.LastContent)
	}
}

// --- Scenario 6: Hibernating state shows RATE LIMITED label ---

// TestBDD_TmuxStatusBar_RateLimitedLabelShownDuringHibernate
//
// Given: a model whose loop is hibernating
// When: a tick occurs
// Then: the tmux bar shows "RATE LIMITED" and omits the token/elapsed fields
func TestBDD_TmuxStatusBar_RateLimitedLabelShownDuringHibernate(t *testing.T) {
	// Given: the loop in hibernate state (matches real pipeline)
	m, _ := setupHibernatingModel(2, 5, 5*time.Minute)
	fakeBar := &tui.FakeStatusBarForTest{}
	m.SetTmuxStatusBar(fakeBar)

	// Precondition: view shows RATE LIMITED
	if !viewContains(m, "RATE LIMITED") {
		t.Fatal("Precondition: model should show RATE LIMITED after hibernate")
	}

	// When: tick
	triggerTick(m)

	// Then: RATE LIMITED shown in tmux bar
	if !strings.Contains(fakeBar.LastContent, "RATE LIMITED") {
		t.Errorf("Expected RATE LIMITED in tmux bar during hibernate, got: %q", fakeBar.LastContent)
	}

	// And: no dangling per-loop labels while nothing is running
	if strings.Contains(fakeBar.LastContent, "tokens:") {
		t.Errorf("Expected no 'tokens:' field during hibernate, got: %q", fakeBar.LastContent)
	}
	if strings.Contains(fakeBar.LastContent, "elapsed:") {
		t.Errorf("Expected no 'elapsed:' field during hibernate, got: %q", fakeBar.LastContent)
	}
}

// --- Scenario 7: Hibernating state shows sleep emoji countdown ---

// TestBDD_TmuxStatusBar_SleepEmojiCountdownShownDuringHibernate
//
// Given: a model in hibernating state with 3 minutes 30 seconds remaining (mocked time)
// When: a tick occurs
// Then: the tmux bar content is exactly the hibernate variant "[repo | branch |
// loop: RATE LIMITED 💤 03:30]" — countdown present, token/elapsed fields absent
func TestBDD_TmuxStatusBar_SleepEmojiCountdownShownDuringHibernate(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tui.SetTimeNowForTest(func() time.Time { return baseTime })
	defer tui.SetTimeNowForTest(time.Now)

	// Given: the loop hibernating with 3m30s remaining
	// hibernateUntil is baseTime + 3m30s; timeNow() is baseTime → 3m30s remain
	hibernateUntil := baseTime.Add(3*time.Minute + 30*time.Second)
	m, l := setupReadyModelWithLoop(2, 5)
	m.SetGitContext("ralph", "main")
	l.Hibernate(hibernateUntil)

	fakeBar := &tui.FakeStatusBarForTest{}
	m.SetTmuxStatusBar(fakeBar)

	// When: tick (timeNow still at baseTime)
	triggerTick(m)

	// Then: countdown shows 💤 03:30
	if !strings.Contains(fakeBar.LastContent, "💤 03:30") {
		t.Errorf("Expected '💤 03:30' in tmux bar during hibernate, got: %q", fakeBar.LastContent)
	}

	// And: the hibernate variant carries no per-loop token/elapsed fields
	if strings.Contains(fakeBar.LastContent, "tokens:") {
		t.Errorf("Expected no 'tokens:' field during hibernate, got: %q", fakeBar.LastContent)
	}
	if strings.Contains(fakeBar.LastContent, "elapsed:") {
		t.Errorf("Expected no 'elapsed:' field during hibernate, got: %q", fakeBar.LastContent)
	}

	// And: the whole bar is the documented hibernate format
	wantHibernate := "[ralph | main | loop: RATE LIMITED 💤 03:30]"
	if fakeBar.LastContent != wantHibernate {
		t.Errorf("Expected hibernate bar %q, got: %q", wantHibernate, fakeBar.LastContent)
	}
}

// --- Scenario 8: No update when tmux bar is inactive ---

// TestBDD_TmuxStatusBar_NoUpdateWhenBarNotSet
//
// Given: a model with no tmux bar configured (nil)
// When: a tick occurs
// Then: no panic occurs (the nil bar is a no-op)
func TestBDD_TmuxStatusBar_NoUpdateWhenBarNotSet(t *testing.T) {
	m := setupReadyModel()
	// No tmux bar set — m.tmuxBar is nil

	// When: tick (should not panic)
	triggerTick(m)

	// Then: no panic (test passes if we get here)
}

// --- Scenario 9: Full status bar format ---

// TestBDD_TmuxStatusBar_FullFormatContainsAllFields
//
// Given: a model with loop at 3/7, git context set, frozen clock and no tokens yet
// When: a tick occurs
// Then: the bar is exactly "[ralph | main | loop: 3/7, tokens: 0, elapsed: 00:00:00]"
// with the current labels ("elapsed:", not the old "uptime:")
func TestBDD_TmuxStatusBar_FullFormatContainsAllFields(t *testing.T) {
	frozen := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tui.SetTimeNowForTest(func() time.Time { return frozen })
	defer tui.SetTimeNowForTest(time.Now)

	m, fakeBar := setupModelWithFakeBar(3, 7)
	m.SetGitContext("ralph", "main")

	// When: tick
	m = triggerTick(m)

	// Then: all fields present in correct format
	content := fakeBar.LastContent
	want := "[ralph | main | loop: 3/7, tokens: 0, elapsed: 00:00:00]"
	if content != want {
		t.Errorf("Expected tmux bar %q, got: %q", want, content)
	}
	if !strings.Contains(content, "loop:") {
		t.Errorf("Expected 'loop:' label in tmux bar, got: %q", content)
	}
	if !strings.Contains(content, "tokens:") {
		t.Errorf("Expected 'tokens:' label in tmux bar, got: %q", content)
	}
	if !strings.Contains(content, "elapsed:") {
		t.Errorf("Expected 'elapsed:' label in tmux bar, got: %q", content)
	}
	if strings.Contains(content, "uptime:") {
		t.Errorf("Expected the old 'uptime:' label to be gone, got: %q", content)
	}
	if !strings.Contains(content, "3/7") {
		t.Errorf("Expected '3/7' in tmux bar, got: %q", content)
	}
	if !strings.Contains(content, " | ") {
		t.Errorf("Expected ' | ' separator in tmux bar, got: %q", content)
	}
	if !strings.Contains(content, "ralph") {
		t.Errorf("Expected 'ralph' repo name in tmux bar, got: %q", content)
	}
	if !strings.Contains(content, "main") {
		t.Errorf("Expected 'main' branch name in tmux bar, got: %q", content)
	}
}
