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

	summaryCursorStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))

	summaryDangerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("1")).Padding(0, 1)
	summaryIdleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Padding(0, 1)
	summaryConfirmStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("6")).Padding(0, 1)
)

type summaryState int

const (
	stateTable summaryState = iota
	stateForm
	stateEdit
	stateConfirmDelete
)

// LogFunc is called when the user submits a new-log entry from the summary screen.
type LogFunc func(issueKey, timeSpent, startTime string) (*jira.WorklogResponse, error)

// SummaryCallbacks wires the summary TUI actions to Jira API calls.
type SummaryCallbacks struct {
	Log     LogFunc
	Update  func(issueKey, worklogID, timeSpent, started, description string) (*jira.WorklogResponse, error)
	Delete  func(issueKey, worklogID string) error
	Refresh func() ([]jira.WorklogWithIssue, error)
}

type summaryEntry struct {
	idx int // flat index into summaryModel.worklogs
	wl  jira.WorklogWithIssue
}

type summaryDay struct {
	label   string
	total   int // seconds
	entries []summaryEntry
}

type summaryModel struct {
	viewport viewport.Model
	title    string

	worklogs  []jira.WorklogWithIssue
	days      []summaryDay
	cursor    int
	subtotals []string
	totalLine string

	state summaryState

	// new-log form
	inputs  [3]textinput.Model
	focused int

	// edit form
	editInputs  [3]textinput.Model
	editFocused int

	// delete confirm
	deleteCursor int // 0 = Cancel, 1 = Delete

	cbs SummaryCallbacks

	feedback string
	isError  bool
}

func (m summaryModel) Init() tea.Cmd { return nil }

type logResultMsg struct {
	response *jira.WorklogResponse
	err      error
}

type editResultMsg struct {
	response *jira.WorklogResponse
	err      error
}

type deleteResultMsg struct {
	err error
}

type refreshResultMsg struct {
	worklogs []jira.WorklogWithIssue
	err      error
}

func (m summaryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case logResultMsg:
		if msg.err != nil {
			m.feedback = msg.err.Error()
			m.isError = true
			return m, nil
		}
		m.feedback = fmt.Sprintf("✓ Logged %s to %s", msg.response.TimeSpent, m.inputs[0].Value())
		m.isError = false
		m.state = stateTable
		return m, m.refreshCmd()

	case editResultMsg:
		if msg.err != nil {
			m.feedback = msg.err.Error()
			m.isError = true
			return m, nil
		}
		m.feedback = fmt.Sprintf("✓ Updated worklog (%s)", msg.response.TimeSpent)
		m.isError = false
		m.state = stateTable
		return m, m.refreshCmd()

	case deleteResultMsg:
		if msg.err != nil {
			m.feedback = msg.err.Error()
			m.isError = true
			return m, nil
		}
		m.feedback = "✓ Deleted worklog"
		m.isError = false
		m.state = stateTable
		return m, m.refreshCmd()

	case refreshResultMsg:
		if msg.err != nil {
			m.feedback = msg.err.Error()
			m.isError = true
			return m, nil
		}
		prevID := ""
		if w := m.currentWorklog(); w != nil {
			prevID = w.Worklog.ID
		}
		m.setWorklogs(msg.worklogs)
		if prevID != "" {
			for i, w := range m.worklogs {
				if w.Worklog.ID == prevID {
					m.cursor = i
					break
				}
			}
		}
		m.viewport.SetContent(m.renderContent())
		return m, nil
	}

	switch m.state {
	case stateTable:
		return m.updateTable(msg)
	case stateForm:
		return m.updateForm(msg)
	case stateEdit:
		return m.updateEdit(msg)
	case stateConfirmDelete:
		return m.updateConfirmDelete(msg)
	}
	return m, nil
}

func (m summaryModel) updateTable(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "down", "j":
			if m.cursor < len(m.worklogs)-1 {
				m.cursor++
				m.viewport.SetContent(m.renderContent())
			}
			return m, nil
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.viewport.SetContent(m.renderContent())
			}
			return m, nil
		case "n":
			m.state = stateForm
			m.focused = 0
			m.feedback = ""
			m.isError = false
			for i := range m.inputs {
				m.inputs[i].SetValue("")
			}
			return m, m.inputs[0].Focus()
		case "e":
			if wl := m.currentWorklog(); wl != nil {
				m = m.openEditForm(wl)
				return m, m.editInputs[0].Focus()
			}
			return m, nil
		case "d":
			if m.currentWorklog() != nil {
				m.state = stateConfirmDelete
				m.deleteCursor = 0
				m.feedback = ""
				m.isError = false
			}
			return m, nil
		case "r":
			m.feedback = "Refreshing..."
			m.isError = false
			return m, m.refreshCmd()
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
			return m.submitLog()
		}
	}

	var cmd tea.Cmd
	m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
	return m, cmd
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

	logFunc := m.cbs.Log
	return m, func() tea.Msg {
		resp, err := logFunc(issueKey, timeSpent, startTime)
		return logResultMsg{response: resp, err: err}
	}
}

func (m summaryModel) openEditForm(wl *jira.WorklogWithIssue) summaryModel {
	m.state = stateEdit
	m.editFocused = 0
	for i := range m.editInputs {
		m.editInputs[i] = textinput.New()
		m.editInputs[i].CharLimit = 128
	}
	m.editInputs[0].SetValue(wl.Worklog.TimeSpent)
	m.editInputs[1].SetValue(wl.Worklog.Started.Format("2006-01-02 15:04"))
	m.editInputs[2].SetValue(jira.WorklogCommentText(&wl.Worklog))
	m.feedback = ""
	m.isError = false
	return m
}

func (m summaryModel) updateEdit(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			m.state = stateTable
			return m, nil
		case "tab", "down":
			m.editInputs[m.editFocused].Blur()
			m.editFocused = (m.editFocused + 1) % len(m.editInputs)
			return m, m.editInputs[m.editFocused].Focus()
		case "shift+tab", "up":
			m.editInputs[m.editFocused].Blur()
			m.editFocused = (m.editFocused + len(m.editInputs) - 1) % len(m.editInputs)
			return m, m.editInputs[m.editFocused].Focus()
		case "enter":
			if m.editFocused < len(m.editInputs)-1 {
				m.editInputs[m.editFocused].Blur()
				m.editFocused++
				return m, m.editInputs[m.editFocused].Focus()
			}
			return m.submitEdit()
		}
	}
	var cmd tea.Cmd
	m.editInputs[m.editFocused], cmd = m.editInputs[m.editFocused].Update(msg)
	return m, cmd
}

func (m summaryModel) submitEdit() (tea.Model, tea.Cmd) {
	wl := m.currentWorklog()
	if wl == nil {
		m.state = stateTable
		return m, nil
	}

	timeSpent := strings.TrimSpace(m.editInputs[0].Value())
	startRaw := strings.TrimSpace(m.editInputs[1].Value())
	description := strings.TrimSpace(m.editInputs[2].Value())

	if timeSpent == "" {
		m.feedback = "Time is required"
		m.isError = true
		return m, nil
	}
	if startRaw == "" {
		m.feedback = "Start time is required"
		m.isError = true
		return m, nil
	}

	var startTime string
	if t, err := time.ParseInLocation("2006-01-02 15:04", startRaw, time.Local); err == nil {
		startTime = t.Format("2006-01-02T15:04:05.000-0700")
	} else if t, err := time.ParseInLocation("15:04", startRaw, time.Local); err == nil {
		orig := wl.Worklog.Started.Time
		full := time.Date(orig.Year(), orig.Month(), orig.Day(), t.Hour(), t.Minute(), 0, 0, time.Local)
		startTime = full.Format("2006-01-02T15:04:05.000-0700")
	} else {
		m.feedback = `Invalid start — use HH:MM or "YYYY-MM-DD HH:MM"`
		m.isError = true
		return m, nil
	}

	updateFn := m.cbs.Update
	issueKey := wl.IssueKey
	id := wl.Worklog.ID
	return m, func() tea.Msg {
		resp, err := updateFn(issueKey, id, timeSpent, startTime, description)
		return editResultMsg{response: resp, err: err}
	}
}

func (m summaryModel) updateConfirmDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "left", "h":
		if m.deleteCursor > 0 {
			m.deleteCursor--
		}
	case "right", "l", "tab":
		if m.deleteCursor < 1 {
			m.deleteCursor++
		}
	case "enter", " ":
		if m.deleteCursor == 1 {
			return m.performDelete()
		}
		m.state = stateTable
	case "y", "Y":
		return m.performDelete()
	case "n", "N", "esc", "q":
		m.state = stateTable
	}
	return m, nil
}

func (m summaryModel) performDelete() (tea.Model, tea.Cmd) {
	wl := m.currentWorklog()
	if wl == nil {
		m.state = stateTable
		return m, nil
	}
	delFn := m.cbs.Delete
	issueKey := wl.IssueKey
	id := wl.Worklog.ID
	m.state = stateTable
	return m, func() tea.Msg {
		return deleteResultMsg{err: delFn(issueKey, id)}
	}
}

func (m summaryModel) refreshCmd() tea.Cmd {
	fn := m.cbs.Refresh
	if fn == nil {
		return nil
	}
	return func() tea.Msg {
		wls, err := fn()
		return refreshResultMsg{worklogs: wls, err: err}
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (m summaryModel) currentWorklog() *jira.WorklogWithIssue {
	if m.cursor < 0 || m.cursor >= len(m.worklogs) {
		return nil
	}
	return &m.worklogs[m.cursor]
}

func (m *summaryModel) setWorklogs(worklogs []jira.WorklogWithIssue) {
	sort.Slice(worklogs, func(i, j int) bool {
		return worklogs[i].Worklog.Started.Before(worklogs[j].Worklog.Started.Time)
	})
	m.worklogs = worklogs
	m.rebuildDays()
	if m.cursor >= len(m.worklogs) {
		m.cursor = len(m.worklogs) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *summaryModel) rebuildDays() {
	m.days = nil
	m.subtotals = nil
	dayIndex := map[string]int{}
	for i, wl := range m.worklogs {
		key := wl.Worklog.Started.Format("2006-01-02")
		if _, ok := dayIndex[key]; !ok {
			dayIndex[key] = len(m.days)
			m.days = append(m.days, summaryDay{label: wl.Worklog.Started.Format("Mon, Jan 2")})
		}
		idx := dayIndex[key]
		m.days[idx].entries = append(m.days[idx].entries, summaryEntry{idx: i, wl: wl})
		m.days[idx].total += wl.Worklog.TimeSpentSeconds
	}
	grandTotal := 0
	for _, d := range m.days {
		h := d.total / 3600
		mn := (d.total % 3600) / 60
		m.subtotals = append(m.subtotals, fmt.Sprintf("  %-14s total: %dh %dm", d.label, h, mn))
		grandTotal += d.total
	}
	gh := grandTotal / 3600
	gm := (grandTotal % 3600) / 60
	m.totalLine = fmt.Sprintf("Grand total: %dh %dm (%d worklogs)", gh, gm, len(m.worklogs))
}

func (m summaryModel) renderContent() string {
	dayHeaderStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6")).Underline(true)
	dayTotalStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3"))
	timeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	spentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))

	const colWidth = 46
	const summaryWidth = 22
	colStyle := lipgloss.NewStyle().Width(colWidth).PaddingRight(2)

	dayColumns := make([]string, 0, len(m.days))
	for _, day := range m.days {
		h := day.total / 3600
		mn := (day.total % 3600) / 60

		var col strings.Builder
		col.WriteString(dayHeaderStyle.Render(day.label))
		col.WriteString("  ")
		col.WriteString(dayTotalStyle.Render(fmt.Sprintf("(%dh %dm)", h, mn)))
		col.WriteString("\n\n")
		for _, entry := range day.entries {
			prefix := "  "
			if entry.idx == m.cursor {
				prefix = summaryCursorStyle.Render("▶ ")
			}
			col.WriteString(prefix)
			col.WriteString(fmt.Sprintf("%s  %s  %s  %s\n",
				timeStyle.Render(entry.wl.Worklog.Started.Format("15:04")),
				keyStyle.Render(fmt.Sprintf("%-10s", entry.wl.IssueKey)),
				fmt.Sprintf("%-*s", summaryWidth, truncate(entry.wl.Summary, summaryWidth)),
				spentStyle.Render(entry.wl.Worklog.TimeSpent),
			))
		}
		dayColumns = append(dayColumns, colStyle.Render(col.String()))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, dayColumns...)
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m summaryModel) View() string {
	var sb strings.Builder
	sb.WriteString(m.viewTable())
	switch m.state {
	case stateForm:
		sb.WriteString("\n")
		sb.WriteString(m.viewForm())
	case stateEdit:
		sb.WriteString("\n")
		sb.WriteString(m.viewEditForm())
	case stateConfirmDelete:
		sb.WriteString("\n")
		sb.WriteString(m.viewConfirmDelete())
	default:
		sb.WriteString(lipgloss.NewStyle().Faint(true).Render("↑/↓ select • n new • e edit • d delete • r refresh • q quit"))
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
	if m.feedback != "" && m.state == stateTable {
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
	return renderFormOverlay("New Log Entry", labels, hints, m.inputs[:], m.focused, m.feedback, m.isError)
}

func (m summaryModel) viewEditForm() string {
	wl := m.currentWorklog()
	title := "Edit Worklog"
	if wl != nil {
		title = fmt.Sprintf("Edit Worklog — %s (%s)", wl.IssueKey, wl.Worklog.ID)
	}
	labels := []string{"Time Spent", "Start Time", "Description"}
	hints := []string{
		"e.g. 1h, 30m, 1h30m",
		`HH:MM or "YYYY-MM-DD HH:MM"`,
		"(empty clears existing comment)",
	}
	return renderFormOverlay(title, labels, hints, m.editInputs[:], m.editFocused, m.feedback, m.isError)
}

func renderFormOverlay(title string, labels, hints []string, inputs []textinput.Model, focused int, feedback string, isErr bool) string {
	var sb strings.Builder
	sb.WriteString(formActiveStyle.Render(title))
	sb.WriteString("\n\n")
	for i, input := range inputs {
		label := formLabelStyle.Render(labels[i] + ":")
		field := input.View()
		hint := lipgloss.NewStyle().Faint(true).Render("  " + hints[i])
		if i == focused {
			sb.WriteString(formActiveStyle.Render("▶ ") + label + field + "\n" + hint + "\n\n")
		} else {
			sb.WriteString("  " + label + field + "\n\n")
		}
	}
	if feedback != "" {
		if isErr {
			sb.WriteString(formErrorStyle.Render("  " + feedback))
		} else {
			sb.WriteString(formSuccessStyle.Render("  " + feedback))
		}
		sb.WriteString("\n\n")
	}
	sb.WriteString(lipgloss.NewStyle().Faint(true).Render("  tab/↑↓ navigate • enter next/submit • esc back"))
	sb.WriteString("\n")
	return formBorderStyle.Render(sb.String())
}

func (m summaryModel) viewConfirmDelete() string {
	wl := m.currentWorklog()
	if wl == nil {
		return ""
	}
	cancel := summaryIdleStyle.Render("Cancel")
	del := summaryIdleStyle.Render("Delete")
	if m.deleteCursor == 0 {
		cancel = summaryConfirmStyle.Render("Cancel")
	} else {
		del = summaryDangerStyle.Render("Delete")
	}
	body := fmt.Sprintf(
		"%s\n\n  %s  %s\n  %s  %s\n  %s  %s\n\n  Delete this worklog? This cannot be undone.\n  %s  %s\n\n  %s",
		formActiveStyle.Render("Confirm delete"),
		formLabelStyle.Render("Issue  :"),
		wl.IssueKey,
		formLabelStyle.Render("Started:"),
		wl.Worklog.Started.Format("Mon Jan 2 15:04"),
		formLabelStyle.Render("Spent  :"),
		wl.Worklog.TimeSpent,
		cancel, del,
		lipgloss.NewStyle().Faint(true).Render("←/→ select • enter confirm • y delete • n/esc cancel"),
	)
	return formBorderStyle.Render(body)
}

// RunSummaryTable renders an interactive Bubble Tea summary for the given worklogs.
func RunSummaryTable(worklogs []jira.WorklogWithIssue, title string, cbs SummaryCallbacks) error {
	if len(worklogs) == 0 {
		fmt.Println("No worklogs found.")
		return nil
	}

	m := summaryModel{
		title: title,
		cbs:   cbs,
	}
	m.setWorklogs(worklogs)

	const colWidth = 46
	vp := viewport.New(colWidth*maxColsInView(len(m.days)), 15)
	vp.SetHorizontalStep(colWidth)
	m.viewport = vp
	m.viewport.SetContent(m.renderContent())

	var inputs [3]textinput.Model
	for i := range inputs {
		inputs[i] = textinput.New()
		inputs[i].CharLimit = 64
	}
	inputs[0].Placeholder = "PROJ-123"
	inputs[1].Placeholder = "1h, 30m, 1h30m"
	inputs[2].Placeholder = "09:30  (leave empty for now)"
	m.inputs = inputs

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	if sm, ok := finalModel.(summaryModel); ok && sm.feedback != "" && !sm.isError {
		fmt.Println(sm.feedback)
	}

	return nil
}

func maxColsInView(n int) int {
	if n < 2 {
		return 2
	}
	if n > 4 {
		return 4
	}
	return n
}
