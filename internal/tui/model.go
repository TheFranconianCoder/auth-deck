package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/TheFranconianCoder/auth-deck/internal/core/entities"
	"github.com/TheFranconianCoder/auth-deck/internal/state"
)

type providerStatus struct {
	name      string
	token     *entities.Token
	lastErr   string
	hasToken  bool
	needsAuth bool
}

type Model struct {
	providers    []providerStatus
	pending      []*state.PendingRequest
	logs         []state.LogEntry
	selected     int
	width        int
	height       int
	ready        bool
	spinner      spinner.Model
	queue        *state.RequestQueue
	browserURL   string
	browserProto string
	notice       string
	fetchToken   func(provider string)
}

type tickMsg struct{}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func NewModel(catalog *state.ProviderCatalog, queue *state.RequestQueue, initial map[string]*entities.Token, fetchToken func(provider string)) Model {
	names := catalog.Names()
	statuses := make([]providerStatus, 0, len(names))
	for _, name := range names {
		status := providerStatus{name: name}
		if token, ok := initial[name]; ok {
			status.token = token
			status.hasToken = true
		}
		statuses = append(statuses, status)
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return Model{
		providers:  statuses,
		pending:    make([]*state.PendingRequest, 0),
		logs:       make([]state.LogEntry, 0),
		spinner:    s,
		queue:      queue,
		fetchToken: fetchToken,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, tickCmd())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case state.RequestAdded:
		m.pending = append(m.pending, msg.Request)
		m.selected = len(m.pending) - 1
		return m, nil

	case state.RequestDone:
		for i, p := range m.pending {
			if p.ID == msg.ID {
				m.pending = append(m.pending[:i], m.pending[i+1:]...)
				break
			}
		}
		if m.selected >= len(m.pending) && len(m.pending) > 0 {
			m.selected = len(m.pending) - 1
		}
		return m, nil

	case state.BrowserOpen:
		m.browserURL = msg.URL
		m.browserProto = msg.Provider
		m.notice = fmt.Sprintf("Browser opened for %s — waiting for callback...", msg.Provider)
		return m, nil

	case state.BrowserClosed:
		m.browserURL = ""
		m.browserProto = ""
		return m, nil

	case state.TokenUpdated:
		for i := range m.providers {
			if m.providers[i].name == msg.Provider {
				if msg.Token != nil {
					m.providers[i].token = msg.Token
					m.providers[i].hasToken = true
					m.providers[i].lastErr = ""
					m.providers[i].needsAuth = false
					m.notice = fmt.Sprintf("Token for %s acquired", msg.Provider)
				} else {
					m.providers[i].hasToken = false
					m.providers[i].lastErr = msg.Err
					m.notice = fmt.Sprintf("Token for %s failed: %s", msg.Provider, msg.Err)
				}
				break
			}
		}
		return m, nil

	case state.ReauthRequired:
		for i := range m.providers {
			if m.providers[i].name == msg.Provider {
				m.providers[i].needsAuth = true
				m.notice = fmt.Sprintf("Re-login required for %s — press [%d] to authenticate", msg.Provider, i+1)
				break
			}
		}
		return m, nil

	case tickMsg:
		m.logs = m.queue.Logs()
		m.pending = m.queue.Pending()
		if m.selected >= len(m.pending) {
			m.selected = len(m.pending) - 1
		}
		if m.selected < 0 {
			m.selected = 0
		}
		return m, tickCmd()

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}

		case "down", "j":
			if m.selected < len(m.pending)-1 {
				m.selected++
			}

		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			idx := int(msg.String()[0]-'0') - 1
			if idx < len(m.providers) {
				if len(m.pending) > 0 {
					if m.selected < 0 || m.selected >= len(m.pending) {
						m.selected = len(m.pending) - 1
					}
					req := m.pending[m.selected]
					req.ProviderCh <- m.providers[idx].name
					m.browserURL = ""
					return m, nil
				}
				if m.fetchToken != nil {
					m.notice = fmt.Sprintf("Fetching token for %s ...", m.providers[idx].name)
					m.fetchToken(m.providers[idx].name)
				}
			}

		case "esc":
			m.notice = ""
			return m, nil
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)

	total := m.width
	if total < 72 {
		total = 72
	}

	leftLines := m.providerLines()
	leftWidth := 0
	for _, line := range leftLines {
		if w := lipgloss.Width(line); w > leftWidth {
			leftWidth = w
		}
	}
	if leftWidth < 24 {
		leftWidth = 24
	}

	rightWidth := total - leftWidth - 9
	if rightWidth < 24 {
		rightWidth = 24
	}

	rightLines := m.logLines(rightWidth)
	rows := len(leftLines)
	if len(rightLines) > rows {
		rows = len(rightLines)
	}
	leftLines = padLines(padRows(leftLines, rows), leftWidth)
	rightLines = padLines(padRows(rightLines, rows), rightWidth)

	header := lipgloss.JoinHorizontal(
		lipgloss.Top,
		box.BorderForeground(lipgloss.Color("62")).Render(join(leftLines)),
		box.BorderForeground(lipgloss.Color("240")).Render(join(rightLines)),
	)

	var b []string
	b = append(b, titleStyle.Render("AuthDeck — OAuth 2.0 Local Token Proxy"), "")
	b = append(b, header, "")

	if m.browserURL != "" {
		b = append(b, box.BorderForeground(lipgloss.Color("33")).Render(join([]string{
			lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("33")).Render("Opening browser..."),
			"",
			fmt.Sprintf("  Provider: %s", m.browserProto),
			lipgloss.NewStyle().Faint(true).Render("  Waiting for callback..."),
		})), "")
	} else if len(m.pending) > 0 {
		lines := []string{lipgloss.NewStyle().Bold(true).Render("New request — select provider:")}
		for i, req := range m.pending {
			cursor := "  "
			if i == m.selected {
				cursor = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("▸ ")
			}
			method := lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Render(req.Method)
			path := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(truncate(req.Path, total-24))
			lines = append(lines, fmt.Sprintf("%s%s %s  %s", cursor, method, path, req.CreatedAt.Format("15:04:05")))
		}
		lines = append(lines, "", lipgloss.NewStyle().Faint(true).Render("[1-9] select provider  [↑↓] navigate"))
		b = append(b, box.BorderForeground(lipgloss.Color("228")).Render(join(lines)), "")
	} else {
		b = append(b, lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("  Waiting for requests..."), "")
	}

	if m.notice != "" {
		b = append(b, lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("  "+truncate(m.notice, total-4)), "")
	}

	b = append(b, lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginTop(1).Render("[q] Quit"))

	out := ""
	for _, line := range b {
		out += line + "\n"
	}
	return out
}

func (m Model) providerLines() []string {
	lines := []string{lipgloss.NewStyle().Bold(true).Render("Providers:")}
	for i, p := range m.providers {
		marker := lipgloss.NewStyle().Foreground(lipgloss.Color("31")).Render("○ no token")
		if p.token != nil && p.token.IsValid() {
			remaining := time.Until(p.token.ExpiresAt)
			marker = lipgloss.NewStyle().Foreground(lipgloss.Color("32")).Render("● " + formatRemaining(remaining))
		} else if p.needsAuth {
			marker = lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Render("↻ re-login")
		} else if p.hasToken {
			marker = lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Render("◐ expired")
		} else if p.lastErr != "" {
			marker = lipgloss.NewStyle().Foreground(lipgloss.Color("31")).Render("✗ error")
		}
		lines = append(lines, fmt.Sprintf(" [%d] %-11.11s %s", i+1, p.name, marker))
	}
	return lines
}

func (m Model) logLines(width int) []string {
	lines := []string{lipgloss.NewStyle().Bold(true).Render("Log:")}

	const rows = 12
	start := len(m.logs) - rows
	if start < 0 {
		start = 0
	}

	// "15:04:05 " + method(6) + " " + provider(10) + " " + status(3) + " "
	const prefix = 31

	for _, e := range m.logs[start:] {
		statusColor := "32"
		if e.Status >= 400 {
			statusColor = "31"
		} else if e.Status >= 300 {
			statusColor = "33"
		}
		statusPlain := "   "
		if e.Status > 0 {
			statusPlain = fmt.Sprintf("%3d", e.Status)
		}
		status := lipgloss.NewStyle().Foreground(lipgloss.Color(statusColor)).Render(statusPlain)

		provider := e.Provider
		if provider == "" {
			provider = "—"
		}

		suffix := ""
		if e.Err != "" {
			suffix = "  " + e.Err
		}
		maxSuffix := width - prefix - 6
		if maxSuffix < 0 {
			maxSuffix = 0
		}
		suffix = truncate(suffix, maxSuffix)

		// Reserve room for the prefix and the trailing duration/error.
		pathWidth := width - prefix - lipgloss.Width(suffix)
		if pathWidth < 0 {
			pathWidth = 0
		}

		line := fmt.Sprintf("%s %-6.6s %-10.10s %s %s",
			e.Time.Format("15:04:05"),
			e.Method,
			provider,
			status,
			truncate(e.Path, pathWidth),
		)
		if suffix != "" {
			line += lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(suffix)
		}
		lines = append(lines, line)
	}
	return lines
}

func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	return s[:n-1] + "…"
}

func padRows(lines []string, rows int) []string {
	for len(lines) < rows {
		lines = append(lines, "")
	}
	return lines
}

// padLines right-pads every line to the given display width so boxes align and
// content does not wrap.
func padLines(lines []string, width int) []string {
	for i, line := range lines {
		if w := lipgloss.Width(line); w < width {
			lines[i] = line + strings.Repeat(" ", width-w)
		}
	}
	return lines
}

func join(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}
