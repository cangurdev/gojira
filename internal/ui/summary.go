package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"gojira/internal/jira"
)

var (
	headerStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	subtotalStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Italic(true)
	totalStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
	baseStyle     = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("8"))

	formLabelStyle   = lipgloss.NewStyle().Faint(true).Width(14)
	formActiveStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	formBorderStyle  = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("6")).Padding(1, 2)
	formSuccessStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
	formErrorStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1"))
)

type summaryState int

const (
	stateTable summaryState = iota
	stateForm
)

// LogFunc is called when the user submits a log entry from the summary screen.
type LogFunc func(issueKey, timeSpent, startTime string) (*jira.WorklogResponse, error)

type summaryModel struct {
	viewport  viewport.Model
	title     string
	subtotals []string
	totalLine string

	state    summaryState
	inputs   [3]textinput.Model // 0: issue, 1: time, 2: start
	focused  int
	logFunc  LogFunc
	feedback string
	isError  bool
}

func (m summaryModel) Init() tea.Cmd {
	return nil
}

func (m summaryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if res, ok := msg.(logResultMsg); ok {
		if res.err != nil {
			m.feedback = res.err.Error()
			m.isError = true
		} else {
			m.feedback = fmt.Sprintf("✓ Logged %s to %s", res.response.TimeSpent, m.inputs[0].Value())
			m.isError = false
			m.state = stateTable
		}
		return m, nil
	}

	switch m.state {
	case stateTable:
		return m.updateTable(msg)
	case stateForm:
		return m.updateForm(msg)
	}
	return m, nil
}

func (m summaryModel) updateTable(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "n":
			m.state = stateForm
			m.focused = 0
			m.feedback = ""
			m.isError = false
			for i := range m.inputs {
				m.inputs[i].SetValue("")
			}
			return m, m.inputs[0].Focus()
		}
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m summaryModel) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.state = stateTable
			return m, nil
		case "tab", "down":
			m.inputs[m.focused].Blur()
			m.focused = (m.focused + 1) % len(m.inputs)
			return m, m.inputs[m.focused].Focus()
		case "shift+tab", "up":
			m.inputs[m.focused].Blur()
			m.focused = (m.focused + len(m.inputs) - 1) % len(m.inputs)
			return m, m.inputs[m.focused].Focus()
		case "enter":
			if m.focused < len(m.inputs)-1 {
				m.inputs[m.focused].Blur()
				m.focused++
				return m, m.inputs[m.focused].Focus()
			}
			// Submit
			return m.submitLog()
		}
	}

	var cmd tea.Cmd
	m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
	return m, cmd
}

type logResultMsg struct {
	response *jira.WorklogResponse
	err      error
}

func (m summaryModel) submitLog() (tea.Model, tea.Cmd) {
	issueKey := strings.TrimSpace(m.inputs[0].Value())
	timeSpent := strings.TrimSpace(m.inputs[1].Value())
	startRaw := strings.TrimSpace(m.inputs[2].Value())

	if issueKey == "" || timeSpent == "" {
		m.feedback = "Issue key and time are required"
		m.isError = true
		return m, nil
	}

	// Resolve start time
	var startTime string
	if startRaw != "" {
		if t, err := time.ParseInLocation("15:04", startRaw, time.Local); err == nil {
			now := time.Now()
			full := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, time.Local)
			startTime = full.Format("2006-01-02T15:04:05.000-0700")
		} else if t, err := time.ParseInLocation("2006-01-02 15:04", startRaw, time.Local); err == nil {
			startTime = t.Format("2006-01-02T15:04:05.000-0700")
		} else {
			m.feedback = `Invalid start time — use HH:MM or "YYYY-MM-DD HH:MM"`
			m.isError = true
			return m, nil
		}
	} else {
		startTime = time.Now().Format("2006-01-02T15:04:05.000-0700")
	}

	logFunc := m.logFunc
	return m, func() tea.Msg {
		resp, err := logFunc(issueKey, timeSpent, startTime)
		return logResultMsg{response: resp, err: err}
	}
}


func (m summaryModel) View() string {
	var sb strings.Builder
	sb.WriteString(m.viewTable())
	if m.state == stateForm {
		sb.WriteString("\n")
		sb.WriteString(m.viewForm())
	} else {
		sb.WriteString(lipgloss.NewStyle().Faint(true).Render("↑/↓/←/→ scroll • n new log • q quit"))
		sb.WriteString("\n")
	}
	return sb.String()
}

func (m summaryModel) viewTable() string {
	var sb strings.Builder
	sb.WriteString(headerStyle.Render(m.title))
	sb.WriteString("\n\n")
	sb.WriteString(baseStyle.Render(m.viewport.View()))
	sb.WriteString("\n")
	for _, s := range m.subtotals {
		sb.WriteString(subtotalStyle.Render(s))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	sb.WriteString(totalStyle.Render(m.totalLine))
	sb.WriteString("\n\n")
	if m.feedback != "" && m.state != stateForm {
		if m.isError {
			sb.WriteString(formErrorStyle.Render(m.feedback))
		} else {
			sb.WriteString(formSuccessStyle.Render(m.feedback))
		}
		sb.WriteString("\n\n")
	}
	return sb.String()
}

func (m summaryModel) viewForm() string {
	labels := []string{"Issue Key", "Time Spent", "Start Time"}
	hints := []string{"e.g. PROJ-123", "e.g. 1h, 30m, 1h30m", "e.g. 09:30 (optional)"}

	var sb strings.Builder
	sb.WriteString(formActiveStyle.Render("New Log Entry"))
	sb.WriteString("\n\n")

	for i, input := range m.inputs {
		label := formLabelStyle.Render(labels[i] + ":")
		field := input.View()
		hint := lipgloss.NewStyle().Faint(true).Render("  " + hints[i])
		if i == m.focused {
			sb.WriteString(formActiveStyle.Render("▶ ") + label + field + "\n" + hint + "\n\n")
		} else {
			sb.WriteString("  " + label + field + "\n\n")
		}
	}

	if m.feedback != "" {
		if m.isError {
			sb.WriteString(formErrorStyle.Render("  " + m.feedback))
		} else {
			sb.WriteString(formSuccessStyle.Render("  " + m.feedback))
		}
		sb.WriteString("\n\n")
	}

	sb.WriteString(lipgloss.NewStyle().Faint(true).Render("  tab/↑↓ navigate • enter next/submit • esc back"))
	sb.WriteString("\n")

	return formBorderStyle.Render(sb.String())
}

// RunSummaryTable renders an interactive Bubble Tea table for the given worklogs.
func RunSummaryTable(worklogs []jira.WorklogWithIssue, title string, logFn LogFunc) error {
	if len(worklogs) == 0 {
		fmt.Println("No worklogs found.")
		return nil
	}

	sort.Slice(worklogs, func(i, j int) bool {
		return worklogs[i].Worklog.Started.Before(worklogs[j].Worklog.Started.Time)
	})

	type dayGroup struct {
		label   string
		entries []jira.WorklogWithIssue
		total   int
	}
	var days []dayGroup
	dayIndex := map[string]int{}

	for _, wl := range worklogs {
		key := wl.Worklog.Started.Format("2006-01-02")
		if _, ok := dayIndex[key]; !ok {
			dayIndex[key] = len(days)
			days = append(days, dayGroup{label: wl.Worklog.Started.Format("Mon, Jan 2")})
		}
		idx := dayIndex[key]
		days[idx].entries = append(days[idx].entries, wl)
		days[idx].total += wl.Worklog.TimeSpentSeconds
	}

	dayHeaderStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6")).Underline(true)
	dayTotalStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3"))
	timeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	spentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))

	const colWidth = 44
	const summaryWidth = 22
	colStyle := lipgloss.NewStyle().Width(colWidth).PaddingRight(2)

	var subtotals []string
	grandTotal := 0

	dayColumns := make([]string, 0, len(days))
	for _, day := range days {
		h := day.total / 3600
		mn := (day.total % 3600) / 60

		var col strings.Builder
		col.WriteString(dayHeaderStyle.Render(day.label))
		col.WriteString("  ")
		col.WriteString(dayTotalStyle.Render(fmt.Sprintf("(%dh %dm)", h, mn)))
		col.WriteString("\n\n")
		for _, wl := range day.entries {
			col.WriteString(fmt.Sprintf("%s  %s  %s  %s\n",
				timeStyle.Render(wl.Worklog.Started.Format("15:04")),
				keyStyle.Render(fmt.Sprintf("%-10s", wl.IssueKey)),
				fmt.Sprintf("%-*s", summaryWidth, truncate(wl.Summary, summaryWidth)),
				spentStyle.Render(wl.Worklog.TimeSpent),
			))
		}
		dayColumns = append(dayColumns, colStyle.Render(col.String()))
		subtotals = append(subtotals, fmt.Sprintf("  %-14s total: %dh %dm", day.label, h, mn))
		grandTotal += day.total
	}

	gh := grandTotal / 3600
	gm := (grandTotal % 3600) / 60
	totalLine := fmt.Sprintf("Grand total: %dh %dm (%d worklogs)", gh, gm, len(worklogs))

	content := lipgloss.JoinHorizontal(lipgloss.Top, dayColumns...)
	vp := viewport.New(100, 15)
	vp.SetHorizontalStep(colWidth)
	vp.SetContent(content)

	// Text inputs
	var inputs [3]textinput.Model
	for i := range inputs {
		inputs[i] = textinput.New()
		inputs[i].CharLimit = 64
	}
	inputs[0].Placeholder = "PROJ-123"
	inputs[1].Placeholder = "1h, 30m, 1h30m"
	inputs[2].Placeholder = "09:30  (leave empty for now)"

	m := summaryModel{
		viewport:  vp,
		title:     title,
		subtotals: subtotals,
		totalLine: totalLine,
		inputs:    inputs,
		logFunc:   logFn,
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	// Print any pending feedback after program exits (e.g. success after log)
	if sm, ok := finalModel.(summaryModel); ok && sm.feedback != "" && !sm.isError {
		fmt.Println(sm.feedback)
	}

	return nil
}
