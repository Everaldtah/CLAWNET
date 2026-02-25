package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/Everaldtah/CLAWNET/internal/market"
	"github.com/Everaldtah/CLAWNET/internal/network"
	"github.com/Everaldtah/CLAWNET/internal/protocol"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7D56F4")).
		MarginLeft(2)

	infoStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A0A0A0"))

	successStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00FF00"))

	errorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF0000"))

	boxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(1).
		Width(40)

	activeBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#00FF00")).
		Padding(1).
		Width(40)
)

// KeyMap defines key bindings
type KeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	Enter    key.Binding
	Quit     key.Binding
	Help     key.Binding
	Tab      key.Binding
	Market   key.Binding
	Peers    key.Binding
	Memory   key.Binding
	Logs     key.Binding
}

// DefaultKeyMap returns default key bindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "left"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "right"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "execute"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next panel"),
		),
		Market: key.NewBinding(
			key.WithKeys("m"),
			key.WithHelp("m", "market"),
		),
		Peers: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "peers"),
		),
		Memory: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "memory"),
		),
		Logs: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "logs"),
		),
	}
}

// Model represents the TUI model
type Model struct {
	// Dependencies
	host       *network.Host
	market     *market.MarketManager
	logger     *logrus.Logger

	// UI state
	keys       KeyMap
	help       help.Model
	spinner    spinner.Model
	input      textinput.Model
	viewport   viewport.Model

	// Panel state
	activePanel int
	panels      []string

	// Data
	peers       []*network.PeerInfo
	auctions    []*market.Auction
	tasks       []*market.ScheduledTask
	wallet      *market.Wallet
	reputation  *market.Reputation
	logs        []string

	// Command mode
	commandMode bool
	command     string

	// Dimensions
	width       int
	height      int

	// Context
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewModel creates a new TUI model
func NewModel(host *network.Host, marketManager *market.MarketManager, logger *logrus.Logger) (*Model, error) {
	ctx, cancel := context.WithCancel(context.Background())

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))

	input := textinput.New()
	input.Placeholder = "Enter command..."
	input.CharLimit = 256
	input.Width = 50

	m := &Model{
		host:       host,
		market:     marketManager,
		logger:     logger,
		keys:       DefaultKeyMap(),
		help:       help.New(),
		spinner:    s,
		input:      input,
		panels:     []string{"peers", "market", "memory", "logs"},
		logs:       make([]string, 0, 100),
		ctx:        ctx,
		cancel:     cancel,
	}

	return m, nil
}

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.tick(),
	)
}

// Update updates the model
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.cancel()
			return m, tea.Quit

		case key.Matches(msg, m.keys.Tab):
			m.activePanel = (m.activePanel + 1) % len(m.panels)

		case key.Matches(msg, m.keys.Market):
			m.activePanel = 1

		case key.Matches(msg, m.keys.Peers):
			m.activePanel = 0

		case key.Matches(msg, m.keys.Memory):
			m.activePanel = 2

		case key.Matches(msg, m.keys.Logs):
			m.activePanel = 3

		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll

		case key.Matches(msg, m.keys.Enter):
			if m.commandMode {
				m.executeCommand(m.input.Value())
				m.input.SetValue("")
				m.commandMode = false
			}

		case msg.String() == "/":
			m.commandMode = true
			m.input.Focus()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 10

	case tickMsg:
		m.refreshData()
		return m, m.tick()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// View renders the UI
func (m *Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var sections []string

	// Header
	sections = append(sections, m.renderHeader())

	// Main content
	content := m.renderMainContent()
	sections = append(sections, content)

	// Command input
	if m.commandMode {
		sections = append(sections, m.renderCommandInput())
	}

	// Help
	sections = append(sections, m.help.View(m.keys))

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderHeader renders the header
func (m *Model) renderHeader() string {
	peerCount := 0
	if m.host != nil {
		peerCount = m.host.GetPeerCount()
	}

	info := fmt.Sprintf("CLAWNET | Peers: %d | Panel: %s",
		peerCount,
		m.panels[m.activePanel],
	)

	return titleStyle.Render(info)
}

// renderMainContent renders the main content area
func (m *Model) renderMainContent() string {
	switch m.panels[m.activePanel] {
	case "peers":
		return m.renderPeersPanel()
	case "market":
		return m.renderMarketPanel()
	case "memory":
		return m.renderMemoryPanel()
	case "logs":
		return m.renderLogsPanel()
	default:
		return "Unknown panel"
	}
}

// renderPeersPanel renders the peers panel
func (m *Model) renderPeersPanel() string {
	var content strings.Builder

	content.WriteString("Connected Peers:\n\n")

	if m.host != nil {
		peers := m.host.GetPeers()
		for _, peer := range peers {
			info := fmt.Sprintf("  %s | Load: %.2f | Reputation: %.2f\n",
				peer.ID.String()[:16],
				peer.Load,
				peer.Reputation,
			)
			content.WriteString(info)
		}
	}

	style := boxStyle
	if m.activePanel == 0 {
		style = activeBoxStyle
	}

	return style.Render(content.String())
}

// renderMarketPanel renders the market panel
func (m *Model) renderMarketPanel() string {
	var content strings.Builder

	content.WriteString("Market Status:\n\n")

	// Wallet info
	if m.wallet != nil {
		content.WriteString(fmt.Sprintf("  Wallet: %.2f credits\n", m.wallet.GetTotalBalance()))
	}

	// Reputation
	if m.reputation != nil {
		content.WriteString(fmt.Sprintf("  Reputation: %.2f\n", m.reputation.Score))
	}

	// Active auctions
	content.WriteString("\nActive Auctions:\n")
	if m.market != nil {
		auctions := m.market.GetActiveAuctions()
		for _, auction := range auctions {
			auction.mu.RLock()
			info := fmt.Sprintf("  %s | Budget: %.2f | Bids: %d\n",
				auction.ID[:8],
				auction.MaxBudget,
				len(auction.Bids),
			)
			auction.mu.RUnlock()
			content.WriteString(info)
		}
	}

	// Active tasks
	content.WriteString("\nActive Tasks:\n")
	if m.market != nil {
		tasks := m.market.GetActiveTasks()
		for _, task := range tasks {
			task.mu.RLock()
			info := fmt.Sprintf("  %s | Type: %s | Status: %s\n",
				task.ID[:8],
				task.OriginalTask.Type,
				task.Status,
			)
			task.mu.RUnlock()
			content.WriteString(info)
		}
	}

	style := boxStyle
	if m.activePanel == 1 {
		style = activeBoxStyle
	}

	return style.Render(content.String())
}

// renderMemoryPanel renders the memory panel
func (m *Model) renderMemoryPanel() string {
	var content strings.Builder

	content.WriteString("Memory Status:\n\n")
	content.WriteString("  Sync Status: Active\n")
	content.WriteString("  Entries: 0\n")
	content.WriteString("  Last Sync: Just now\n")

	style := boxStyle
	if m.activePanel == 2 {
		style = activeBoxStyle
	}

	return style.Render(content.String())
}

// renderLogsPanel renders the logs panel
func (m *Model) renderLogsPanel() string {
	var content strings.Builder

	content.WriteString("Recent Logs:\n\n")

	// Show last 20 logs
	start := len(m.logs) - 20
	if start < 0 {
		start = 0
	}

	for _, log := range m.logs[start:] {
		content.WriteString(log + "\n")
	}

	style := boxStyle
	if m.activePanel == 3 {
		style = activeBoxStyle
	}

	return style.Render(content.String())
}

// renderCommandInput renders the command input
func (m *Model) renderCommandInput() string {
	return m.input.View()
}

// executeCommand executes a command
func (m *Model) executeCommand(cmd string) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "/market":
		if len(parts) >= 2 {
			switch parts[1] {
			case "submit":
				if len(parts) >= 3 {
					description := strings.Join(parts[2:], " ")
					m.submitMarketTask(description)
				}
			case "status":
				m.addLog("Market status: Active")
			case "history":
				m.addLog("Showing task history...")
			}
		}

	case "/peer":
		if len(parts) >= 2 {
			switch parts[1] {
			case "list":
				m.addLog(fmt.Sprintf("Connected peers: %d", m.host.GetPeerCount()))
			case "connect":
				if len(parts) >= 3 {
					m.addLog(fmt.Sprintf("Connecting to %s...", parts[2]))
				}
			}
		}

	case "/memory":
		if len(parts) >= 2 {
			switch parts[1] {
			case "sync":
				m.addLog("Syncing memory...")
			case "list":
				m.addLog("Listing memory entries...")
			}
		}

	case "/quit", "/exit":
		m.cancel()

	default:
		m.addLog(fmt.Sprintf("Unknown command: %s", parts[0]))
	}
}

// submitMarketTask submits a task to the market
func (m *Model) submitMarketTask(description string) {
	if m.market == nil {
		m.addLog("Market not available")
		return
	}

	// Create task announcement
	task := &protocol.MarketTaskAnnouncePayload{
		TaskID:               generateID(),
		Description:          description,
		Type:                 protocol.TaskTypeOpenClawPrompt,
		MaxBudget:            10.0,
		Deadline:             time.Now().Add(time.Minute).UnixNano(),
		RequiredCapabilities: []string{"ai-inference"},
		MinimumReputation:    0.3,
		RequesterID:          m.host.ID().String(),
		BidTimeout:           int64(5 * time.Second),
		EscrowRequired:       true,
		ConsensusMode:        false,
	}

	_, err := m.market.SubmitTask(task)
	if err != nil {
		m.addLog(fmt.Sprintf("Failed to submit task: %v", err))
		return
	}

	m.addLog(fmt.Sprintf("Task submitted: %s", task.TaskID[:8]))
}

// addLog adds a log entry
func (m *Model) addLog(msg string) {
	timestamp := time.Now().Format("15:04:05")
	m.logs = append(m.logs, fmt.Sprintf("[%s] %s", timestamp, msg))

	// Keep only last 100 logs
	if len(m.logs) > 100 {
		m.logs = m.logs[len(m.logs)-100:]
	}
}

// refreshData refreshes data from dependencies
func (m *Model) refreshData() {
	if m.market != nil {
		m.wallet = m.market.GetWallet()
		m.reputation = m.market.GetReputation()
	}
}

// tickMsg is a tick message
type tickMsg struct{}

// tick returns a tick command
func (m *Model) tick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

// generateID generates a short ID
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// Run runs the TUI
func Run(host *network.Host, market *market.MarketManager, logger *logrus.Logger) error {
	model, err := NewModel(host, market, logger)
	if err != nil {
		return err
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}
