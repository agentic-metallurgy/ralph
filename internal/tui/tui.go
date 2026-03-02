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

// timeNow is the time source used for seasonal checks (overridable in tests)
var timeNow = time.Now

// SetTimeNowForTest overrides the time source for testing seasonal behavior
func SetTimeNowForTest(fn func() time.Time) {
	timeNow = fn
}

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
	colorBg        = lipgloss.Color("#1A1B26")
	colorRed       = lipgloss.Color("#F7768E")
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
)

// Message represents a single activity message in the feed
type Message struct {
	Role    MessageRole
	Content string
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
		return lipgloss.NewStyle().Bold(true).Foreground(colorPurple)
	case RoleUser:
		return lipgloss.NewStyle().Foreground(colorDimGray)
	case RoleSystem:
		return lipgloss.NewStyle().Foreground(colorGreen)
	case RoleLoop:
		return lipgloss.NewStyle().Bold(true).Foreground(colorPurple)
	case RoleLoopStopped:
		return lipgloss.NewStyle().Bold(true).Foreground(colorRed)
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
	stats          *stats.TokenStats
	currentLoop    int
	totalLoops     int
	activeAgents   int
	currentTask    string // Current task (e.g., "#6 Change the lib/gold into lib/silver")
	completedTasks int    // Number of completed tasks from plan
	totalTasks     int    // Total number of tasks from plan
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
	viewport          viewport.Model
	activityHeight    int
	footerHeight      int
	msgChan           <-chan Message
	doneChan          <-chan struct{}
	loop              *loop.Loop
	tmuxBar           *tmux.StatusBar
}

// NewModel creates and returns a new initialized Model
func NewModel() Model {
	now := time.Now()
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
func (m *Model) SetTmuxStatusBar(sb *tmux.StatusBar) {
	m.tmuxBar = sb
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
	return m.baseElapsed + time.Since(m.startTime)
}

// getLoopElapsed returns the elapsed time for the current loop iteration
func (m Model) getLoopElapsed() time.Duration {
	if m.loopTimerPaused {
		return m.loopPausedElapsed
	}
	return m.loopBaseElapsed + time.Since(m.loopStartTime)
}

// AddMessage adds a message to the activity feed
func (m *Model) AddMessage(msg Message) {
	m.messages = append(m.messages, msg)
	if len(m.messages) > m.maxMessages {
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

// agentUpdateMsg is sent to update active agent count
type agentUpdateMsg struct {
	count int
}

// taskUpdateMsg is sent to update the current IMPLEMENTATION_PLAN.md task
type taskUpdateMsg struct {
	task string
}

// modeUpdateMsg is sent to update the current mode display
type modeUpdateMsg struct {
	mode string
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

		// Guard viewport dimensions against going below 1
		vpWidth := max(m.width-4, 1)
		vpHeight := max(m.activityHeight-2, 1)

		// Initialize or update viewport
		if !m.viewportReady {
			m.viewport = viewport.New(vpWidth, vpHeight)
			m.viewport.SetContent(m.renderActivityContent())
			m.viewport.GotoBottom()
			m.viewportReady = true
		} else {
			m.viewport.Width = vpWidth
			m.viewport.Height = vpHeight
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
					totalElapsed = m.baseElapsed + time.Since(m.startTime)
				}
				m.stats.TotalElapsedNs = totalElapsed.Nanoseconds()
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
					m.pausedElapsed = m.baseElapsed + time.Since(m.startTime)
					m.timerPaused = true
				}
				if !m.loopTimerPaused {
					m.loopPausedElapsed = m.loopBaseElapsed + time.Since(m.loopStartTime)
					m.loopTimerPaused = true
				}
				m.loop.Pause()
			}
			return m, nil
		case "r", "s":
			// Resume the loop - resume elapsed time from where we paused (both total and per-loop)
			// Also handles resuming after completion when new loops were added via '+'
			// 's' key is the "start" shortcut shown when completed with pending loops
			if m.loop != nil {
				if m.timerPaused {
					m.baseElapsed = m.pausedElapsed
					m.startTime = time.Now()
					m.timerPaused = false
				}
				if m.loopTimerPaused {
					m.loopBaseElapsed = m.loopPausedElapsed
					m.loopStartTime = time.Now()
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
		// Update viewport content and schedule next tick
		// Note: we do NOT call GotoBottom() here — that would override the user's
		// scroll position every 250ms, making the viewport effectively unscrollable.
		// GotoBottom() is only called on viewport init and when new messages arrive.
		if m.viewportReady {
			m.viewport.SetContent(m.renderActivityContent())
		}
		m.updateTmuxStatusBar()
		return m, tickCmd()

	case newMessageMsg:
		m.AddMessage(Message(msg))
		if m.viewportReady {
			m.viewport.SetContent(m.renderActivityContent())
			m.viewport.GotoBottom()
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

	case agentUpdateMsg:
		m.activeAgents = msg.count
		return m, nil

	case taskUpdateMsg:
		m.currentTask = msg.task
		return m, nil

	case modeUpdateMsg:
		m.currentMode = msg.mode
		return m, nil

	case completedTasksUpdateMsg:
		m.completedTasks = msg.completed
		m.totalTasks = msg.total
		return m, nil

	case loopStartedMsg:
		// New loop iteration started — reset per-loop timer and tokens
		m.loopStartTime = time.Now()
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
			m.pausedElapsed = m.baseElapsed + time.Since(m.startTime)
			m.timerPaused = true
		}
		if !m.loopTimerPaused {
			m.loopPausedElapsed = m.loopBaseElapsed + time.Since(m.loopStartTime)
			m.loopTimerPaused = true
		}
		return m, nil
	}

	// Handle viewport scrolling
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// renderActivityContent renders the message content for the viewport
func (m Model) renderActivityContent() string {
	if len(m.messages) == 0 {
		waitStyle := lipgloss.NewStyle().Foreground(colorDimGray)
		return waitStyle.Render("Waiting for activity...")
	}

	var lines []string
	for _, msg := range m.messages {
		icon := msg.GetIcon()
		style := msg.GetStyle()

		// Format: icon + styled content
		line := fmt.Sprintf("%s %s", icon, style.Render(msg.Content))
		lines = append(lines, line)
		lines = append(lines, "") // Add empty line between messages
	}

	return strings.Join(lines, "\n")
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

	// Choose colors based on state
	borderColor := colorBlue
	statusText := "RUNNING"
	if m.completed {
		borderColor = colorGreen
		statusText = "COMPLETED"
	} else if isPaused {
		borderColor = colorRed
		statusText = "STOPPED"
	}

	// Activity panel style
	activityStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2).
		Width(m.width - 2).
		Height(m.activityHeight)

	// Centered status title at top
	statusTitle := lipgloss.NewStyle().
		Bold(true).
		Foreground(borderColor).
		Width(m.width - 2).
		Align(lipgloss.Center).
		Render(statusText)

	activityContent := activityStyle.Render(m.viewport.View())

	// Add centered status title above activity panel
	activityPanel := lipgloss.JoinVertical(
		lipgloss.Left,
		statusTitle,
		activityContent,
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

	// Usage & Cost panel
	usageCostContent := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render("Usage & Cost"),
		lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Total Tokens:"), valueStyle.Render(fmt.Sprintf(" %s", stats.FormatTokens(m.stats.TotalTokens())))),
		lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Input:"), valueStyle.Render(fmt.Sprintf(" %s", stats.FormatTokens(m.stats.InputTokens)))),
		lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Output:"), valueStyle.Render(fmt.Sprintf(" %s", stats.FormatTokens(m.stats.OutputTokens)))),
		lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Cache Write:"), valueStyle.Render(fmt.Sprintf(" %s", stats.FormatTokens(m.stats.CacheCreationTokens)))),
		lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Cache Read:"), valueStyle.Render(fmt.Sprintf(" %s", stats.FormatTokens(m.stats.CacheReadTokens)))),
		lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Total Cost:"), costStyle.Render(fmt.Sprintf(" $%.6f", m.stats.TotalCostUSD))),
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
	statusText := "Running"
	statusStyle := valueStyle.Foreground(colorGreen)
	if m.completed {
		statusText = "Completed"
		statusStyle = valueStyle.Foreground(colorGreen)
	} else if isPaused {
		statusText = "Stopped"
		statusStyle = valueStyle.Foreground(colorRed)
	}

	// Active Agents display
	agentDisplay := fmt.Sprintf(" %d", m.activeAgents)
	agentStyle := valueStyle
	if m.activeAgents > 0 {
		agentStyle = valueStyle.Foreground(colorGreen)
	}

	// Current Mode display
	modeDisplay := " -"
	if m.currentMode != "" {
		modeDisplay = fmt.Sprintf(" %s", m.currentMode)
	}

	// Completed Tasks display
	completedDisplay := fmt.Sprintf(" %d/%d", m.completedTasks, m.totalTasks)

	loopDetailsContent := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render("Ralph Loop Details"),
		lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Loop:"), valueStyle.Render(fmt.Sprintf(" %s", loopDisplay))),
		lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Total Time:"), valueStyle.Render(fmt.Sprintf(" %s", timeDisplay))),
		lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Status:"), statusStyle.Render(fmt.Sprintf(" %s", statusText))),
		lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Active Agents:"), agentStyle.Render(agentDisplay)),
		lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Completed Tasks:"), valueStyle.Render(completedDisplay)),
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
	if hasPendingLoops {
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

	loopDisplay := "#0/0"
	if m.totalLoops > 0 {
		loopDisplay = fmt.Sprintf("#%d/%d", m.currentLoop, m.totalLoops)
	}

	// Per-loop tokens and elapsed time (not cumulative)
	tokenDisplay := stats.FormatTokens(m.loopTotalTokens)

	loopElapsed := m.getLoopElapsed()
	hours := int(loopElapsed.Hours())
	minutes := int(loopElapsed.Minutes()) % 60
	seconds := int(loopElapsed.Seconds()) % 60
	timeDisplay := fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)

	m.tmuxBar.Update(tmux.FormatStatusRight(loopDisplay, tokenDisplay, timeDisplay))
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

// SendAgentUpdate is a helper command to update active agent count
func SendAgentUpdate(count int) tea.Cmd {
	return func() tea.Msg {
		return agentUpdateMsg{count: count}
	}
}

// SendTaskUpdate is a helper command to update the current task
func SendTaskUpdate(task string) tea.Cmd {
	return func() tea.Msg {
		return taskUpdateMsg{task: task}
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

// TickMsgForTest returns a tickMsg for use in tests
func TickMsgForTest() tea.Msg {
	return tickMsg(time.Now())
}

// Run starts the Bubble Tea program
func Run() error {
	p := tea.NewProgram(NewModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// RunWithChannels starts the TUI with external message and done channels
func RunWithChannels(msgChan <-chan Message, doneChan <-chan struct{}) error {
	model := NewModelWithChannels(msgChan, doneChan)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// CreateProgram creates a Bubble Tea program that can be controlled externally
func CreateProgram(msgChan <-chan Message, doneChan <-chan struct{}) *tea.Program {
	model := NewModelWithChannels(msgChan, doneChan)
	return tea.NewProgram(model, tea.WithAltScreen())
}
