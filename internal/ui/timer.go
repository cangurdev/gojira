package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	timerIssueStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	timerElapsedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2")).Width(10)
	timerLabelStyle   = lipgloss.NewStyle().Faint(true)
	timerHintStyle    = lipgloss.NewStyle().Faint(true)

	confirmYesStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("2")).Padding(0, 1)
	confirmNoStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("1")).Padding(0, 1)
	confirmIdleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Padding(0, 1)
)

// ── Live timer status ─────────────────────────────────────────────────────────

type tickMsg time.Time

type timerStatusModel struct {
	issueKey  string
	startedAt time.Time
	elapsed   time.Duration
	quit      bool
}

func (m timerStatusModel) Init() tea.Cmd {
	return tickEvery()
}

func tickEvery() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m timerStatusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		m.elapsed = time.Since(m.startedAt)
		return m, tickEvery()
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quit = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m timerStatusModel) View() string {
	elapsed := m.elapsed.Round(time.Second)
	h := int(elapsed.Hours())
	min := int(elapsed.Minutes()) % 60
	sec := int(elapsed.Seconds()) % 60

	elapsedStr := fmt.Sprintf("%02d:%02d:%02d", h, min, sec)

	return fmt.Sprintf(
		"\n  %s  %s\n  %s  %s\n  %s  %s\n\n  %s\n",
		timerLabelStyle.Render("Issue  :"),
		timerIssueStyle.Render(m.issueKey),
		timerLabelStyle.Render("Started:"),
		timerLabelStyle.Render(m.startedAt.Format("15:04")),
		timerLabelStyle.Render("Elapsed:"),
		timerElapsedStyle.Render(elapsedStr),
		timerHintStyle.Render("q quit"),
	)
}

// RunTimerStatus shows a live-updating timer display.
func RunTimerStatus(issueKey string, startedAt time.Time) error {
	m := timerStatusModel{
		issueKey:  issueKey,
		startedAt: startedAt,
		elapsed:   time.Since(startedAt),
	}
	_, err := tea.NewProgram(m).Run()
	return err
}

// ── Stop confirmation ─────────────────────────────────────────────────────────

// TimerStopChoice is the result of the stop confirmation prompt.
type TimerStopChoice int

const (
	TimerStopLog    TimerStopChoice = iota // log to Jira
	TimerStopDiscard                       // discard, don't log
	TimerStopCancel                        // cancelled (ctrl+c)
)

type timerConfirmModel struct {
	issueKey  string
	startedAt time.Time
	elapsed   string
	cursor    int // 0 = Log, 1 = Discard
	choice    TimerStopChoice
	done      bool
}

func (m timerConfirmModel) Init() tea.Cmd { return nil }

func (m timerConfirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			if m.cursor > 0 {
				m.cursor--
			}
		case "right", "l", "tab":
			if m.cursor < 1 {
				m.cursor++
			}
		case "enter", " ":
			if m.cursor == 0 {
				m.choice = TimerStopLog
			} else {
				m.choice = TimerStopDiscard
			}
			m.done = true
			return m, tea.Quit
		case "y", "Y":
			m.choice = TimerStopLog
			m.done = true
			return m, tea.Quit
		case "n", "N":
			m.choice = TimerStopDiscard
			m.done = true
			return m, tea.Quit
		case "ctrl+c", "esc", "q":
			m.choice = TimerStopCancel
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m timerConfirmModel) View() string {
	yesStyle := confirmIdleStyle
	noStyle := confirmIdleStyle
	if m.cursor == 0 {
		yesStyle = confirmYesStyle
	} else {
		noStyle = confirmNoStyle
	}

	return fmt.Sprintf(
		"\n  %s  %s\n  %s  %s\n  %s  %s\n\n  Log this time to Jira?\n  %s  %s\n\n  %s\n",
		timerLabelStyle.Render("Issue  :"),
		timerIssueStyle.Render(m.issueKey),
		timerLabelStyle.Render("Started:"),
		timerLabelStyle.Render(m.startedAt.Format("15:04")),
		timerLabelStyle.Render("Elapsed:"),
		timerElapsedStyle.Render(m.elapsed),
		yesStyle.Render("Log"),
		noStyle.Render("Discard"),
		timerHintStyle.Render("←/→ select • enter confirm • y/n • esc cancel"),
	)
}

// RunTimerConfirm shows an interactive stop confirmation and returns the user's choice.
func RunTimerConfirm(issueKey string, startedAt time.Time, elapsed string) (TimerStopChoice, error) {
	m := timerConfirmModel{
		issueKey:  issueKey,
		startedAt: startedAt,
		elapsed:   elapsed,
		cursor:    0,
		choice:    TimerStopCancel,
	}
	result, err := tea.NewProgram(m).Run()
	if err != nil {
		return TimerStopCancel, err
	}
	return result.(timerConfirmModel).choice, nil
}
