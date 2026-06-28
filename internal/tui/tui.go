package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cloudosai/ralph-go/internal/loop"
	"github.com/cloudosai/ralph-go/internal/stats"
	"github.com/cloudosai/ralph-go/internal/tmux"
)

// Minimum terminal dimensions for proper rendering
const (
	minWidth  = 40
	minHeight = 15
)

// timeNow is the time source used for seasonal checks and elapsed time calculations (overridable in tests)
var timeNow = time.Now

// SetTimeNowForTest overrides the time source for testing (seasonal behavior and elapsed time)
func SetTimeNowForTest(fn func() time.Time) {
	timeNow = fn
}

// tmuxBarUpdater is the interface for managing the tmux status bar (production and test)
type tmuxBarUpdater interface {
	IsActive() bool
	Update(string)
	Restore()
}

// FakeStatusBarForTest is a test fake for the tmux status bar that records Update calls.
// Inject it via SetTmuxStatusBar to observe what content the TUI sends to tmux.
type FakeStatusBarForTest struct {
	LastContent string
}

func (f *FakeStatusBarForTest) IsActive() bool        { return true }
func (f *FakeStatusBarForTest) Update(content string) { f.LastContent = content }
func (f *FakeStatusBarForTest) Restore()              {}

// isOctober returns true if the current month is October
func isOctober() bool {
	return timeNow().Month() == time.October
}

// Color palette matching Python visualizer (Tokyo Night theme)
var (
	colorBlue      = lipgloss.Color("#7AA2F7")
	colorPurple    = lipgloss.Color("#BB9AF7")
	colorGreen     = lipgloss.Color("#9ECE6A")
	colorDimGray   = lipgloss.Color("#565F89")
	colorLightGray = lipgloss.Color("#C0CAF5")
colorRed       = lipgloss.Color("#F7768E")
	colorOrange    = lipgloss.Color("#FF9E64")
)

// MessageRole represents the type of message sender
type MessageRole string

const (
	RoleAssistant   MessageRole = "assistant"
	RoleTool        MessageRole = "tool"
	RoleUser        MessageRole = "user"
	RoleSystem      MessageRole = "system"
	RoleLoop        MessageRole = "loop"
	RoleLoopStopped MessageRole = "loop_stopped"
	RoleHibernate   MessageRole = "hibernate"
	RoleThinking    MessageRole = "thinking"
)

// Message represents a single activity message in the feed.
// For RoleTool messages the ACP-modeled fields (ToolUseID, Kind, Status) let
// the row render a kind icon + lifecycle status glyph and be mutated in place
// when the tool finishes.
type Message struct {
	Role      MessageRole
	Content   string
	ToolUseID string        // correlation key for in-place status updates (RoleTool)
	Kind      string        // ACP tool kind: read/edit/execute/search/fetch/think/...
	Status    string        // ACP tool status: in_progress/completed/failed/pending
	StartedAt time.Time     // when an in_progress tool row was added (TUI clock)
	Elapsed   time.Duration // wall-clock duration once the tool completed/failed
}

// PlanItem mirrors parser.PlanItem with plain-string status so the tui package
// stays free of a parser import (matching how Message.Kind/Status are kept as
// plain strings). It is one entry of the agent's TodoWrite-authored plan.
type PlanItem struct {
	Content string
	Status  string // "pending" | "in_progress" | "completed"
}

// spinnerFrames animates in_progress tool rows, advanced once per tick.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// formatToolDuration renders a compact elapsed time, e.g. "420ms", "1.4s",
// "2m3s".
func formatToolDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
}

// toolStatusGlyph returns the leading lifecycle glyph for a tool row.
func toolStatusGlyph(status string) string {
	switch status {
	case "in_progress":
		return "◐"
	case "completed":
		return "✓"
	case "failed":
		return "✗"
	case "pending":
		return "○"
	default:
		return " "
	}
}

// toolKindIcon returns the icon for an ACP tool kind.
func toolKindIcon(kind string) string {
	switch kind {
	case "read":
		return "📖"
	case "edit":
		return "✏️"
	case "delete":
		return "🗑️"
	case "move":
		return "🔀"
	case "search":
		return "🔎"
	case "execute":
		return "⚡"
	case "fetch":
		return "🌐"
	case "think":
		return "💭"
	default:
		return "🔧"
	}
}

// GetIcon returns the emoji icon for this message's role
func (m Message) GetIcon() string {
	switch m.Role {
	case RoleAssistant:
		if isOctober() {
			return "👻"
		}
		return "🤖"
	case RoleTool:
		return "🔧"
	case RoleUser:
		return "📝"
	case RoleSystem:
		return "💰"
	case RoleLoop:
		return "🚀"
	case RoleLoopStopped:
		return "🛑"
	case RoleHibernate:
		return "💤"
	case RoleThinking:
		return "💭"
	default:
		return "📝"
	}
}

// GetStyle returns the lipgloss style for this message's role
func (m Message) GetStyle() lipgloss.Style {
	switch m.Role {
	case RoleAssistant:
		return lipgloss.NewStyle().Bold(true).Foreground(colorBlue)
	case RoleTool:
		// Color the tool row by lifecycle status: failed→red, completed→green,
		// running/other→purple.
		switch m.Status {
		case "failed":
			return lipgloss.NewStyle().Bold(true).Foreground(colorRed)
		case "completed":
			return lipgloss.NewStyle().Bold(true).Foreground(colorGreen)
		default:
			return lipgloss.NewStyle().Bold(true).Foreground(colorPurple)
		}
	case RoleUser:
		return lipgloss.NewStyle().Foreground(colorDimGray)
	case RoleSystem:
		return lipgloss.NewStyle().Foreground(colorGreen)
	case RoleLoop:
		return lipgloss.NewStyle().Bold(true).Foreground(colorPurple)
	case RoleLoopStopped:
		return lipgloss.NewStyle().Bold(true).Foreground(colorRed)
	case RoleHibernate:
		return lipgloss.NewStyle().Bold(true).Foreground(colorOrange)
	case RoleThinking:
		return lipgloss.NewStyle().Italic(true).Foreground(colorDimGray)
	default:
		return lipgloss.NewStyle().Foreground(colorDimGray)
	}
}

// Model represents the TUI application state
type Model struct {
	ready          bool
	viewportReady  bool
	width          int
	height         int
	quitting       bool
	completed      bool // whether the loop has finished all iterations
	messages       []Message
	maxMessages    int
	spinnerFrame    int // advances each tick to animate in_progress rows
	inProgressTools int // count of tool rows currently in_progress
	stats          *stats.TokenStats
	currentLoop    int
	totalLoops     int
	currentTask    string // Current task (e.g., "#6 Change the lib/gold into lib/silver")
	completedTasks int    // Number of completed tasks from plan
	totalTasks     int    // Total number of tasks from plan
	plan           []PlanItem // Agent's TodoWrite-authored plan (ACP plan panel)
	currentMode    string // Current mode display ("Planning", "Building", or "")
	startTime      time.Time
	baseElapsed    time.Duration // elapsed time from previous sessions
	timerPaused    bool          // whether elapsed time tracking is paused
	pausedElapsed  time.Duration // elapsed time when paused (for display)
	// Per-loop tracking for tmux status bar (spec: stats should be about current loop)
	loopTotalTokens   int64         // tokens accumulated in the current loop iteration
	loopStartTime     time.Time     // when the current loop iteration started
	loopBaseElapsed   time.Duration // per-loop elapsed from before pause within same loop
	loopTimerPaused   bool          // whether per-loop timer is paused
	loopPausedElapsed time.Duration // per-loop elapsed at time of pause
	thinkingViewport  viewport.Model // left pane (2:1): thinking/assistant narrative, word-wrapped
	toolViewport      viewport.Model // right pane (1:1): tool-use rows + plan panel
	activityHeight    int
	footerHeight      int
	msgChan           <-chan Message
	doneChan          <-chan struct{}
	loop              *loop.Loop
	tmuxBar           tmuxBarUpdater
	hibernating       bool      // whether loop is hibernating due to rate limit
	hibernateUntil    time.Time // when rate limit resets
	repoName          string    // git repo name for tmux status bar
	branchName        string    // git branch name for tmux status bar
}

// NewModel creates and returns a new initialized Model
func NewModel() Model {
	now := timeNow()
	return Model{
		ready:          false,
		width:          0,
		height:         0,
		quitting:       false,
		messages:       []Message{},
		maxMessages:    100000,
		stats:          stats.NewTokenStats(),
		currentLoop:    0,
		totalLoops:     0,
		startTime:      now,
		loopStartTime:  now,
		activityHeight: 0,
		footerHeight:   11,
	}
}

// NewModelWithChannels creates a Model with external message channels for integration
func NewModelWithChannels(msgChan <-chan Message, doneChan <-chan struct{}) Model {
	m := NewModel()
	m.msgChan = msgChan
	m.doneChan = doneChan
	return m
}

// SetStats sets the stats object (for loading persisted stats)
func (m *Model) SetStats(s *stats.TokenStats) {
	m.stats = s
}

// SetLoopProgress updates the loop progress display
func (m *Model) SetLoopProgress(current, total int) {
	m.currentLoop = current
	m.totalLoops = total
}

// SetLoop sets the loop reference for pause/resume control
func (m *Model) SetLoop(l *loop.Loop) {
	m.loop = l
}

// SetBaseElapsed sets the elapsed time from previous sessions
func (m *Model) SetBaseElapsed(d time.Duration) {
	m.baseElapsed = d
}

// SetTmuxStatusBar sets the tmux status bar manager for live tmux status updates
func (m *Model) SetTmuxStatusBar(sb tmuxBarUpdater) {
	m.tmuxBar = sb
}

// SetGitContext sets the repo and branch names for the tmux status bar
func (m *Model) SetGitContext(repo, branch string) {
	m.repoName = repo
	m.branchName = branch
}

// SetCompletedTasks sets the completed/total task counts from the implementation plan
func (m *Model) SetCompletedTasks(completed, total int) {
	m.completedTasks = completed
	m.totalTasks = total
}

// SetCurrentMode sets the current mode display ("Planning", "Building", or "")
func (m *Model) SetCurrentMode(mode string) {
	m.currentMode = mode
}

// SetCurrentTask sets the initial current task display value
func (m *Model) SetCurrentTask(task string) {
	m.currentTask = task
}

// getElapsed returns the current total elapsed time
func (m Model) getElapsed() time.Duration {
	if m.timerPaused {
		return m.pausedElapsed
	}
	return m.baseElapsed + timeNow().Sub(m.startTime)
}

// getLoopElapsed returns the elapsed time for the current loop iteration
func (m Model) getLoopElapsed() time.Duration {
	if m.loopTimerPaused {
		return m.loopPausedElapsed
	}
	return m.loopBaseElapsed + timeNow().Sub(m.loopStartTime)
}

// AddMessage adds a message to the activity feed
func (m *Model) AddMessage(msg Message) {
	if msg.Role == RoleTool && msg.Status == "in_progress" {
		if msg.StartedAt.IsZero() {
			msg.StartedAt = timeNow()
		}
		m.inProgressTools++
	}
	m.messages = append(m.messages, msg)
	if len(m.messages) > m.maxMessages {
		// Keep the in_progress counter correct if we evict a still-running row.
		if evicted := m.messages[0]; evicted.Role == RoleTool && evicted.Status == "in_progress" && m.inProgressTools > 0 {
			m.inProgressTools--
		}
		m.messages = m.messages[1:]
	}
}

// tickMsg is sent periodically to update the display
type tickMsg time.Time

// newMessageMsg is sent when a new message is received from the channel
type newMessageMsg Message

// loopUpdateMsg is sent to update loop progress
type loopUpdateMsg struct {
	current int
	total   int
}

// statsUpdateMsg is sent to update stats
type statsUpdateMsg struct {
	stats *stats.TokenStats
}

// taskUpdateMsg is sent to update the current IMPLEMENTATION_PLAN.md task
type taskUpdateMsg struct {
	task string
}

// toolStatusUpdateMsg is sent to flip an existing tool row's lifecycle status
// (e.g. in_progress → completed/failed) by matching its tool_use ID.
type toolStatusUpdateMsg struct {
	toolUseID string
	status    string
}

// modeUpdateMsg is sent to update the current mode display
type modeUpdateMsg struct {
	mode string
}

// planUpdateMsg replaces the agent's plan (a full-list TodoWrite snapshot).
type planUpdateMsg struct {
	items []PlanItem
}

// completedTasksUpdateMsg is sent to update the completed/total task counts
type completedTasksUpdateMsg struct {
	completed int
	total     int
}

// loopStartedMsg is sent when a new loop iteration begins (resets per-loop stats)
type loopStartedMsg struct{}

// loopStatsUpdateMsg is sent to update per-loop token count
type loopStatsUpdateMsg struct {
	totalTokens int64
}

// doneMsg is sent when processing is complete
type doneMsg struct{}

// hibernateMsg is sent when rate limit is detected
type hibernateMsg struct {
	until time.Time
}

// loopRefMsg is sent to update the loop reference (e.g., when transitioning between plan and build phases)
type loopRefMsg struct {
	loop *loop.Loop
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tea.ClearScreen, tickCmd()}

	// If we have channels, start listening
	if m.msgChan != nil {
		cmds = append(cmds, waitForMessage(m.msgChan))
	}
	if m.doneChan != nil {
		cmds = append(cmds, waitForDone(m.doneChan))
	}

	return tea.Batch(cmds...)
}

// tickCmd creates a tick command for periodic updates
func tickCmd() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// waitForMessage creates a command to wait for messages from the channel
func waitForMessage(ch <-chan Message) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return doneMsg{}
		}
		return newMessageMsg(msg)
	}
}

// waitForDone creates a command to wait for the done signal
func waitForDone(ch <-chan struct{}) tea.Cmd {
	return func() tea.Msg {
		<-ch
		return doneMsg{}
	}
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.activityHeight = m.height - m.footerHeight - 2 // account for borders
		m.ready = true

		// If terminal is too small, skip viewport setup
		if m.width < minWidth || m.height < minHeight {
			return m, nil
		}

		// Split the activity area 2:1 — a wide "thinking" pane and a narrow
		// "tool use" pane. Each box's outer width plus its rounded border (+2)
		// sums to m.width so the row fills the terminal exactly.
		leftStyleWidth := max((m.width-4)*2/3, 1)
		rightStyleWidth := max((m.width-4)-leftStyleWidth, 1)
		leftVpWidth := max(leftStyleWidth-4, 1)
		rightVpWidth := max(rightStyleWidth-4, 1)
		vpHeight := max(m.activityHeight-2, 1)

		// Initialize or update both viewports
		if !m.viewportReady {
			m.thinkingViewport = viewport.New(leftVpWidth, vpHeight)
			m.toolViewport = viewport.New(rightVpWidth, vpHeight)
			m.viewportReady = true
			m.thinkingViewport.SetContent(m.renderThinkingContent())
			m.toolViewport.SetContent(m.renderToolContent())
			m.thinkingViewport.GotoBottom()
			m.toolViewport.GotoBottom()
		} else {
			m.thinkingViewport.Width = leftVpWidth
			m.thinkingViewport.Height = vpHeight
			m.toolViewport.Width = rightVpWidth
			m.toolViewport.Height = vpHeight
			// Re-wrap content to the new widths.
			m.thinkingViewport.SetContent(m.renderThinkingContent())
			m.toolViewport.SetContent(m.renderToolContent())
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			// Persist total elapsed time to stats before quitting
			if m.stats != nil {
				var totalElapsed time.Duration
				if m.timerPaused {
					totalElapsed = m.pausedElapsed
				} else {
					totalElapsed = m.baseElapsed + timeNow().Sub(m.startTime)
				}
				m.stats.SetTotalElapsedNs(totalElapsed.Nanoseconds())
			}
			// Restore tmux status bar to its original state
			if m.tmuxBar != nil {
				m.tmuxBar.Restore()
			}
			m.quitting = true
			return m, tea.Quit
		case "p":
			// Pause the loop - freeze elapsed time (both total and per-loop)
			if m.loop != nil {
				if !m.timerPaused {
					m.pausedElapsed = m.baseElapsed + timeNow().Sub(m.startTime)
					m.timerPaused = true
				}
				if !m.loopTimerPaused {
					m.loopPausedElapsed = m.loopBaseElapsed + timeNow().Sub(m.loopStartTime)
					m.loopTimerPaused = true
				}
				m.loop.Pause()
			}
			return m, nil
		case "r", "s":
			// Resume the loop - resume elapsed time from where we paused (both total and per-loop)
			// Also handles resuming after completion when new loops were added via '+'
			// 's' key is the "start" shortcut shown when completed with pending loops
			// When hibernating, 'r' wakes from hibernate early
			if m.loop != nil {
				// Handle hibernate wake first
				if m.loop.IsHibernating() {
					m.loop.Wake()
					m.hibernating = false
					// Resume timers when waking from hibernate
					if m.timerPaused {
						m.baseElapsed = m.pausedElapsed
						m.startTime = timeNow()
						m.timerPaused = false
					}
					if m.loopTimerPaused {
						m.loopBaseElapsed = m.loopPausedElapsed
						m.loopStartTime = timeNow()
						m.loopTimerPaused = false
					}
					return m, nil
				}
				if m.timerPaused {
					m.baseElapsed = m.pausedElapsed
					m.startTime = timeNow()
					m.timerPaused = false
				}
				if m.loopTimerPaused {
					m.loopBaseElapsed = m.loopPausedElapsed
					m.loopStartTime = timeNow()
					m.loopTimerPaused = false
				}
				// Clear completed state when resuming with pending loops
				if m.completed && m.totalLoops > m.currentLoop {
					m.completed = false
				}
				m.loop.Resume()
			}
			return m, nil
		case "+":
			// Add a loop iteration (works even after completion to enable extending loops)
			if m.loop != nil {
				m.totalLoops++
				m.loop.SetIterations(m.totalLoops)
			}
			return m, nil
		case "-":
			// Subtract a loop iteration (floor: can't go below current loop)
			if m.loop != nil && m.totalLoops > m.currentLoop {
				m.totalLoops--
				m.loop.SetIterations(m.totalLoops)
			}
			return m, nil
		}

	case tickMsg:
		// Advance the spinner so in_progress rows and the thinking indicator animate.
		m.spinnerFrame++
		// Update viewport content and schedule next tick
		// Note: we do NOT call GotoBottom() here — that would override the user's
		// scroll position every 250ms, making the viewport effectively unscrollable.
		// GotoBottom() is only called on viewport init and when new messages arrive.
		if m.viewportReady {
			m.thinkingViewport.SetContent(m.renderThinkingContent())
			m.toolViewport.SetContent(m.renderToolContent())
			// The tool pane auto-follows the latest activity; the thinking pane
			// preserves the user's scroll position (no GotoBottom here).
			m.toolViewport.GotoBottom()
		}
		m.updateTmuxStatusBar()
		return m, tickCmd()

	case newMessageMsg:
		m.AddMessage(Message(msg))
		if m.viewportReady {
			m.thinkingViewport.SetContent(m.renderThinkingContent())
			m.toolViewport.SetContent(m.renderToolContent())
			m.thinkingViewport.GotoBottom()
			m.toolViewport.GotoBottom()
		}
		// Continue listening for more messages
		if m.msgChan != nil {
			cmds = append(cmds, waitForMessage(m.msgChan))
		}
		return m, tea.Batch(cmds...)

	case loopUpdateMsg:
		m.currentLoop = msg.current
		m.totalLoops = msg.total
		return m, nil

	case statsUpdateMsg:
		m.stats = msg.stats
		return m, nil

	case taskUpdateMsg:
		m.currentTask = msg.task
		return m, nil

	case toolStatusUpdateMsg:
		// Find the most recent tool row with this ID and update its status
		// in place. No-op if not found (e.g. row evicted by maxMessages cap).
		for i := len(m.messages) - 1; i >= 0; i-- {
			if m.messages[i].Role == RoleTool && m.messages[i].ToolUseID == msg.toolUseID {
				// Only the first resolution of a running row records timing and
				// drops the in_progress count.
				if m.messages[i].Status == "in_progress" {
					if m.inProgressTools > 0 {
						m.inProgressTools--
					}
					if !m.messages[i].StartedAt.IsZero() {
						m.messages[i].Elapsed = timeNow().Sub(m.messages[i].StartedAt)
					}
				}
				m.messages[i].Status = msg.status
				break
			}
		}
		if m.viewportReady {
			m.thinkingViewport.SetContent(m.renderThinkingContent())
			m.toolViewport.SetContent(m.renderToolContent())
			m.toolViewport.GotoBottom()
		}
		return m, nil

	case modeUpdateMsg:
		m.currentMode = msg.mode
		return m, nil

	case planUpdateMsg:
		// Full-list replace. Derive the footer counters from the plan so the
		// panel and footer share a single source of truth.
		m.plan = msg.items
		completed, current := 0, ""
		for _, it := range msg.items {
			switch it.Status {
			case "completed":
				completed++
			case "in_progress":
				if current == "" {
					current = it.Content
				}
			}
		}
		m.completedTasks = completed
		m.totalTasks = len(msg.items)
		if current != "" {
			m.currentTask = current
		}
		if m.viewportReady {
			m.thinkingViewport.SetContent(m.renderThinkingContent())
			m.toolViewport.SetContent(m.renderToolContent())
			m.toolViewport.GotoBottom()
		}
		return m, nil

	case completedTasksUpdateMsg:
		m.completedTasks = msg.completed
		m.totalTasks = msg.total
		return m, nil

	case loopStartedMsg:
		// New loop iteration started — reset per-loop timer and tokens
		m.loopStartTime = timeNow()
		m.loopBaseElapsed = 0
		m.loopTimerPaused = false
		m.loopPausedElapsed = 0
		m.loopTotalTokens = 0
		return m, nil

	case loopStatsUpdateMsg:
		m.loopTotalTokens = msg.totalTokens
		return m, nil

	case doneMsg:
		// Processing is done — freeze both timers and mark as completed
		m.completed = true
		if !m.timerPaused {
			m.pausedElapsed = m.baseElapsed + timeNow().Sub(m.startTime)
			m.timerPaused = true
		}
		if !m.loopTimerPaused {
			m.loopPausedElapsed = m.loopBaseElapsed + timeNow().Sub(m.loopStartTime)
			m.loopTimerPaused = true
		}
		return m, nil

	case hibernateMsg:
		m.hibernating = true
		m.hibernateUntil = msg.until
		return m, nil

	case loopRefMsg:
		m.loop = msg.loop
		return m, nil
	}

	// Handle viewport scrolling — scroll keys drive the thinking pane (the tool
	// pane auto-follows the latest activity).
	m.thinkingViewport, cmd = m.thinkingViewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// toolElapsed returns the formatted elapsed time for a tool row: a live
// running time while in_progress, the final duration once resolved, or "" if
// no start time was recorded.
func (m Model) toolElapsed(msg Message) string {
	if msg.StartedAt.IsZero() {
		return ""
	}
	if msg.Status == "in_progress" {
		return formatToolDuration(timeNow().Sub(msg.StartedAt))
	}
	if msg.Elapsed > 0 {
		return formatToolDuration(msg.Elapsed)
	}
	return ""
}

// planPanelMaxItems caps how many plan entries are shown before collapsing the
// remainder into a "…and N more" line, keeping the panel compact.
const planPanelMaxItems = 8

// renderPlanPanel renders the agent's TodoWrite plan as a compact checklist:
// ✓ completed (green, dim), spinner/◐ in_progress (purple), ○ pending (dim).
// Returns "" when there is no plan.
func (m Model) renderPlanPanel() string {
	if len(m.plan) == 0 {
		return ""
	}
	dimStyle := lipgloss.NewStyle().Foreground(colorDimGray)
	doneStyle := lipgloss.NewStyle().Foreground(colorGreen).Strikethrough(true)
	currentStyle := lipgloss.NewStyle().Bold(true).Foreground(colorPurple)
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(colorPurple)

	completed := 0
	for _, it := range m.plan {
		if it.Status == "completed" {
			completed++
		}
	}

	var lines []string
	lines = append(lines, headerStyle.Render(fmt.Sprintf("📋 Plan (%d/%d)", completed, len(m.plan))))
	for i, it := range m.plan {
		if i >= planPanelMaxItems {
			lines = append(lines, dimStyle.Render(fmt.Sprintf("   …and %d more", len(m.plan)-planPanelMaxItems)))
			break
		}
		var glyph, text string
		switch it.Status {
		case "completed":
			glyph = "✓"
			text = doneStyle.Render(it.Content)
		case "in_progress":
			glyph = spinnerFrames[m.spinnerFrame%len(spinnerFrames)]
			text = currentStyle.Render(it.Content)
		default:
			glyph = "○"
			text = dimStyle.Render(it.Content)
		}
		lines = append(lines, fmt.Sprintf("  %s %s", glyph, text))
	}
	return strings.Join(lines, "\n")
}

// renderNarrativeLine renders one non-tool message for the thinking pane as a
// hanging-indent block: the role icon sits in a fixed gutter and the styled
// content is word-wrapped to the remaining width, so long thinking/assistant
// text is shown in full instead of being clipped to a single line.
func renderNarrativeLine(msg Message, width int) string {
	bodyWidth := max(width-3, 1)
	body := msg.GetStyle().Width(bodyWidth).Render(msg.Content)
	gutter := lipgloss.NewStyle().Width(3).Render(msg.GetIcon())
	return lipgloss.JoinHorizontal(lipgloss.Top, gutter, body)
}

// renderThinkingContent renders the left (2/3) pane: the thinking/assistant
// narrative — every non-tool message word-wrapped to the pane width — plus the
// idle "thinking…" indicator. Tool-use rows live in the right pane instead.
func (m Model) renderThinkingContent() string {
	dimStyle := lipgloss.NewStyle().Foreground(colorDimGray)

	// Nothing has happened yet: show the waiting placeholder (no idle dots),
	// matching the pre-split behavior.
	if len(m.messages) == 0 {
		return dimStyle.Render("Waiting for activity...")
	}

	width := m.thinkingViewport.Width
	if width < 1 {
		width = max(m.width-4, 1)
	}

	var lines []string
	for _, msg := range m.messages {
		if msg.Role == RoleTool {
			continue // tool rows render in the right pane
		}
		lines = append(lines, renderNarrativeLine(msg, width))
		lines = append(lines, "") // blank line between messages
	}

	// Thinking/waiting indicator: when the loop is live but nothing is
	// executing, the model is deciding its next step. Animate dots so the
	// gap between steps reads as active rather than stalled.
	if m.inProgressTools == 0 && !m.completed && !m.hibernating && !m.quitting && !m.timerPaused {
		dots := strings.Repeat(".", 1+(m.spinnerFrame%3))
		lines = append(lines, dimStyle.Italic(true).Render("💭 thinking"+dots))
	}

	return strings.Join(lines, "\n")
}

// renderToolContent renders the right (1/3) pane: the agent's plan panel pinned
// at the top followed by the tool-use rows, each rendered exactly as before the
// split (status glyph + kind icon + title + dim elapsed time).
func (m Model) renderToolContent() string {
	planPanel := m.renderPlanPanel()
	dimStyle := lipgloss.NewStyle().Foreground(colorDimGray)

	var lines []string
	for _, msg := range m.messages {
		if msg.Role != RoleTool {
			continue
		}
		var line string
		if msg.Status != "" {
			// ACP-modeled tool row: status glyph + kind icon + styled title +
			// dim elapsed time. in_progress rows show an animated spinner and a
			// live-updating timer; resolved rows show their final duration.
			glyph := toolStatusGlyph(msg.Status)
			if msg.Status == "in_progress" {
				glyph = spinnerFrames[m.spinnerFrame%len(spinnerFrames)]
			}
			line = fmt.Sprintf("%s %s %s", glyph, toolKindIcon(msg.Kind), msg.GetStyle().Render(msg.Content))
			if dur := m.toolElapsed(msg); dur != "" {
				line += " " + dimStyle.Render("("+dur+")")
			}
		} else {
			// Status-less tool message: icon + styled content.
			line = fmt.Sprintf("%s %s", msg.GetIcon(), msg.GetStyle().Render(msg.Content))
		}
		lines = append(lines, line)
		lines = append(lines, "") // blank line between rows
	}

	content := strings.Join(lines, "\n")
	if planPanel != "" {
		if content != "" {
			return planPanel + "\n\n" + content
		}
		return planPanel
	}
	return content
}

// View renders the UI
func (m Model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}

	if !m.ready {
		// Return empty string for a clean alt screen during init
		// instead of flashing unstyled "Initializing..." text
		return ""
	}

	// Show message if terminal is too small for the full layout
	if m.width < minWidth || m.height < minHeight {
		return fmt.Sprintf("\n  Terminal too small (%dx%d). Minimum: %dx%d\n",
			m.width, m.height, minWidth, minHeight)
	}

	// Render the main layout
	return m.renderLayout()
}

// renderLayout creates the full layout with activity panel and footer
func (m Model) renderLayout() string {
	// Check if loop is paused or completed
	isPaused := m.loop != nil && m.loop.IsPaused()
	isHibernating := m.loop != nil && m.loop.IsHibernating()

	// Choose colors based on state
	borderColor := colorBlue
	statusText := "RUNNING"
	if m.completed {
		borderColor = colorGreen
		statusText = "COMPLETED"
	} else if isHibernating {
		borderColor = colorOrange
		statusText = "RATE LIMITED"
	} else if isPaused {
		borderColor = colorRed
		statusText = "STOPPED"
	}

	// Split the activity area 2:1 — a wide "thinking" pane and a narrow
	// "tool use" pane. Each box's outer width plus its rounded border (+2) sums
	// to m.width so the joined row fills the terminal exactly.
	leftStyleWidth := max((m.width-4)*2/3, 1)
	rightStyleWidth := max((m.width-4)-leftStyleWidth, 1)

	paneStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2).
		Height(m.activityHeight)

	thinkingPane := paneStyle.Width(leftStyleWidth).Render(m.thinkingViewport.View())
	toolPane := paneStyle.Width(rightStyleWidth).Render(m.toolViewport.View())
	panes := lipgloss.JoinHorizontal(lipgloss.Top, thinkingPane, toolPane)

	// Centered status title at top
	statusTitle := lipgloss.NewStyle().
		Bold(true).
		Foreground(borderColor).
		Width(m.width - 2).
		Align(lipgloss.Center).
		Render(statusText)

	// Add centered status title above the split activity panes
	activityPanel := lipgloss.JoinVertical(
		lipgloss.Left,
		statusTitle,
		panes,
	)

	// Render footer panels
	footerContent := m.renderFooter()

	// Join activity and footer
	return lipgloss.JoinVertical(
		lipgloss.Left,
		activityPanel,
		footerContent,
	)
}

// renderFooter renders the two-panel footer with hotkey bar
func (m Model) renderFooter() string {
	// Calculate panel width (divide by 2, accounting for spacing)
	panelWidth := (m.width - 6) / 2

	// Panel styles
	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPurple).
		Padding(0, 1).
		Width(panelWidth).
		Height(m.footerHeight - 3) // Leave room for hotkey bar

	labelStyle := lipgloss.NewStyle().
		Foreground(colorBlue).
		Align(lipgloss.Right).
		Width(17)

	valueStyle := lipgloss.NewStyle().
		Foreground(colorLightGray)

	costStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorGreen)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorPurple)

	// Take a consistent snapshot of stats for display (avoids races with writer goroutine)
	snap := m.stats.Snapshot()

	// Usage & Cost panel
	usageCostContent := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render("Usage & Cost"),
		lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Total Tokens:"), valueStyle.Render(fmt.Sprintf(" %s", stats.FormatTokens(snap.TotalTokensCount)))),
		lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Input:"), valueStyle.Render(fmt.Sprintf(" %s", stats.FormatTokens(snap.InputTokens)))),
		lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Output:"), valueStyle.Render(fmt.Sprintf(" %s", stats.FormatTokens(snap.OutputTokens)))),
		lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Cache Write:"), valueStyle.Render(fmt.Sprintf(" %s", stats.FormatTokens(snap.CacheCreationTokens)))),
		lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Cache Read:"), valueStyle.Render(fmt.Sprintf(" %s", stats.FormatTokens(snap.CacheReadTokens)))),
		lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Total Cost:"), costStyle.Render(fmt.Sprintf(" $%.6f", snap.TotalCostUSD))),
	)
	usageCostPanel := panelStyle.Render(usageCostContent)

	// Loop Details panel
	loopDisplay := "#0/0"
	if m.totalLoops > 0 {
		loopDisplay = fmt.Sprintf("#%d/%d", m.currentLoop, m.totalLoops)
	}

	elapsed := m.getElapsed()
	hours := int(elapsed.Hours())
	minutes := int(elapsed.Minutes()) % 60
	seconds := int(elapsed.Seconds()) % 60
	timeDisplay := fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)

	// Status display
	isPaused := m.loop != nil && m.loop.IsPaused()
	isHibernating := m.loop != nil && m.loop.IsHibernating()
	statusText := "Running"
	statusStyle := valueStyle.Foreground(colorGreen)
	if m.completed {
		statusText = "Completed"
		statusStyle = valueStyle.Foreground(colorGreen)
	} else if isHibernating {
		// Show countdown timer when hibernating
		remaining := time.Until(m.hibernateUntil)
		if remaining < 0 {
			remaining = 0
		}
		mins := int(remaining.Minutes())
		secs := int(remaining.Seconds()) % 60
		statusText = fmt.Sprintf("Rate Limited 💤 %02d:%02d", mins, secs)
		statusStyle = valueStyle.Foreground(colorOrange)
	} else if isPaused {
		statusText = "Stopped"
		statusStyle = valueStyle.Foreground(colorRed)
	}

	// Current Mode display
	modeDisplay := " -"
	if m.currentMode != "" {
		modeDisplay = fmt.Sprintf(" %s", m.currentMode)
	}

	// Completed Tasks display
	completedDisplay := fmt.Sprintf(" %d/%d", m.completedTasks, m.totalTasks)

	// Current Task display
	taskDisplay := " -"
	if m.currentTask != "" {
		taskDisplay = fmt.Sprintf(" %s", m.currentTask)
	}

	loopDetailsContent := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render("Ralph Loop Details"),
		lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Loop:"), valueStyle.Render(fmt.Sprintf(" %s", loopDisplay))),
		lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Total Time:"), valueStyle.Render(fmt.Sprintf(" %s", timeDisplay))),
		lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Status:"), statusStyle.Render(fmt.Sprintf(" %s", statusText))),
		lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Completed Tasks:"), valueStyle.Render(completedDisplay)),
		lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Current Task:"), valueStyle.Render(taskDisplay)),
		lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Current Mode:"), valueStyle.Render(modeDisplay)),
	)
	loopDetailsPanel := panelStyle.Render(loopDetailsContent)

	// Join panels horizontally
	panels := lipgloss.JoinHorizontal(
		lipgloss.Top,
		usageCostPanel,
		loopDetailsPanel,
	)

	// Hotkey bar
	dimStyle := lipgloss.NewStyle().Foreground(colorDimGray)
	highlightStyle := lipgloss.NewStyle().Bold(true).Foreground(colorLightGray)

	quitKey := highlightStyle.Render("(q)")
	quitLabel := highlightStyle.Render("uit")
	pauseKey := dimStyle.Render("(p)ause")
	resumeKey := dimStyle.Render("(r)esume")
	loopsKey := highlightStyle.Render("(+)/(-)")
	loopsLabel := highlightStyle.Render(" # of loops")

	// Illuminate resume/start depending on state
	hasPendingLoops := m.completed && m.totalLoops > m.currentLoop
	if isHibernating {
		resumeKey = highlightStyle.Render("(r) wake")
		pauseKey = dimStyle.Render("(p)ause")
	} else if hasPendingLoops {
		resumeKey = highlightStyle.Render("(s)tart")
	} else if isPaused {
		resumeKey = highlightStyle.Render("(r)esume")
	} else if !m.completed {
		pauseKey = highlightStyle.Render("(p)ause")
	}

	hotkeyBar := lipgloss.NewStyle().
		Width(m.width - 2).
		Align(lipgloss.Left).
		PaddingLeft(1).
		Render(fmt.Sprintf("%s%s   %s   %s   %s%s", quitKey, quitLabel, resumeKey, pauseKey, loopsKey, loopsLabel))

	return lipgloss.JoinVertical(
		lipgloss.Left,
		panels,
		hotkeyBar,
	)
}

// updateTmuxStatusBar updates the tmux status-right bar with current loop stats
// (spec: stats should be about the current loop, not cumulative)
func (m Model) updateTmuxStatusBar() {
	if m.tmuxBar == nil || !m.tmuxBar.IsActive() {
		return
	}

	// If hibernating, show countdown instead of normal stats
	if m.hibernating {
		remaining := m.hibernateUntil.Sub(timeNow())
		if remaining < 0 {
			remaining = 0
		}
		mins := int(remaining.Minutes())
		secs := int(remaining.Seconds()) % 60
		hibernateDisplay := fmt.Sprintf("RATE LIMITED 💤 %02d:%02d", mins, secs)
		m.tmuxBar.Update(tmux.FormatStatusRight(m.repoName, m.branchName, hibernateDisplay, ""))
		return
	}

	loopDisplay := "0/0"
	if m.totalLoops > 0 {
		loopDisplay = fmt.Sprintf("%d/%d", m.currentLoop, m.totalLoops)
	}

	// Total session uptime
	elapsed := m.getElapsed()
	hours := int(elapsed.Hours())
	minutes := int(elapsed.Minutes()) % 60
	seconds := int(elapsed.Seconds()) % 60
	timeDisplay := fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)

	m.tmuxBar.Update(tmux.FormatStatusRight(m.repoName, m.branchName, loopDisplay, timeDisplay))
}

// SendMessage is a helper command to send a message to the TUI
func SendMessage(msg Message) tea.Cmd {
	return func() tea.Msg {
		return newMessageMsg(msg)
	}
}

// SendLoopUpdate is a helper command to update loop progress
func SendLoopUpdate(current, total int) tea.Cmd {
	return func() tea.Msg {
		return loopUpdateMsg{current: current, total: total}
	}
}

// SendStatsUpdate is a helper command to update stats
func SendStatsUpdate(s *stats.TokenStats) tea.Cmd {
	return func() tea.Msg {
		return statsUpdateMsg{stats: s}
	}
}

// SendTaskUpdate is a helper command to update the current task
func SendTaskUpdate(task string) tea.Cmd {
	return func() tea.Msg {
		return taskUpdateMsg{task: task}
	}
}

// SendToolStatusUpdate is a helper command to update a tool row's lifecycle
// status (completed/failed) by its tool_use ID.
func SendToolStatusUpdate(toolUseID, status string) tea.Cmd {
	return func() tea.Msg {
		return toolStatusUpdateMsg{toolUseID: toolUseID, status: status}
	}
}

// SendPlanUpdate is a helper command to replace the agent's plan (the panel +
// footer counters are derived from it).
func SendPlanUpdate(items []PlanItem) tea.Cmd {
	return func() tea.Msg {
		return planUpdateMsg{items: items}
	}
}

// SendModeUpdate is a helper command to update the current mode display
func SendModeUpdate(mode string) tea.Cmd {
	return func() tea.Msg {
		return modeUpdateMsg{mode: mode}
	}
}

// SendCompletedTasksUpdate is a helper command to update completed/total task counts
func SendCompletedTasksUpdate(completed, total int) tea.Cmd {
	return func() tea.Msg {
		return completedTasksUpdateMsg{completed: completed, total: total}
	}
}

// SendLoopStarted is a helper command to signal a new loop iteration has begun
func SendLoopStarted() tea.Cmd {
	return func() tea.Msg {
		return loopStartedMsg{}
	}
}

// SendLoopStatsUpdate is a helper command to update per-loop token count
func SendLoopStatsUpdate(totalTokens int64) tea.Cmd {
	return func() tea.Msg {
		return loopStatsUpdateMsg{totalTokens: totalTokens}
	}
}

// SendDone is a helper command to signal processing completion
func SendDone() tea.Cmd {
	return func() tea.Msg {
		return doneMsg{}
	}
}

// SendHibernate is a helper command to signal rate limit hibernate state
func SendHibernate(until time.Time) tea.Cmd {
	return func() tea.Msg {
		return hibernateMsg{until: until}
	}
}

// SendLoopRef is a helper command to update the loop reference in the TUI model.
// Used in plan-and-build mode to swap the loop when transitioning between phases.
func SendLoopRef(l *loop.Loop) tea.Cmd {
	return func() tea.Msg {
		return loopRefMsg{loop: l}
	}
}

// TickMsgForTest returns a tickMsg for use in tests
func TickMsgForTest() tea.Msg {
	return tickMsg(timeNow())
}

// WaitForMessageForTest exposes waitForMessage for BDD channel-flow tests.
// The channel must be pre-filled (buffered) so the command returns without blocking.
func WaitForMessageForTest(ch <-chan Message) tea.Cmd {
	return waitForMessage(ch)
}

// WaitForDoneForTest exposes waitForDone for BDD channel-flow tests.
// The channel must be pre-signaled (buffered) so the command returns without blocking.
func WaitForDoneForTest(ch <-chan struct{}) tea.Cmd {
	return waitForDone(ch)
}

// SetMaxMessagesForTest overrides the maxMessages cap for boundary testing.
func (m *Model) SetMaxMessagesForTest(n int) {
	m.maxMessages = n
}

// MessageCountForTest returns the current number of messages in the activity feed.
func (m *Model) MessageCountForTest() int {
	return len(m.messages)
}

