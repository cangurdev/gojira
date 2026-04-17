package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
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

	pivotHeaderStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	pivotSelectedStyle = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("17")).Foreground(lipgloss.Color("15"))
	pivotTotalStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3"))
	pivotZeroStyle     = lipgloss.NewStyle().Faint(true)
	pivotCursorStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))

	summaryDangerStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("1")).Padding(0, 1)
	summaryIdleStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Padding(0, 1)
	summaryConfirmStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("6")).Padding(0, 1)
)

type summaryState int

const (
	stateTable summaryState = iota
	stateDetail
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

type pivotRow struct {
	projectKey  string
	projectName string
	dayTotals   map[string]int // date key "2006-01-02" -> seconds
	total       int
}

type summaryModel struct {
	title      string
	worklogs   []jira.WorklogWithIssue
	rows       []pivotRow
	cols       []string // sorted date keys
	colLabels  []string // "Mon 14"
	colTotals  []int    // per-col total seconds
	grandTotal int

	cursorRow int
	cursorCol int // 0 = Total col, 1+ = day cols

	state summaryState

	// detail overlay
	detailWorklogs []jira.WorklogWithIssue
	detailCursor   int
	detailTitle    string

	// new-log form
	inputs  [3]textinput.Model
	focused int

	// edit form
	editInputs  [3]textinput.Model
	editFocused int

	// delete confirm
	deleteCursor int

	cbs      SummaryCallbacks
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

type deleteResultMsg struct{ err error }

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
		m.state = stateDetail
		return m, m.refreshCmd()

	case deleteResultMsg:
		if msg.err != nil {
			m.feedback = msg.err.Error()
			m.isError = true
			return m, nil
		}
		m.feedback = "✓ Deleted worklog"
		m.isError = false
		m.state = stateDetail
		return m, m.refreshCmd()

	case refreshResultMsg:
		if msg.err != nil {
			m.feedback = msg.err.Error()
			m.isError = true
			return m, nil
		}
		m.setWorklogs(msg.worklogs)
		if m.state == stateDetail {
			m.rebuildDetail()
		}
		return m, nil
	}

	switch m.state {
	case stateTable:
		return m.updateTable(msg)
	case stateDetail:
		return m.updateDetail(msg)
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
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "q", "ctrl+c", "esc":
		return m, tea.Quit
	case "up", "k":
		if m.cursorRow > 0 {
			m.cursorRow--
		}
	case "down", "j":
		if m.cursorRow < len(m.rows)-1 {
			m.cursorRow++
		}
	case "left", "h":
		if m.cursorCol > 0 {
			m.cursorCol--
		}
	case "right", "l":
		if m.cursorCol < len(m.cols) {
			m.cursorCol++
		}
	case "enter", " ":
		if len(m.rows) > 0 {
			m.state = stateDetail
			m.detailCursor = 0
			m.rebuildDetail()
		}
	case "n":
		m.state = stateForm
		m.focused = 0
		m.feedback = ""
		m.isError = false
		for i := range m.inputs {
			m.inputs[i].SetValue("")
		}
		return m, m.inputs[0].Focus()
	case "r":
		m.feedback = "Refreshing..."
		m.isError = false
		return m, m.refreshCmd()
	}
	return m, nil
}

func (m summaryModel) updateDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "esc", "q":
		m.state = stateTable
		m.feedback = ""
	case "up", "k":
		if m.detailCursor > 0 {
			m.detailCursor--
		}
	case "down", "j":
		if m.detailCursor < len(m.detailWorklogs)-1 {
			m.detailCursor++
		}
	case "e":
		if wl := m.currentDetailWorklog(); wl != nil {
			m = m.openEditForm(wl)
			return m, m.editInputs[0].Focus()
		}
	case "d":
		if m.currentDetailWorklog() != nil {
			m.state = stateConfirmDelete
			m.deleteCursor = 0
			m.feedback = ""
			m.isError = false
		}
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
	return m, nil
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
			m.state = stateDetail
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
	wl := m.currentDetailWorklog()
	if wl == nil {
		m.state = stateDetail
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
		m.state = stateDetail
	case "y", "Y":
		return m.performDelete()
	case "n", "N", "esc", "q":
		m.state = stateDetail
	}
	return m, nil
}

func (m summaryModel) performDelete() (tea.Model, tea.Cmd) {
	wl := m.currentDetailWorklog()
	if wl == nil {
		m.state = stateDetail
		return m, nil
	}
	delFn := m.cbs.Delete
	issueKey := wl.IssueKey
	id := wl.Worklog.ID
	m.state = stateDetail
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

func (m summaryModel) currentDetailWorklog() *jira.WorklogWithIssue {
	if m.detailCursor < 0 || m.detailCursor >= len(m.detailWorklogs) {
		return nil
	}
	return &m.detailWorklogs[m.detailCursor]
}

func (m *summaryModel) setWorklogs(worklogs []jira.WorklogWithIssue) {
	m.worklogs = worklogs
	m.rebuildPivot()
}

func (m *summaryModel) rebuildPivot() {
	dateSet := map[string]bool{}
	issueMap := map[string]*pivotRow{}
	var issueOrder []string

	for _, wl := range m.worklogs {
		dk := wl.Worklog.Started.Format("2006-01-02")
		dateSet[dk] = true
		pk := wl.ProjectKey
		if pk == "" {
			// fallback: derive from issue key
			if i := strings.Index(wl.IssueKey, "-"); i > 0 {
				pk = wl.IssueKey[:i]
			} else {
				pk = wl.IssueKey
			}
		}
		if _, ok := issueMap[pk]; !ok {
			issueOrder = append(issueOrder, pk)
			name := wl.ProjectName
			if name == "" {
				name = pk
			}
			issueMap[pk] = &pivotRow{
				projectKey:  pk,
				projectName: name,
				dayTotals:   map[string]int{},
			}
		}
		r := issueMap[pk]
		r.dayTotals[dk] += wl.Worklog.TimeSpentSeconds
		r.total += wl.Worklog.TimeSpentSeconds
	}

	cols := make([]string, 0, len(dateSet))
	for dk := range dateSet {
		cols = append(cols, dk)
	}
	sort.Strings(cols)
	m.cols = cols

	m.colLabels = make([]string, len(cols))
	for i, dk := range cols {
		t, _ := time.Parse("2006-01-02", dk)
		m.colLabels[i] = t.Format("Mon 2")
	}

	m.colTotals = make([]int, len(cols))
	for i, dk := range cols {
		for _, r := range issueMap {
			m.colTotals[i] += r.dayTotals[dk]
		}
	}

	m.grandTotal = 0
	m.rows = make([]pivotRow, 0, len(issueOrder))
	for _, ik := range issueOrder {
		m.rows = append(m.rows, *issueMap[ik])
		m.grandTotal += issueMap[ik].total
	}
	sort.Slice(m.rows, func(i, j int) bool {
		return m.rows[i].total > m.rows[j].total
	})

	if m.cursorRow >= len(m.rows) {
		m.cursorRow = len(m.rows) - 1
	}
	if m.cursorRow < 0 {
		m.cursorRow = 0
	}
	if m.cursorCol > len(m.cols) {
		m.cursorCol = len(m.cols)
	}
}

func (m *summaryModel) rebuildDetail() {
	if m.cursorRow >= len(m.rows) {
		m.detailWorklogs = nil
		return
	}
	row := m.rows[m.cursorRow]
	var filtered []jira.WorklogWithIssue
	for _, wl := range m.worklogs {
		pk := wl.ProjectKey
		if pk == "" {
			if i := strings.Index(wl.IssueKey, "-"); i > 0 {
				pk = wl.IssueKey[:i]
			} else {
				pk = wl.IssueKey
			}
		}
		if pk != row.projectKey {
			continue
		}
		if m.cursorCol > 0 {
			dk := m.cols[m.cursorCol-1]
			if wl.Worklog.Started.Format("2006-01-02") != dk {
				continue
			}
		}
		filtered = append(filtered, wl)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Worklog.Started.Before(filtered[j].Worklog.Started.Time)
	})
	m.detailWorklogs = filtered

	if m.cursorCol == 0 {
		m.detailTitle = fmt.Sprintf("%s — tüm worklogs", row.projectName)
	} else {
		dk := m.cols[m.cursorCol-1]
		t, _ := time.Parse("2006-01-02", dk)
		m.detailTitle = fmt.Sprintf("%s — %s", row.projectName, t.Format("Mon Jan 2"))
	}

	if m.detailCursor >= len(m.detailWorklogs) {
		m.detailCursor = len(m.detailWorklogs) - 1
	}
	if m.detailCursor < 0 {
		m.detailCursor = 0
	}
}

func fmtDur(secs int) string {
	if secs == 0 {
		return "─"
	}
	h := secs / 3600
	mn := (secs % 3600) / 60
	if h == 0 {
		return fmt.Sprintf("%dm", mn)
	}
	if mn == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %dm", h, mn)
}

// ── Render ────────────────────────────────────────────────────────────────────

const (
	issueColW = 26
	timeColW  = 9
)

func (m summaryModel) renderPivot() string {
	issueStyle := func(w int) lipgloss.Style { return lipgloss.NewStyle().Width(w) }
	timeStyle := lipgloss.NewStyle().Width(timeColW)

	// Header
	var hdr strings.Builder
	hdr.WriteString(pivotHeaderStyle.Render(issueStyle(issueColW).Render("Issue")))
	hdr.WriteString(pivotHeaderStyle.Render(timeStyle.Render("Total")))
	for _, lbl := range m.colLabels {
		hdr.WriteString(pivotHeaderStyle.Render(timeStyle.Render(lbl)))
	}

	sepLen := issueColW + timeColW*(1+len(m.cols))
	sep := strings.Repeat("─", sepLen)

	var sb strings.Builder
	sb.WriteString(hdr.String())
	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Faint(true).Render(sep))
	sb.WriteString("\n")

	for ri, row := range m.rows {
		isRow := ri == m.cursorRow && m.state == stateTable

		label := truncate(row.projectName, issueColW-2)
		prefix := "  "
		if isRow {
			prefix = pivotCursorStyle.Render("▶ ")
		}
		var line strings.Builder
		line.WriteString(prefix)
		line.WriteString(issueStyle(issueColW - 2).Render(label))

		// total cell
		totalStr := fmtDur(row.total)
		if isRow && m.cursorCol == 0 {
			line.WriteString(pivotSelectedStyle.Render(timeStyle.Render(totalStr)))
		} else {
			line.WriteString(timeStyle.Render(totalStr))
		}

		// day cells
		for ci, dk := range m.cols {
			secs := row.dayTotals[dk]
			cellStr := fmtDur(secs)
			selected := isRow && m.cursorCol == ci+1
			if selected {
				line.WriteString(pivotSelectedStyle.Render(timeStyle.Render(cellStr)))
			} else if secs == 0 {
				line.WriteString(pivotZeroStyle.Render(timeStyle.Render(cellStr)))
			} else {
				line.WriteString(timeStyle.Render(cellStr))
			}
		}
		sb.WriteString(line.String())
		sb.WriteString("\n")
	}

	sb.WriteString(lipgloss.NewStyle().Faint(true).Render(sep))
	sb.WriteString("\n")

	// total row
	var tl strings.Builder
	tl.WriteString("  ")
	tl.WriteString(pivotTotalStyle.Render(issueStyle(issueColW - 2).Render("Total")))
	tl.WriteString(pivotTotalStyle.Render(timeStyle.Render(fmtDur(m.grandTotal))))
	for i := range m.cols {
		tl.WriteString(pivotTotalStyle.Render(timeStyle.Render(fmtDur(m.colTotals[i]))))
	}
	sb.WriteString(tl.String())
	sb.WriteString("\n")

	return sb.String()
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m summaryModel) View() string {
	var sb strings.Builder
	sb.WriteString(headerStyle.Render(m.title))
	sb.WriteString("\n\n")
	sb.WriteString(m.renderPivot())
	sb.WriteString("\n")

	if m.feedback != "" && m.state == stateTable {
		if m.isError {
			sb.WriteString(formErrorStyle.Render(m.feedback))
		} else {
			sb.WriteString(formSuccessStyle.Render(m.feedback))
		}
		sb.WriteString("\n\n")
	}

	switch m.state {
	case stateTable:
		sb.WriteString(lipgloss.NewStyle().Faint(true).Render("↑/↓ satır • ←/→ sütun • enter detay • n yeni log • r yenile • q çıkış"))
		sb.WriteString("\n")
	case stateDetail:
		sb.WriteString("\n")
		sb.WriteString(m.viewDetail())
	case stateForm:
		sb.WriteString("\n")
		sb.WriteString(m.viewForm())
	case stateEdit:
		sb.WriteString("\n")
		sb.WriteString(m.viewEditForm())
	case stateConfirmDelete:
		sb.WriteString("\n")
		sb.WriteString(m.viewConfirmDelete())
	}
	return sb.String()
}

func (m summaryModel) viewDetail() string {
	var sb strings.Builder
	sb.WriteString(formActiveStyle.Render(m.detailTitle))
	sb.WriteString("\n\n")

	timeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	spentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Width(8)
	descStyle := lipgloss.NewStyle().Faint(true)
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true)
	summaryStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))

	if len(m.detailWorklogs) == 0 {
		sb.WriteString(lipgloss.NewStyle().Faint(true).Render("Bu hücrede worklog yok."))
		sb.WriteString("\n\n")
	}

	lastKey := ""
	for i, wl := range m.detailWorklogs {
		if wl.IssueKey != lastKey {
			lastKey = wl.IssueKey
			sb.WriteString(fmt.Sprintf("  %s  %s\n",
				keyStyle.Render(wl.IssueKey),
				summaryStyle.Render(truncate(wl.Summary, 60)),
			))
		}
		prefix := "    "
		if i == m.detailCursor {
			prefix = "  " + pivotCursorStyle.Render("▶ ")
		}
		desc := jira.WorklogCommentText(&wl.Worklog)
		if desc == "" {
			desc = "─"
		}
		line := fmt.Sprintf("%s  %s  %s",
			timeStyle.Render(wl.Worklog.Started.Format("15:04")),
			spentStyle.Render(wl.Worklog.TimeSpent),
			descStyle.Render(truncate(desc, 50)),
		)
		sb.WriteString(prefix + line + "\n")
	}

	if m.feedback != "" {
		sb.WriteString("\n")
		if m.isError {
			sb.WriteString(formErrorStyle.Render(m.feedback))
		} else {
			sb.WriteString(formSuccessStyle.Render(m.feedback))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Faint(true).Render("↑/↓ seç • e düzenle • d sil • n yeni • esc geri"))
	sb.WriteString("\n")

	return formBorderStyle.Render(sb.String())
}

func (m summaryModel) viewForm() string {
	labels := []string{"Issue Key", "Time Spent", "Start Time"}
	hints := []string{"e.g. PROJ-123", "e.g. 1h, 30m, 1h30m", "e.g. 09:30 (optional)"}
	return renderFormOverlay("New Log Entry", labels, hints, m.inputs[:], m.focused, m.feedback, m.isError)
}

func (m summaryModel) viewEditForm() string {
	wl := m.currentDetailWorklog()
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
	wl := m.currentDetailWorklog()
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
		formLabelStyle.Render("Issue  :"), wl.IssueKey,
		formLabelStyle.Render("Started:"), wl.Worklog.Started.Format("Mon Jan 2 15:04"),
		formLabelStyle.Render("Spent  :"), wl.Worklog.TimeSpent,
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

	m := summaryModel{title: title, cbs: cbs}
	m.setWorklogs(worklogs)

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
