package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cloudosai/ralph-go/internal/stats"
)

// Color palette matching Python visualizer (Tokyo Night theme)
var (
	colorBlue     = lipgloss.Color("#7AA2F7")
	colorPurple   = lipgloss.Color("#BB9AF7")
	colorGreen    = lipgloss.Color("#9ECE6A")
	colorDimGray  = lipgloss.Color("#565F89")
	colorLightGray = lipgloss.Color("#C0CAF5")
	colorBg       = lipgloss.Color("#1A1B26")
)

// MessageRole represents the type of message sender
type MessageRole string

const (
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"
	RoleUser      MessageRole = "user"
	RoleSystem    MessageRole = "system"
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
		return "🤖"
	case RoleTool:
		return "🔧"
	case RoleUser:
		return "📝"
	case RoleSystem:
		return "💰"
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
	default:
		return lipgloss.NewStyle().Foreground(colorDimGray)
	}
}

// Model represents the TUI application state
type Model struct {
	ready          bool
	width          int
	height         int
	quitting       bool
	messages       []Message
	maxMessages    int
	stats          *stats.TokenStats
	currentLoop    int
	totalLoops     int
	startTime      time.Time
	viewport       viewport.Model
	activityHeight int
	footerHeight   int
	msgChan        <-chan Message
	doneChan       <-chan struct{}
}

// NewModel creates and returns a new initialized Model
func NewModel() Model {
	return Model{
		ready:          false,
		width:          0,
		height:         0,
		quitting:       false,
		messages:       []Message{},
		maxMessages:    20,
		stats:          stats.NewTokenStats(),
		currentLoop:    0,
		totalLoops:     0,
		startTime:      time.Now(),
		activityHeight: 0,
		footerHeight:   9,
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

// doneMsg is sent when processing is complete
type doneMsg struct{}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tickCmd()}

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

		// Initialize or update viewport
		if !m.ready {
			m.viewport = viewport.New(m.width-4, m.activityHeight-2)
			m.viewport.SetContent(m.renderActivityContent())
			m.ready = true
		} else {
			m.viewport.Width = m.width - 4
			m.viewport.Height = m.activityHeight - 2
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}

	case tickMsg:
		// Update viewport content and schedule next tick
		if m.ready {
			m.viewport.SetContent(m.renderActivityContent())
			m.viewport.GotoBottom()
		}
		return m, tickCmd()

	case newMessageMsg:
		m.AddMessage(Message(msg))
		if m.ready {
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

	case doneMsg:
		// Processing is done, but keep TUI running until user quits
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
		return "Initializing..."
	}

	// Render the main layout
	return m.renderLayout()
}

// renderLayout creates the full layout with activity panel and footer
func (m Model) renderLayout() string {
	// Activity panel style
	activityStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBlue).
		Padding(1, 2).
		Width(m.width - 2).
		Height(m.activityHeight).
		Background(colorBg)

	// Render activity panel with title
	activityTitle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorBlue).
		Render("Activity")

	activityContent := activityStyle.Render(m.viewport.View())

	// Add title to activity panel
	activityPanel := lipgloss.JoinVertical(
		lipgloss.Left,
		activityTitle,
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

// renderFooter renders the three-panel footer
func (m Model) renderFooter() string {
	// Calculate panel width (divide by 3, accounting for spacing)
	panelWidth := (m.width - 8) / 3

	// Panel styles
	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPurple).
		Padding(0, 1).
		Width(panelWidth).
		Height(m.footerHeight - 2).
		Background(colorBg)

	labelStyle := lipgloss.NewStyle().
		Foreground(colorBlue).
		Align(lipgloss.Right).
		Width(12)

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
		lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Total Tokens:"), valueStyle.Render(fmt.Sprintf(" %d", m.stats.TotalTokens()))),
		lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Input:"), valueStyle.Render(fmt.Sprintf(" %d", m.stats.InputTokens))),
		lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Output:"), valueStyle.Render(fmt.Sprintf(" %d", m.stats.OutputTokens))),
		lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Total Cost:"), costStyle.Render(fmt.Sprintf(" $%.6f", m.stats.TotalCostUSD))),
	)
	usageCostPanel := panelStyle.Render(usageCostContent)

	// Loop Details panel
	loopDisplay := "0/0"
	if m.totalLoops > 0 {
		loopDisplay = fmt.Sprintf("%d/%d", m.currentLoop, m.totalLoops)
	}

	elapsed := time.Since(m.startTime)
	hours := int(elapsed.Hours())
	minutes := int(elapsed.Minutes()) % 60
	seconds := int(elapsed.Seconds()) % 60
	timeDisplay := fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)

	loopDetailsContent := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render("Loop Details"),
		lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Loop:"), valueStyle.Render(fmt.Sprintf(" %s", loopDisplay))),
		lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Elapsed:"), valueStyle.Render(fmt.Sprintf(" %s", timeDisplay))),
	)
	loopDetailsPanel := panelStyle.Render(loopDetailsContent)

	// Placeholder panel
	placeholderContent := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render("Placeholder 2"),
		lipgloss.NewStyle().Foreground(colorDimGray).Render("Coming soon..."),
	)
	placeholderPanel := panelStyle.Render(placeholderContent)

	// Join all panels horizontally
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		usageCostPanel,
		loopDetailsPanel,
		placeholderPanel,
	)
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
