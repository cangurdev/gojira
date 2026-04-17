package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"gojira/internal/jira"
)

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	wlTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	wlBorderStyle   = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("8"))
	wlHintStyle     = lipgloss.NewStyle().Faint(true)
	wlStatusOK      = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	wlStatusErr     = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	wlOverlayStyle  = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("6")).Padding(1, 2)
	wlDangerStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("1")).Padding(0, 1)
	wlDangerIdle    = lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Padding(0, 1)
)

// ── Callbacks ─────────────────────────────────────────────────────────────────

type WorklogCallbacks struct {
	Refresh func() ([]jira.WorklogWithIssue, error)
	Update  func(issueKey, worklogID, timeSpent, started, description string) (*jira.WorklogResponse, error)
	Delete  func(issueKey, worklogID string) error
}

// ── Model ─────────────────────────────────────────────────────────────────────

type worklogMode int

const (
	worklogModeList worklogMode = iota
	worklogModeEdit
	worklogModeConfirmDelete
)

type worklogEditModel struct {
	worklogs []jira.WorklogWithIssue
	table    table.Model
	title    string
	cbs      WorklogCallbacks

	mode worklogMode

	// edit form
	editInputs  [3]textinput.Model // time, start, description
	editFocused int

	// delete confirm
	deleteCursor int // 0 = Cancel, 1 = Delete

	status string
	isErr  bool
}

type wlActionMsg struct {
	ok  string
	err error
}

type wlRefreshMsg struct {
	worklogs []jira.WorklogWithIssue
	err      error
}

type wlClearStatusMsg struct{}

func (m worklogEditModel) Init() tea.Cmd { return nil }

func (m worklogEditModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case wlActionMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
			m.isErr = true
			return m, wlClearAfter(4 * time.Second)
		}
		m.status = msg.ok
		m.isErr = false
		return m, tea.Batch(m.refreshCmd(), wlClearAfter(3*time.Second))

	case wlRefreshMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
			m.isErr = true
			return m, wlClearAfter(4 * time.Second)
		}
		// Preserve cursor by worklog ID
		selectedID := ""
		if wl := m.currentWorklog(); wl != nil {
			selectedID = wl.Worklog.ID
		}
		m.worklogs = sortedWorklogs(msg.worklogs)
		m.table.SetRows(buildWorklogRows(m.worklogs))
		if selectedID != "" {
			for i, w := range m.worklogs {
				if w.Worklog.ID == selectedID {
					m.table.SetCursor(i)
					break
				}
			}
		}
		return m, nil

	case wlClearStatusMsg:
		m.status = ""
		m.isErr = false
		return m, nil
	}

	switch m.mode {
	case worklogModeList:
		return m.updateList(msg)
	case worklogModeEdit:
		return m.updateEdit(msg)
	case worklogModeConfirmDelete:
		return m.updateConfirmDelete(msg)
	}
	return m, nil
}

func (m worklogEditModel) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "e":
			if wl := m.currentWorklog(); wl != nil {
				m = m.openEditForm(wl)
				return m, m.editInputs[0].Focus()
			}
		case "d":
			if m.currentWorklog() != nil {
				m.mode = worklogModeConfirmDelete
				m.deleteCursor = 0 // default to Cancel
				return m, nil
			}
		case "r":
			m.status = "Refreshing..."
			m.isErr = false
			return m, m.refreshCmd()
		}
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m worklogEditModel) openEditForm(wl *jira.WorklogWithIssue) worklogEditModel {
	m.mode = worklogModeEdit
	m.editFocused = 0

	for i := range m.editInputs {
		m.editInputs[i] = textinput.New()
		m.editInputs[i].CharLimit = 128
	}
	m.editInputs[0].SetValue(wl.Worklog.TimeSpent)
	m.editInputs[1].SetValue(wl.Worklog.Started.Format("2006-01-02 15:04"))
	m.editInputs[2].SetValue(jira.WorklogCommentText(&wl.Worklog))
	return m
}

func (m worklogEditModel) updateEdit(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			m.mode = worklogModeList
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

func (m worklogEditModel) updateConfirmDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			wl := m.currentWorklog()
			if wl == nil {
				m.mode = worklogModeList
				return m, nil
			}
			m.mode = worklogModeList
			return m, m.deleteCmd(wl.IssueKey, wl.Worklog.ID)
		}
		m.mode = worklogModeList
		return m, nil
	case "y", "Y":
		wl := m.currentWorklog()
		if wl == nil {
			m.mode = worklogModeList
			return m, nil
		}
		m.mode = worklogModeList
		return m, m.deleteCmd(wl.IssueKey, wl.Worklog.ID)
	case "n", "N", "esc", "q":
		m.mode = worklogModeList
		return m, nil
	}
	return m, nil
}

func (m worklogEditModel) submitEdit() (tea.Model, tea.Cmd) {
	wl := m.currentWorklog()
	if wl == nil {
		m.mode = worklogModeList
		return m, nil
	}

	timeSpent := strings.TrimSpace(m.editInputs[0].Value())
	startRaw := strings.TrimSpace(m.editInputs[1].Value())
	description := strings.TrimSpace(m.editInputs[2].Value())

	if timeSpent == "" {
		m.status = "Time is required"
		m.isErr = true
		return m, nil
	}

	var startTime string
	if startRaw == "" {
		m.status = "Start time is required"
		m.isErr = true
		return m, nil
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04", startRaw, time.Local); err == nil {
		startTime = t.Format("2006-01-02T15:04:05.000-0700")
	} else if t, err := time.ParseInLocation("15:04", startRaw, time.Local); err == nil {
		orig := wl.Worklog.Started.Time
		full := time.Date(orig.Year(), orig.Month(), orig.Day(), t.Hour(), t.Minute(), 0, 0, time.Local)
		startTime = full.Format("2006-01-02T15:04:05.000-0700")
	} else {
		m.status = `Invalid start — use HH:MM or "YYYY-MM-DD HH:MM"`
		m.isErr = true
		return m, nil
	}

	m.mode = worklogModeList
	return m, m.updateCmd(wl.IssueKey, wl.Worklog.ID, timeSpent, startTime, description)
}

// ── Commands ──────────────────────────────────────────────────────────────────

func (m worklogEditModel) refreshCmd() tea.Cmd {
	fn := m.cbs.Refresh
	return func() tea.Msg {
		wls, err := fn()
		return wlRefreshMsg{worklogs: wls, err: err}
	}
}

func (m worklogEditModel) updateCmd(issueKey, id, timeSpent, started, desc string) tea.Cmd {
	fn := m.cbs.Update
	return func() tea.Msg {
		resp, err := fn(issueKey, id, timeSpent, started, desc)
		if err != nil {
			return wlActionMsg{err: err}
		}
		return wlActionMsg{ok: fmt.Sprintf("✓ Updated worklog on %s (%s)", issueKey, resp.TimeSpent)}
	}
}

func (m worklogEditModel) deleteCmd(issueKey, id string) tea.Cmd {
	fn := m.cbs.Delete
	return func() tea.Msg {
		if err := fn(issueKey, id); err != nil {
			return wlActionMsg{err: err}
		}
		return wlActionMsg{ok: fmt.Sprintf("✓ Deleted worklog on %s", issueKey)}
	}
}

func wlClearAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return wlClearStatusMsg{}
	})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (m worklogEditModel) currentWorklog() *jira.WorklogWithIssue {
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.worklogs) {
		return nil
	}
	return &m.worklogs[idx]
}

func sortedWorklogs(wls []jira.WorklogWithIssue) []jira.WorklogWithIssue {
	out := make([]jira.WorklogWithIssue, len(wls))
	copy(out, wls)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Worklog.Started.Before(out[j].Worklog.Started.Time)
	})
	return out
}

func buildWorklogRows(wls []jira.WorklogWithIssue) []table.Row {
	rows := make([]table.Row, len(wls))
	for i, wl := range wls {
		rows[i] = table.Row{
			wl.Worklog.Started.Format("Mon Jan 2"),
			wl.Worklog.Started.Format("15:04"),
			wl.IssueKey,
			truncate(wl.Summary, 40),
			wl.Worklog.TimeSpent,
		}
	}
	return rows
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m worklogEditModel) View() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(wlTitleStyle.Render(m.title))
	sb.WriteString("\n\n")
	sb.WriteString(wlBorderStyle.Render(m.table.View()))
	sb.WriteString("\n")
	if m.status != "" {
		if m.isErr {
			sb.WriteString(wlStatusErr.Render(m.status))
		} else {
			sb.WriteString(wlStatusOK.Render(m.status))
		}
		sb.WriteString("\n")
	}
	sb.WriteString(wlHintStyle.Render(fmt.Sprintf("%d worklogs  •  ↑/↓ navigate  •  e edit  •  d delete  •  r refresh  •  q quit", len(m.worklogs))))
	sb.WriteString("\n")

	base := sb.String()
	switch m.mode {
	case worklogModeEdit:
		return base + "\n" + m.viewEditOverlay()
	case worklogModeConfirmDelete:
		return base + "\n" + m.viewDeleteOverlay()
	}
	return base
}

func (m worklogEditModel) viewEditOverlay() string {
	wl := m.currentWorklog()
	if wl == nil {
		return ""
	}
	labels := []string{"Time Spent", "Start Time", "Description"}
	hints := []string{
		"e.g. 1h, 30m, 1h30m",
		`HH:MM or "YYYY-MM-DD HH:MM"`,
		"(leave empty to keep none — existing will be cleared)",
	}

	var sb strings.Builder
	sb.WriteString(wlTitleStyle.Render(fmt.Sprintf("Edit worklog — %s  (%s)", wl.IssueKey, wl.Worklog.ID)))
	sb.WriteString("\n\n")
	for i, input := range m.editInputs {
		label := formLabelStyle.Render(labels[i] + ":")
		field := input.View()
		hint := lipgloss.NewStyle().Faint(true).Render("  " + hints[i])
		if i == m.editFocused {
			sb.WriteString(formActiveStyle.Render("▶ ") + label + field + "\n" + hint + "\n\n")
		} else {
			sb.WriteString("  " + label + field + "\n\n")
		}
	}
	sb.WriteString(wlHintStyle.Render("tab/↑↓ navigate • enter next/submit • esc cancel"))
	return wlOverlayStyle.Render(sb.String())
}

func (m worklogEditModel) viewDeleteOverlay() string {
	wl := m.currentWorklog()
	if wl == nil {
		return ""
	}

	cancelStyle := wlDangerIdle
	deleteStyle := wlDangerIdle
	if m.deleteCursor == 0 {
		cancelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("6")).Padding(0, 1)
	} else {
		deleteStyle = wlDangerStyle
	}

	body := fmt.Sprintf(
		"%s\n\n  %s  %s\n  %s  %s\n  %s  %s\n\n  Delete this worklog? This cannot be undone.\n  %s  %s\n\n  %s",
		wlTitleStyle.Render("Confirm delete"),
		formLabelStyle.Render("Issue  :"),
		wl.IssueKey,
		formLabelStyle.Render("Started:"),
		wl.Worklog.Started.Format("Mon Jan 2 15:04"),
		formLabelStyle.Render("Spent  :"),
		wl.Worklog.TimeSpent,
		cancelStyle.Render("Cancel"),
		deleteStyle.Render("Delete"),
		wlHintStyle.Render("←/→ select • enter confirm • y delete • n/esc cancel"),
	)
	return wlOverlayStyle.Render(body)
}

// ── Entry point ───────────────────────────────────────────────────────────────

// RunWorklogEditor launches the interactive worklog editor.
func RunWorklogEditor(title string, initial []jira.WorklogWithIssue, cbs WorklogCallbacks) error {
	wls := sortedWorklogs(initial)

	columns := []table.Column{
		{Title: "Day", Width: 14},
		{Title: "Time", Width: 6},
		{Title: "Issue", Width: 14},
		{Title: "Summary", Width: 40},
		{Title: "Spent", Width: 10},
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(buildWorklogRows(wls)),
		table.WithFocused(true),
		table.WithHeight(15),
	)
	s := table.DefaultStyles()
	s.Header = s.Header.Bold(true).Foreground(lipgloss.Color("6"))
	s.Selected = s.Selected.Foreground(lipgloss.Color("0")).Background(lipgloss.Color("6"))
	t.SetStyles(s)

	m := worklogEditModel{
		worklogs: wls,
		table:    t,
		title:    title,
		cbs:      cbs,
	}

	_, err := tea.NewProgram(m).Run()
	return err
}
