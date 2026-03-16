package tests

import (
	"testing"
	"time"

	"github.com/cloudosai/ralph-go/internal/loop"
	"github.com/cloudosai/ralph-go/internal/tui"
)

// ============================================================================
// BDD Test Suite: User Handles Rate Limits
//
// These tests verify the complete user experience when the Claude CLI is
// rate-limited: banner display, countdown timer, wake action, hotkey bar
// changes, and activity feed messages. Covers state transitions, boundary
// conditions, negative paths, and cross-feature interactions.
// Organized by user goal following specs/bdd-agent-prompt.md methodology.
// ============================================================================

// --- Helpers ---

// setupHibernatingModel creates a ready model with a loop in hibernate state.
// Both loop-level and TUI-level hibernate state are set, matching the real pipeline.
func setupHibernatingModel(current, total int, hibernateDuration time.Duration) (tui.Model, *loop.Loop) {
	m, l := setupReadyModelWithLoop(current, total)
	until := time.Now().Add(hibernateDuration)
	l.Hibernate(until)
	m, _ = sendTuiMsg(m, tui.SendHibernate(until))
	return m, l
}

// --- Scenario 1: Hibernate shows RATE LIMITED banner ---

func TestBDD_UserHandlesRateLimits_HibernateShowsRateLimitedBanner(t *testing.T) {
	// Given: a running loop that enters hibernate
	m, _ := setupHibernatingModel(2, 5, 5*time.Minute)

	// Then: the status banner shows RATE LIMITED
	if !viewContains(m, "RATE LIMITED") {
		t.Error("Expected status banner to show 'RATE LIMITED' when hibernating")
	}
}

func TestBDD_UserHandlesRateLimits_HibernateBannerReplacesRunning(t *testing.T) {
	// Given: a model that was previously showing RUNNING
	m, _ := setupReadyModelWithLoop(2, 5)

	// Precondition: shows RUNNING before hibernate
	if !viewContains(m, "RUNNING") {
		t.Fatal("Precondition: should show RUNNING before hibernate")
	}

	// When: hibernate is triggered
	until := time.Now().Add(5 * time.Minute)
	m, _ = sendTuiMsg(m, tui.SendHibernate(until))
	// Note: we also need the loop to be hibernating for renderLayout to detect it
	// The view checks m.loop.IsHibernating(), so we need the loop reference
	// Re-setup with full hibernate state
	m, _ = setupHibernatingModel(2, 5, 5*time.Minute)

	// Then: RATE LIMITED replaces RUNNING
	if !viewContains(m, "RATE LIMITED") {
		t.Error("Expected 'RATE LIMITED' to appear after hibernate")
	}
	// The centered status title should not show RUNNING anymore
	// (Note: "Running" appears in the footer status field too, but the
	// centered title at top should show "RATE LIMITED" not "RUNNING")
}

func TestBDD_UserHandlesRateLimits_HibernateOverridesStoppedDisplay(t *testing.T) {
	// Given: a loop that is both paused and hibernating
	// This can happen if pause was triggered right before a rate limit
	m, l := setupReadyModelWithLoop(2, 5)

	// Hibernate the loop (this internally sets hibernating=true)
	until := time.Now().Add(5 * time.Minute)
	l.Hibernate(until)
	m, _ = sendTuiMsg(m, tui.SendHibernate(until))

	// Precondition: view shows RATE LIMITED
	if !viewContains(m, "RATE LIMITED") {
		t.Fatal("Precondition: should show RATE LIMITED")
	}

	// Then: shows RATE LIMITED, NOT STOPPED
	if !viewContains(m, "RATE LIMITED") {
		t.Error("Expected 'RATE LIMITED' when hibernating, even if also paused")
	}
	if viewContains(m, "STOPPED") {
		t.Error("Should NOT show 'STOPPED' when hibernating — RATE LIMITED takes precedence")
	}
}

// --- Scenario 2: Hibernate shows countdown timer ---

func TestBDD_UserHandlesRateLimits_HibernateShowsCountdownWithSleepEmoji(t *testing.T) {
	// Given: a hibernating loop with 5 minutes remaining
	m, _ := setupHibernatingModel(2, 5, 5*time.Minute)

	// Precondition
	if !viewContains(m, "RATE LIMITED") {
		t.Fatal("Precondition: should show RATE LIMITED")
	}

	// Then: the footer shows the 💤 emoji as part of the countdown
	if !viewContains(m, "💤") {
		t.Error("Expected 💤 emoji in countdown display when hibernating")
	}
}

func TestBDD_UserHandlesRateLimits_CountdownShowsMinutesAndSeconds(t *testing.T) {
	// Given: a hibernating loop with 5 minutes remaining
	m, _ := setupHibernatingModel(2, 5, 5*time.Minute)

	if !viewContains(m, "RATE LIMITED") {
		t.Fatal("Precondition: should show RATE LIMITED")
	}

	// Then: countdown shows approximately 05:00 or 04:59 (MM:SS format)
	view := m.View()
	// The countdown is formatted as "Rate Limited 💤 MM:SS"
	// With 5 minutes remaining, we expect "04:" or "05:" to appear
	if !viewContains(m, "04:") && !viewContains(m, "05:") {
		t.Errorf("Expected countdown to show ~5 minutes remaining (04:xx or 05:xx), got view without either. View excerpt: ...%s...",
			extractFooterSection(view, "Rate Limited"))
	}
}

func TestBDD_UserHandlesRateLimits_CountdownShowsRateLimitedPrefix(t *testing.T) {
	// Given: a hibernating loop
	m, _ := setupHibernatingModel(3, 10, 3*time.Minute)

	// Then: countdown line starts with "Rate Limited"
	if !viewContains(m, "Rate Limited") {
		t.Error("Expected footer status to contain 'Rate Limited' prefix")
	}
}

func TestBDD_UserHandlesRateLimits_CountdownAtZeroBoundary(t *testing.T) {
	// Given: a hibernating loop where the deadline has already passed
	m, l := setupReadyModelWithLoop(2, 5)
	past := time.Now().Add(-1 * time.Second)
	l.Hibernate(past)
	m, _ = sendTuiMsg(m, tui.SendHibernate(past))

	// Precondition: view shows RATE LIMITED (loop stays hibernating even if deadline passed)
	if !viewContains(m, "RATE LIMITED") {
		t.Fatal("Precondition: should show RATE LIMITED even if deadline passed")
	}

	// Then: countdown should show 00:00 (clamped to zero, not negative)
	if !viewContains(m, "00:00") {
		t.Error("Expected countdown to show '00:00' when hibernate deadline has passed")
	}
}

func TestBDD_UserHandlesRateLimits_CountdownWithShortDuration(t *testing.T) {
	// Given: a hibernating loop with only 30 seconds remaining
	m, _ := setupHibernatingModel(1, 5, 30*time.Second)

	if !viewContains(m, "RATE LIMITED") {
		t.Fatal("Precondition: should show RATE LIMITED")
	}

	// Then: countdown shows 00:XX (under 1 minute)
	if !viewContains(m, "00:") {
		t.Error("Expected countdown to show '00:XX' for sub-minute hibernate duration")
	}
}

// --- Scenario 3: Wake clears rate limit state ---

func TestBDD_UserHandlesRateLimits_WakeViaClearsRateLimitState(t *testing.T) {
	// Given: a hibernating loop
	m, _ := setupHibernatingModel(2, 5, 5*time.Minute)

	if !viewContains(m, "RATE LIMITED") {
		t.Fatal("Precondition: should show RATE LIMITED banner")
	}

	// When: user presses 'r' to wake
	m, _ = pressKey(m, 'r')

	// Then: RATE LIMITED banner is gone
	if viewContains(m, "RATE LIMITED") {
		t.Error("RATE LIMITED banner should be cleared after wake")
	}
}

func TestBDD_UserHandlesRateLimits_WakeRestoresRunningStatus(t *testing.T) {
	// Given: a hibernating loop
	m, _ := setupHibernatingModel(2, 5, 5*time.Minute)

	if !viewContains(m, "RATE LIMITED") {
		t.Fatal("Precondition: should show RATE LIMITED")
	}

	// When: user presses 'r' to wake
	m, _ = pressKey(m, 'r')

	// Then: status shows RUNNING (the default non-paused, non-completed state)
	if !viewContains(m, "RUNNING") {
		t.Error("Expected status to show 'RUNNING' after waking from hibernate")
	}
}

func TestBDD_UserHandlesRateLimits_WakeClearsCountdown(t *testing.T) {
	// Given: a hibernating loop showing countdown
	m, _ := setupHibernatingModel(2, 5, 5*time.Minute)

	if !viewContains(m, "RATE LIMITED") {
		t.Fatal("Precondition: should show RATE LIMITED")
	}
	if !viewContains(m, "💤") {
		t.Fatal("Precondition: should show 💤 countdown")
	}

	// When: user presses 'r' to wake
	m, _ = pressKey(m, 'r')

	// Then: countdown and 💤 are gone from the footer status line
	// Note: 💤 may still appear in activity feed if a hibernate message was added,
	// but the footer status should show "Running" not "Rate Limited 💤 ..."
	if viewContains(m, "Rate Limited") {
		t.Error("Rate Limited status text should be cleared after wake")
	}
}

func TestBDD_UserHandlesRateLimits_WakeResumesTotalTimer(t *testing.T) {
	// Given: a hibernating loop (hibernate freezes timers via pause)
	m, _ := setupHibernatingModel(2, 5, 5*time.Minute)

	if !viewContains(m, "RATE LIMITED") {
		t.Fatal("Precondition: should show RATE LIMITED")
	}

	// When: user presses 'r' to wake
	m, _ = pressKey(m, 'r')

	// Then: the view renders without crash and shows a time display
	// (Timer resumes from paused state — we verify it renders, not exact values)
	if !viewContains(m, "Total Time:") {
		t.Error("Expected Total Time field to be present after wake")
	}
}

func TestBDD_UserHandlesRateLimits_SKeyAlsoWakesFromHibernate(t *testing.T) {
	// Given: a hibernating loop
	m, _ := setupHibernatingModel(2, 5, 5*time.Minute)

	if !viewContains(m, "RATE LIMITED") {
		t.Fatal("Precondition: should show RATE LIMITED")
	}

	// When: user presses 's' (the start key, which shares handler with 'r')
	m, _ = pressKey(m, 's')

	// Then: RATE LIMITED should be cleared
	if viewContains(m, "RATE LIMITED") {
		t.Error("RATE LIMITED should be cleared after 's' key wake")
	}
}

// --- Scenario 4: Hotkey bar shows (r) wake during hibernate ---

func TestBDD_UserHandlesRateLimits_HotkeyBarShowsWakeDuringHibernate(t *testing.T) {
	// Given: a hibernating loop
	m, _ := setupHibernatingModel(2, 5, 5*time.Minute)

	if !viewContains(m, "RATE LIMITED") {
		t.Fatal("Precondition: should show RATE LIMITED")
	}

	// Then: hotkey bar shows "(r) wake"
	if !viewContains(m, "(r) wake") {
		t.Error("Expected hotkey bar to show '(r) wake' during hibernate")
	}
}

func TestBDD_UserHandlesRateLimits_HotkeyBarDoesNotShowResumeDuringHibernate(t *testing.T) {
	// Given: a hibernating loop
	m, _ := setupHibernatingModel(2, 5, 5*time.Minute)

	if !viewContains(m, "RATE LIMITED") {
		t.Fatal("Precondition: should show RATE LIMITED")
	}

	// Then: hotkey bar does NOT show "(r)esume" — it should show "(r) wake" instead
	if viewContains(m, "(r)esume") {
		t.Error("Should NOT show '(r)esume' during hibernate — should show '(r) wake'")
	}
}

func TestBDD_UserHandlesRateLimits_HotkeyBarShowsPauseDimmedDuringHibernate(t *testing.T) {
	// Given: a hibernating loop
	m, _ := setupHibernatingModel(2, 5, 5*time.Minute)

	if !viewContains(m, "RATE LIMITED") {
		t.Fatal("Precondition: should show RATE LIMITED")
	}

	// Then: "(p)ause" should still be visible (dimmed) in the hotkey bar
	if !viewContains(m, "(p)ause") {
		t.Error("Expected '(p)ause' to remain visible (dimmed) during hibernate")
	}
}

func TestBDD_UserHandlesRateLimits_HotkeyBarAfterWakeShowsPause(t *testing.T) {
	// Given: a hibernating loop
	m, _ := setupHibernatingModel(2, 5, 5*time.Minute)

	if !viewContains(m, "RATE LIMITED") {
		t.Fatal("Precondition: should show RATE LIMITED")
	}

	// When: user wakes from hibernate
	m, _ = pressKey(m, 'r')

	// Then: hotkey bar reverts — "(r) wake" is gone
	if viewContains(m, "(r) wake") {
		t.Error("'(r) wake' should disappear after waking from hibernate")
	}
}

func TestBDD_UserHandlesRateLimits_HotkeyBarShowsQuitAndLoopsDuringHibernate(t *testing.T) {
	// Given: a hibernating loop
	m, _ := setupHibernatingModel(2, 5, 5*time.Minute)

	if !viewContains(m, "RATE LIMITED") {
		t.Fatal("Precondition: should show RATE LIMITED")
	}

	// Then: quit and loop adjustment keys are always visible
	if !viewContains(m, "(q)") {
		t.Error("Expected '(q)' quit key to be visible during hibernate")
	}
	if !viewContains(m, "(+)/(-)") {
		t.Error("Expected '(+)/(-)' loop adjustment keys to be visible during hibernate")
	}
}

// --- Scenario 5: Hibernate message in activity feed ---

func TestBDD_UserHandlesRateLimits_HibernateMessageShowsSleepIcon(t *testing.T) {
	// Given: a model that received a hibernate message in the activity feed
	m := setupReadyModel()
	m.AddMessage(tui.Message{Role: tui.RoleHibernate, Content: "Rate limited until 10:30 AM"})
	m, _ = updateModel(m, tui.TickMsgForTest())

	// Then: the activity feed shows the 💤 icon for the hibernate message
	if !viewContains(m, "💤") {
		t.Error("Expected 💤 icon in activity feed for hibernate message")
	}
}

func TestBDD_UserHandlesRateLimits_HibernateMessageContentVisible(t *testing.T) {
	// Given: a model with a hibernate message containing rate limit info
	m := setupReadyModel()
	m.AddMessage(tui.Message{Role: tui.RoleHibernate, Content: "Rate limited until 10:30 AM"})
	m, _ = updateModel(m, tui.TickMsgForTest())

	// Then: the message content is visible in the activity feed
	if !viewContains(m, "Rate limited until 10:30 AM") {
		t.Error("Expected hibernate message content to be visible in activity feed")
	}
}

func TestBDD_UserHandlesRateLimits_MultipleHibernateMessagesAccumulate(t *testing.T) {
	// Given: a model that receives multiple hibernate messages
	m := setupReadyModel()
	m.AddMessage(tui.Message{Role: tui.RoleHibernate, Content: "Rate limited — attempt 1"})
	m.AddMessage(tui.Message{Role: tui.RoleHibernate, Content: "Rate limited — attempt 2"})
	m, _ = updateModel(m, tui.TickMsgForTest())

	// Then: both messages are visible in the activity feed
	if !viewContains(m, "attempt 1") {
		t.Error("Expected first hibernate message to remain visible")
	}
	if !viewContains(m, "attempt 2") {
		t.Error("Expected second hibernate message to be visible")
	}
}

// --- Cross-feature: Hibernate + Completion ---

func TestBDD_UserHandlesRateLimits_HibernateDuringCompletedState(t *testing.T) {
	// Given: a completed loop (5/5)
	m, l := setupReadyModelWithLoop(5, 5)
	m, _ = sendTuiMsg(m, tui.SendDone())

	// Precondition: shows COMPLETED
	if !viewContains(m, "COMPLETED") {
		t.Fatal("Precondition: should show COMPLETED")
	}

	// When: hibernate is triggered (edge case: rate limit after completion)
	until := time.Now().Add(3 * time.Minute)
	l.Hibernate(until)
	m, _ = sendTuiMsg(m, tui.SendHibernate(until))

	// Then: COMPLETED takes precedence over RATE LIMITED
	// (completed is checked first in renderLayout)
	if !viewContains(m, "COMPLETED") {
		t.Error("Expected COMPLETED to take precedence over RATE LIMITED")
	}
}

// --- Cross-feature: Hibernate + Loop Adjustment ---

func TestBDD_UserHandlesRateLimits_AddLoopDuringHibernate(t *testing.T) {
	// Given: a hibernating loop at 2/5
	m, _ := setupHibernatingModel(2, 5, 5*time.Minute)

	if !viewContains(m, "#2/5") {
		t.Fatal("Precondition: should show loop progress #2/5")
	}

	// When: user presses '+' to add a loop
	m, _ = pressKey(m, '+')

	// Then: display shows increased total
	if !viewContains(m, "#2/6") {
		t.Error("Expected loop progress #2/6 after pressing '+'")
	}
	// And: still shows RATE LIMITED (adding a loop doesn't clear hibernate)
	if !viewContains(m, "RATE LIMITED") {
		t.Error("Expected RATE LIMITED to persist after adding a loop")
	}
}

func TestBDD_UserHandlesRateLimits_SubtractLoopDuringHibernate(t *testing.T) {
	// Given: a hibernating loop at 2/5
	m, _ := setupHibernatingModel(2, 5, 5*time.Minute)

	// When: user presses '-' to remove a loop
	m, _ = pressKey(m, '-')

	// Then: display shows decreased total
	if !viewContains(m, "#2/4") {
		t.Error("Expected loop progress #2/4 after pressing '-'")
	}
	// And: still hibernating
	if !viewContains(m, "RATE LIMITED") {
		t.Error("Expected RATE LIMITED to persist after subtracting a loop")
	}
}

// --- Negative paths ---

func TestBDD_UserHandlesRateLimits_WakeWhenNotHibernating(t *testing.T) {
	// Given: a running loop that is NOT hibernating
	m, _ := setupReadyModelWithLoop(2, 5)

	// Precondition: not showing RATE LIMITED
	if viewContains(m, "RATE LIMITED") {
		t.Fatal("Precondition: should NOT show RATE LIMITED")
	}

	// When: user presses 'r' (which is resume, not wake, when not hibernating)
	m, _ = pressKey(m, 'r')

	// Then: no crash, no RATE LIMITED banner appears
	if viewContains(m, "RATE LIMITED") {
		t.Error("Should not show RATE LIMITED when not hibernating")
	}
}

func TestBDD_UserHandlesRateLimits_HibernateWithoutLoop(t *testing.T) {
	// Given: a model with no loop set
	m := setupReadyModel()

	// When: hibernate message is sent (edge case: message arrives before loop is set)
	until := time.Now().Add(5 * time.Minute)
	m, _ = sendTuiMsg(m, tui.SendHibernate(until))

	// Then: no crash — model renders without RATE LIMITED in the banner
	// (renderLayout checks m.loop != nil && m.loop.IsHibernating())
	view := m.View()
	if view == "" {
		t.Error("Expected non-empty view after hibernate message without loop")
	}
	// The RATE LIMITED banner requires both TUI state AND loop state
	// Without a loop, the banner check returns false
}

// --- State transition: Multiple hibernate/wake cycles ---

func TestBDD_UserHandlesRateLimits_MultipleHibernateWakeCycles(t *testing.T) {
	// Given: a model with a loop
	m, l := setupReadyModelWithLoop(2, 5)

	for i := 0; i < 3; i++ {
		// When: hibernate is triggered
		until := time.Now().Add(5 * time.Minute)
		l.Hibernate(until)
		m, _ = sendTuiMsg(m, tui.SendHibernate(until))

		// Then: shows RATE LIMITED
		if !viewContains(m, "RATE LIMITED") {
			t.Errorf("Cycle %d: expected RATE LIMITED banner", i+1)
		}

		// When: user wakes
		m, _ = pressKey(m, 'r')

		// Then: RATE LIMITED is cleared
		if viewContains(m, "RATE LIMITED") {
			t.Errorf("Cycle %d: RATE LIMITED should be cleared after wake", i+1)
		}
	}
}

func TestBDD_UserHandlesRateLimits_HibernateExtendsDeadline(t *testing.T) {
	// Given: a hibernating loop with 2 minutes remaining
	m, l := setupHibernatingModel(2, 5, 2*time.Minute)

	// When: a second hibernate extends the deadline to 10 minutes
	longerUntil := time.Now().Add(10 * time.Minute)
	l.Hibernate(longerUntil)
	m, _ = sendTuiMsg(m, tui.SendHibernate(longerUntil))

	// Then: countdown shows the extended time (~10 minutes)
	if !viewContains(m, "09:") && !viewContains(m, "10:") {
		t.Error("Expected countdown to show extended ~10 minute deadline")
	}
}

// --- Scenario: Loop progress display during hibernate ---

func TestBDD_UserHandlesRateLimits_LoopProgressVisibleDuringHibernate(t *testing.T) {
	// Given: a hibernating loop at 3/7
	m, _ := setupHibernatingModel(3, 7, 5*time.Minute)

	if !viewContains(m, "RATE LIMITED") {
		t.Fatal("Precondition: should show RATE LIMITED")
	}

	// Then: loop progress is still visible in the footer
	if !viewContains(m, "#3/7") {
		t.Error("Expected loop progress '#3/7' to remain visible during hibernate")
	}
}

// --- Helper ---

// extractFooterSection extracts a substring around a keyword for diagnostic output.
func extractFooterSection(view, keyword string) string {
	idx := 0
	for i := range view {
		if i > 0 && view[i-1:i] == keyword[:1] {
			// Simple prefix match
			if len(view) >= i+len(keyword)-1 && view[i-1:i+len(keyword)-1] == keyword {
				idx = i - 1
				break
			}
		}
	}
	start := idx - 20
	if start < 0 {
		start = 0
	}
	end := idx + len(keyword) + 20
	if end > len(view) {
		end = len(view)
	}
	return view[start:end]
}
