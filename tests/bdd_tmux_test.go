package tests

// ============================================================================
// BDD Test Suite: Tmux Status Bar Content (Task 8)
//
// User goal: while ralph is running inside tmux, the status bar shows repo name,
// branch, loop progress, and session uptime so the user can monitor build
// progress at a glance without looking at the TUI.
//
// updateTmuxStatusBar() is called on every tick and formats the content as:
//
//	[repo | branch | loop: N/M, uptime: HH:MM:SS]
//
// During hibernation it shows:
//
//	[repo | branch | RATE LIMITED 💤 MM:SS]
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

// --- Scenario 4: Session uptime shown ---

// TestBDD_TmuxStatusBar_ElapsedTimeShownOnTick
//
// Given: a model where 65 seconds of session uptime have elapsed (mocked time)
// When: a tick occurs
// Then: the tmux bar content includes "00:01:05"
func TestBDD_TmuxStatusBar_ElapsedTimeShownOnTick(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tui.SetTimeNowForTest(func() time.Time { return baseTime })
	defer tui.SetTimeNowForTest(time.Now)

	// Given: model created at baseTime (loopStartTime = baseTime)
	m, fakeBar := setupModelWithFakeBar(1, 3)

	// Advance time by 65 seconds
	tui.SetTimeNowForTest(func() time.Time { return baseTime.Add(65 * time.Second) })

	// When: tick
	triggerTick(m)

	// Then: elapsed time shown as HH:MM:SS
	if !strings.Contains(fakeBar.LastContent, "00:01:05") {
		t.Errorf("Expected 00:01:05 in tmux bar, got: %q", fakeBar.LastContent)
	}
}

// --- Scenario 5: Hibernating state shows RATE LIMITED label ---

// TestBDD_TmuxStatusBar_RateLimitedLabelShownDuringHibernate
//
// Given: a model in hibernating state (both loop and TUI state set)
// When: a tick occurs
// Then: the tmux bar content includes "RATE LIMITED"
func TestBDD_TmuxStatusBar_RateLimitedLabelShownDuringHibernate(t *testing.T) {
	// Given: model and loop both in hibernate state (matches real pipeline)
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
}

// --- Scenario 6: Hibernating state shows sleep emoji countdown ---

// TestBDD_TmuxStatusBar_SleepEmojiCountdownShownDuringHibernate
//
// Given: a model in hibernating state with 3 minutes 30 seconds remaining (mocked time)
// When: a tick occurs
// Then: the tmux bar content includes "💤 03:30"
func TestBDD_TmuxStatusBar_SleepEmojiCountdownShownDuringHibernate(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tui.SetTimeNowForTest(func() time.Time { return baseTime })
	defer tui.SetTimeNowForTest(time.Now)

	// Given: model and loop both in hibernate state with 3m30s remaining
	// hibernateUntil is baseTime + 3m30s; timeNow() is baseTime → 3m30s remain
	hibernateUntil := baseTime.Add(3*time.Minute + 30*time.Second)
	m, _ := setupReadyModelWithLoop(2, 5)
	m, _ = sendTuiMsg(m, tui.SendHibernate(hibernateUntil))

	fakeBar := &tui.FakeStatusBarForTest{}
	m.SetTmuxStatusBar(fakeBar)

	// When: tick (timeNow still at baseTime)
	triggerTick(m)

	// Then: countdown shows 💤 03:30
	if !strings.Contains(fakeBar.LastContent, "💤 03:30") {
		t.Errorf("Expected '💤 03:30' in tmux bar during hibernate, got: %q", fakeBar.LastContent)
	}
}

// --- Scenario 7: No update when tmux bar is inactive ---

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

// --- Scenario 8: Full status bar format ---

// TestBDD_TmuxStatusBar_FullFormatContainsAllFields
//
// Given: a model with loop at 3/7, git context set, 0 elapsed
// When: a tick occurs
// Then: the tmux bar uses the expected format with all fields present
func TestBDD_TmuxStatusBar_FullFormatContainsAllFields(t *testing.T) {
	m, fakeBar := setupModelWithFakeBar(3, 7)
	m.SetGitContext("ralph", "main")

	// When: tick
	triggerTick(m)

	// Then: all fields present in correct format
	content := fakeBar.LastContent
	if !strings.Contains(content, "loop:") {
		t.Errorf("Expected 'loop:' label in tmux bar, got: %q", content)
	}
	if !strings.Contains(content, "uptime:") {
		t.Errorf("Expected 'uptime:' label in tmux bar, got: %q", content)
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
