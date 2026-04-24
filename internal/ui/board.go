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

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	boardTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	boardColHeader     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6")).Underline(true)
	boardColCount      = lipgloss.NewStyle().Faint(true)
	boardCardStyle     = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("8")).Padding(0, 1)
	boardCardSelected  = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("6")).Padding(0, 1).Bold(true)
	boardCardMineStyle = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("3")).Padding(0, 1)
	boardKeyStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true)
	boardAssigneeStyle = lipgloss.NewStyle().Faint(true)
	boardHintStyle     = lipgloss.NewStyle().Faint(true)
	boardStatusOK      = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	boardStatusErr     = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	boardOverlayStyle  = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("6")).Padding(1, 2)
)

// ── Callbacks ─────────────────────────────────────────────────────────────────

// BoardCallbacks wires TUI actions to Jira API calls. All functions run on a
// goroutine via tea.Cmd — return errors as-is so the model can show them.
type BoardCallbacks struct {
	FetchIssues      func() ([]jira.Issue, error)
	FetchTransitions func(issueKey string) ([]jira.Transition, error)
	DoTransition     func(issueKey, transitionID string) error
	AddWorklog       func(issueKey, timeSpent, startTime, description string) (*jira.WorklogResponse, error)
	CreateBranch     func(issue jira.Issue) (string, error)
	OpenInBrowser    func(issueKey string)
	CurrentUserID    string
	BoardColumns     []jira.BoardColumn
}

// ── Model ─────────────────────────────────────────────────────────────────────

type boardMode int

const (
	boardModeNormal boardMode = iota
	boardModeMove
	boardModeWorklog
	boardModeHelp
)

type boardColumn struct {
	status string
	cards  []jira.Issue
}

type boardMoveOption struct {
	label      string
	transition jira.Transition
}

type boardModel struct {
	title string
	cbs   BoardCallbacks

	allIssues  []jira.Issue
	columns    []boardColumn
	colOffsets []int // scroll offset per column (index = first visible card)
	mineOnly   bool

	cursorCol int
	cursorRow int

	mode boardMode

	// move overlay
	moveOptions []boardMoveOption
	moveCursor  int

	// worklog overlay
	wlInputs  [2]textinput.Model
	wlFocused int

	status     string
	isErr      bool
	statusTime time.Time

	width  int
	height int
	ready  bool
}

// ── Messages ──────────────────────────────────────────────────────────────────

type boardIssuesMsg struct {
	issues []jira.Issue
	err    error
}

type boardTransitionsMsg struct {
	transitions []jira.Transition
	err         error
}

type boardActionMsg struct {
	ok      string
	err     error
	refresh bool
}

type boardClearStatusMsg time.Time

// ── Commands ──────────────────────────────────────────────────────────────────

func (m boardModel) fetchIssues() tea.Cmd {
	fn := m.cbs.FetchIssues
	return func() tea.Msg {
		issues, err := fn()
		return boardIssuesMsg{issues: issues, err: err}
	}
}

func (m boardModel) fetchTransitions(issueKey string) tea.Cmd {
	fn := m.cbs.FetchTransitions
	return func() tea.Msg {
		ts, err := fn(issueKey)
		return boardTransitionsMsg{transitions: ts, err: err}
	}
}

func (m boardModel) doTransitionCmd(issueKey, transitionID, transitionName string) tea.Cmd {
	fn := m.cbs.DoTransition
	return func() tea.Msg {
		if err := fn(issueKey, transitionID); err != nil {
			return boardActionMsg{err: err}
		}
		return boardActionMsg{ok: fmt.Sprintf("✓ %s → %s", issueKey, transitionName), refresh: true}
	}
}

func (m boardModel) addWorklogCmd(issueKey, timeSpent, startTime string) tea.Cmd {
	fn := m.cbs.AddWorklog
	return func() tea.Msg {
		resp, err := fn(issueKey, timeSpent, startTime, "")
		if err != nil {
			return boardActionMsg{err: err}
		}
		return boardActionMsg{ok: fmt.Sprintf("✓ Logged %s to %s", resp.TimeSpent, issueKey), refresh: true}
	}
}

func (m boardModel) createBranchCmd(issue jira.Issue) tea.Cmd {
	fn := m.cbs.CreateBranch
	return func() tea.Msg {
		if fn == nil {
			return boardActionMsg{err: fmt.Errorf("branch creation is not configured")}
		}

		branchName, err := fn(issue)
		if err != nil {
			return boardActionMsg{err: err}
		}

		return boardActionMsg{ok: fmt.Sprintf("✓ Created branch %s", branchName)}
	}
}

func clearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return boardClearStatusMsg(t)
	})
}

// ── Init/Update/View ──────────────────────────────────────────────────────────

func (m boardModel) Init() tea.Cmd { return nil }

func (m boardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case boardIssuesMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
			m.isErr = true
			return m, clearStatusAfter(4 * time.Second)
		}
		m.allIssues = msg.issues
		m.rebuildColumns()
		m.clampCursor()
		m.status = "Refreshed"
		m.isErr = false
		return m, clearStatusAfter(2 * time.Second)

	case boardTransitionsMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
			m.isErr = true
			m.mode = boardModeNormal
			return m, clearStatusAfter(4 * time.Second)
		}
		m.moveOptions = m.buildMoveOptions(msg.transitions)
		m.moveCursor = 0
		if len(m.moveOptions) == 0 {
			m.mode = boardModeNormal
			m.status = "No matching target columns available"
			m.isErr = true
			return m, clearStatusAfter(3 * time.Second)
		}
		return m, nil

	case boardActionMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
			m.isErr = true
			return m, clearStatusAfter(4 * time.Second)
		}
		m.status = msg.ok
		m.isErr = false
		if msg.refresh {
			// After mutating actions, refetch issues to reflect new state.
			return m, tea.Batch(m.fetchIssues(), clearStatusAfter(3*time.Second))
		}
		return m, clearStatusAfter(3 * time.Second)

	case boardClearStatusMsg:
		m.status = ""
		m.isErr = false
		return m, nil
	}

	switch m.mode {
	case boardModeNormal:
		return m.updateNormal(msg)
	case boardModeMove:
		return m.updateMove(msg)
	case boardModeWorklog:
		return m.updateWorklog(msg)
	case boardModeHelp:
		return m.updateHelp(msg)
	}
	return m, nil
}

func (m boardModel) updateNormal(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "q", "ctrl+c", "esc":
		return m, tea.Quit
	case "left", "h":
		if m.cursorCol > 0 {
			m.cursorCol--
			m.clampRow()
			m.ensureCursorVisible()
		}
	case "right", "l":
		if m.cursorCol < len(m.columns)-1 {
			m.cursorCol++
			m.clampRow()
			m.ensureCursorVisible()
		}
	case "up", "k":
		if m.cursorRow > 0 {
			m.cursorRow--
			m.ensureCursorVisible()
		}
	case "down", "j":
		col := m.currentColumn()
		if col != nil && m.cursorRow < len(col.cards)-1 {
			m.cursorRow++
			m.ensureCursorVisible()
		}
	case "pgup", "ctrl+u":
		per := m.cardsPerView()
		m.cursorRow -= per
		if m.cursorRow < 0 {
			m.cursorRow = 0
		}
		m.ensureCursorVisible()
	case "pgdown", "ctrl+d":
		col := m.currentColumn()
		if col != nil {
			per := m.cardsPerView()
			m.cursorRow += per
			if m.cursorRow >= len(col.cards) {
				m.cursorRow = len(col.cards) - 1
			}
			m.ensureCursorVisible()
		}
	case "g", "home":
		m.cursorRow = 0
		m.ensureCursorVisible()
	case "G", "end":
		col := m.currentColumn()
		if col != nil && len(col.cards) > 0 {
			m.cursorRow = len(col.cards) - 1
			m.ensureCursorVisible()
		}
	case "a":
		m.mineOnly = !m.mineOnly
		m.rebuildColumns()
		m.clampCursor()
	case "r":
		m.status = "Refreshing..."
		m.isErr = false
		return m, m.fetchIssues()
	case "?":
		m.mode = boardModeHelp
	case "o":
		if issue := m.currentIssue(); issue != nil && m.cbs.OpenInBrowser != nil {
			m.cbs.OpenInBrowser(issue.Key)
			m.status = fmt.Sprintf("Opened %s", issue.Key)
			m.isErr = false
			return m, clearStatusAfter(2 * time.Second)
		}
	case "b":
		if issue := m.currentIssue(); issue != nil {
			return m, m.createBranchCmd(*issue)
		}
	case "m":
		if issue := m.currentIssue(); issue != nil {
			m.mode = boardModeMove
			m.moveOptions = nil
			m.status = "Loading transitions..."
			return m, m.fetchTransitions(issue.Key)
		}
	case "w":
		if m.currentIssue() == nil {
			return m, nil
		}
		m.mode = boardModeWorklog
		m.wlFocused = 0
		for i := range m.wlInputs {
			m.wlInputs[i] = textinput.New()
			m.wlInputs[i].CharLimit = 32
		}
		m.wlInputs[0].Placeholder = "1h, 30m, 1h30m"
		m.wlInputs[1].Placeholder = "09:30 (optional)"
		return m, m.wlInputs[0].Focus()
	}
	return m, nil
}

func (m boardModel) updateMove(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "esc", "q":
		m.mode = boardModeNormal
		m.status = ""
		return m, nil
	case "up", "k":
		if m.moveCursor > 0 {
			m.moveCursor--
		}
	case "down", "j":
		if m.moveCursor < len(m.moveOptions)-1 {
			m.moveCursor++
		}
	case "enter", " ":
		issue := m.currentIssue()
		if issue == nil || len(m.moveOptions) == 0 {
			m.mode = boardModeNormal
			return m, nil
		}
		option := m.moveOptions[m.moveCursor]
		t := option.transition
		m.mode = boardModeNormal
		return m, m.doTransitionCmd(issue.Key, t.ID, option.label)
	}
	return m, nil
}

func (m boardModel) updateWorklog(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if ok {
		switch km.String() {
		case "esc":
			m.mode = boardModeNormal
			return m, nil
		case "tab", "down":
			m.wlInputs[m.wlFocused].Blur()
			m.wlFocused = (m.wlFocused + 1) % len(m.wlInputs)
			return m, m.wlInputs[m.wlFocused].Focus()
		case "shift+tab", "up":
			m.wlInputs[m.wlFocused].Blur()
			m.wlFocused = (m.wlFocused + len(m.wlInputs) - 1) % len(m.wlInputs)
			return m, m.wlInputs[m.wlFocused].Focus()
		case "enter":
			if m.wlFocused == 0 {
				m.wlInputs[0].Blur()
				m.wlFocused = 1
				return m, m.wlInputs[1].Focus()
			}
			return m.submitWorklog()
		}
	}

	var cmd tea.Cmd
	m.wlInputs[m.wlFocused], cmd = m.wlInputs[m.wlFocused].Update(msg)
	return m, cmd
}

func (m boardModel) updateHelp(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.KeyMsg); ok {
		m.mode = boardModeNormal
	}
	return m, nil
}

func (m boardModel) submitWorklog() (tea.Model, tea.Cmd) {
	issue := m.currentIssue()
	if issue == nil {
		m.mode = boardModeNormal
		return m, nil
	}
	timeSpent := strings.TrimSpace(m.wlInputs[0].Value())
	startRaw := strings.TrimSpace(m.wlInputs[1].Value())

	if timeSpent == "" {
		m.status = "Time is required"
		m.isErr = true
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
			m.status = `Invalid start — use HH:MM or "YYYY-MM-DD HH:MM"`
			m.isErr = true
			return m, nil
		}
	} else {
		startTime = time.Now().Format("2006-01-02T15:04:05.000-0700")
	}

	m.mode = boardModeNormal
	return m, m.addWorklogCmd(issue.Key, timeSpent, startTime)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (m *boardModel) rebuildColumns() {
	// Preserve current selection (issue key) so refresh/toggle stays focused
	var selectedKey string
	if issue := m.currentIssue(); issue != nil {
		selectedKey = issue.Key
	}

	statusOrder := []string{"To Do", "Open", "Backlog", "In Progress", "In Review", "Ready for QA", "In QA", "Done", "Closed"}
	statusIdx := map[string]int{}
	for i, s := range statusOrder {
		statusIdx[strings.ToLower(s)] = i
	}

	filtered := m.allIssues
	if m.mineOnly {
		filtered = nil
		for _, is := range m.allIssues {
			if is.Fields.Assignee != nil && is.Fields.Assignee.AccountID == m.cbs.CurrentUserID {
				filtered = append(filtered, is)
			}
		}
	}

	grouped := map[string][]jira.Issue{}
	var order []string
	seen := map[string]bool{}
	for _, is := range filtered {
		s := is.Fields.Status.Name
		if !seen[s] {
			seen[s] = true
			order = append(order, s)
		}
		grouped[s] = append(grouped[s], is)
	}

	sort.SliceStable(order, func(i, j int) bool {
		ai, aok := statusIdx[strings.ToLower(order[i])]
		bi, bok := statusIdx[strings.ToLower(order[j])]
		if !aok {
			ai = len(statusOrder)
		}
		if !bok {
			bi = len(statusOrder)
		}
		if ai != bi {
			return ai < bi
		}
		return order[i] < order[j]
	})

	cols := make([]boardColumn, 0, len(order))
	for _, s := range order {
		cards := grouped[s]
		sort.SliceStable(cards, func(i, j int) bool { return cards[i].Key < cards[j].Key })
		cols = append(cols, boardColumn{status: s, cards: cards})
	}
	m.columns = cols
	m.colOffsets = make([]int, len(cols))

	// Restore selection if possible
	if selectedKey != "" {
		for ci, c := range cols {
			for ri, card := range c.cards {
				if card.Key == selectedKey {
					m.cursorCol = ci
					m.cursorRow = ri
					return
				}
			}
		}
	}
}

func (m *boardModel) clampCursor() {
	if len(m.columns) == 0 {
		m.cursorCol = 0
		m.cursorRow = 0
		return
	}
	if m.cursorCol >= len(m.columns) {
		m.cursorCol = len(m.columns) - 1
	}
	if m.cursorCol < 0 {
		m.cursorCol = 0
	}
	m.clampRow()
}

func (m *boardModel) clampRow() {
	col := m.currentColumn()
	if col == nil || len(col.cards) == 0 {
		m.cursorRow = 0
		return
	}
	if m.cursorRow >= len(col.cards) {
		m.cursorRow = len(col.cards) - 1
	}
	if m.cursorRow < 0 {
		m.cursorRow = 0
	}
}

// rowsPerCard is the vertical space a rendered card + gap consumes.
// Matches the current rounded-border + 3-line body layout: 5 card rows + 1 gap.
const rowsPerCard = 6

// cardsPerView computes how many cards fit vertically given terminal height
// minus title/status/hint/column-header chrome.
func (m boardModel) cardsPerView() int {
	const chromeRows = 9 // blank + title + blank + col header + blank + blank + status + hint + margin
	h := m.height - chromeRows
	if h < rowsPerCard {
		return 1
	}
	return h / rowsPerCard
}

// ensureCursorVisible scrolls the current column so cursorRow is on screen.
func (m *boardModel) ensureCursorVisible() {
	if m.cursorCol < 0 || m.cursorCol >= len(m.colOffsets) {
		return
	}
	per := m.cardsPerView()
	off := m.colOffsets[m.cursorCol]
	if m.cursorRow < off {
		off = m.cursorRow
	}
	if m.cursorRow >= off+per {
		off = m.cursorRow - per + 1
	}
	if off < 0 {
		off = 0
	}
	m.colOffsets[m.cursorCol] = off
}

func (m boardModel) currentColumn() *boardColumn {
	if m.cursorCol < 0 || m.cursorCol >= len(m.columns) {
		return nil
	}
	return &m.columns[m.cursorCol]
}

func (m boardModel) currentIssue() *jira.Issue {
	col := m.currentColumn()
	if col == nil || m.cursorRow < 0 || m.cursorRow >= len(col.cards) {
		return nil
	}
	return &col.cards[m.cursorRow]
}

func (m boardModel) buildMoveOptions(transitions []jira.Transition) []boardMoveOption {
	issue := m.currentIssue()
	if issue == nil {
		return nil
	}

	columns := m.cbs.BoardColumns
	if len(columns) == 0 {
		columns = make([]jira.BoardColumn, 0, len(m.columns))
		for _, col := range m.columns {
			columns = append(columns, jira.BoardColumn{
				Name: col.status,
				Statuses: []jira.BoardColumnStatus{
					{Name: col.status},
				},
			})
		}
	}

	matches := jira.MatchTransitionsToBoardColumns(columns, transitions, issue.Fields.Status.Name)
	options := make([]boardMoveOption, 0, len(matches))
	for _, match := range matches {
		options = append(options, boardMoveOption{
			label:      match.Column.Name,
			transition: match.Transition,
		})
	}

	return options
}

func initials(name string) string {
	if name == "" {
		return "--"
	}
	parts := strings.Fields(name)
	if len(parts) == 1 {
		if len(parts[0]) >= 2 {
			return strings.ToUpper(parts[0][:2])
		}
		return strings.ToUpper(parts[0])
	}
	return strings.ToUpper(string(parts[0][0]) + string(parts[len(parts)-1][0]))
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m boardModel) View() string {
	if !m.ready {
		return "Loading..."
	}

	var sb strings.Builder

	// Title bar
	titleLine := boardTitleStyle.Render(m.title)
	if m.mineOnly {
		titleLine += "  " + boardHintStyle.Render("[mine only]")
	}
	sb.WriteString("\n")
	sb.WriteString(titleLine)
	sb.WriteString("\n\n")

	// Columns
	sb.WriteString(m.viewColumns())

	// Status line
	sb.WriteString("\n")
	if m.status != "" {
		if m.isErr {
			sb.WriteString(boardStatusErr.Render(m.status))
		} else {
			sb.WriteString(boardStatusOK.Render(m.status))
		}
		sb.WriteString("\n")
	}

	// Hints
	sb.WriteString(boardHintStyle.Render("h/l cols • j/k rows • pgup/pgdn page • b branch • m move • w worklog • o open • a mine • r refresh • ? help • q quit"))
	sb.WriteString("\n")

	base := sb.String()

	// Overlays
	switch m.mode {
	case boardModeMove:
		return base + "\n" + m.viewMoveOverlay()
	case boardModeWorklog:
		return base + "\n" + m.viewWorklogOverlay()
	case boardModeHelp:
		return base + "\n" + m.viewHelpOverlay()
	}
	return base
}

func (m boardModel) viewColumns() string {
	if len(m.columns) == 0 {
		return boardHintStyle.Render("  No issues.")
	}

	// Compute column width based on available terminal width
	totalCols := len(m.columns)
	width := m.width
	if width <= 0 {
		width = 120
	}
	// Reserve 2 chars padding between columns
	colW := width / totalCols
	if colW < 24 {
		colW = 24
	}
	if colW > 36 {
		colW = 36
	}
	cardInnerW := colW - 4 // account for borders + padding

	per := m.cardsPerView()

	rendered := make([]string, totalCols)
	for ci, col := range m.columns {
		var colSb strings.Builder
		header := boardColHeader.Render(col.status) + " " + boardColCount.Render(fmt.Sprintf("(%d)", len(col.cards)))
		colSb.WriteString(header)
		colSb.WriteString("\n\n")

		offset := 0
		if ci < len(m.colOffsets) {
			offset = m.colOffsets[ci]
		}
		end := offset + per
		if end > len(col.cards) {
			end = len(col.cards)
		}

		if offset > 0 {
			colSb.WriteString(boardHintStyle.Render(fmt.Sprintf("  ↑ %d more", offset)))
			colSb.WriteString("\n")
		}

		for ri := offset; ri < end; ri++ {
			card := col.cards[ri]
			isSelected := ci == m.cursorCol && ri == m.cursorRow

			assignee := "Unassigned"
			isMine := false
			if card.Fields.Assignee != nil {
				assignee = card.Fields.Assignee.DisplayName
				if card.Fields.Assignee.AccountID == m.cbs.CurrentUserID {
					isMine = true
				}
			}

			summary := truncate(card.Fields.Summary, cardInnerW)
			keyLine := boardKeyStyle.Render(card.Key)
			assigneeLine := boardAssigneeStyle.Render("@" + initials(assignee))

			cardBody := fmt.Sprintf("%s\n%s\n%s", keyLine, summary, assigneeLine)

			style := boardCardStyle
			if isSelected {
				style = boardCardSelected
			} else if isMine {
				style = boardCardMineStyle
			}
			colSb.WriteString(style.Width(colW - 2).Render(cardBody))
			colSb.WriteString("\n")
		}

		if end < len(col.cards) {
			colSb.WriteString(boardHintStyle.Render(fmt.Sprintf("  ↓ %d more", len(col.cards)-end)))
			colSb.WriteString("\n")
		}

		rendered[ci] = lipgloss.NewStyle().Width(colW).Render(colSb.String())
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
}

func (m boardModel) viewMoveOverlay() string {
	issue := m.currentIssue()
	if issue == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(boardTitleStyle.Render(fmt.Sprintf("Move %s", issue.Key)))
	sb.WriteString("\n\n")
	if len(m.moveOptions) == 0 {
		sb.WriteString(boardHintStyle.Render("Loading..."))
	} else {
		for i, option := range m.moveOptions {
			if i == m.moveCursor {
				sb.WriteString(selectCursorStyle.Render("▶ " + option.label))
			} else {
				sb.WriteString(selectItemStyle.Render("  " + option.label))
			}
			sb.WriteString("\n")
		}
	}
	sb.WriteString("\n")
	sb.WriteString(boardHintStyle.Render("↑/↓ select • enter confirm • esc cancel"))
	return boardOverlayStyle.Render(sb.String())
}

func (m boardModel) viewWorklogOverlay() string {
	issue := m.currentIssue()
	if issue == nil {
		return ""
	}
	labels := []string{"Time Spent", "Start Time"}
	hints := []string{"e.g. 1h, 30m, 1h30m", "e.g. 09:30 (leave empty = now)"}

	var sb strings.Builder
	sb.WriteString(boardTitleStyle.Render(fmt.Sprintf("Log work — %s", issue.Key)))
	sb.WriteString("\n\n")
	for i, input := range m.wlInputs {
		label := formLabelStyle.Render(labels[i] + ":")
		field := input.View()
		hint := lipgloss.NewStyle().Faint(true).Render("  " + hints[i])
		if i == m.wlFocused {
			sb.WriteString(formActiveStyle.Render("▶ ") + label + field + "\n" + hint + "\n\n")
		} else {
			sb.WriteString("  " + label + field + "\n\n")
		}
	}
	sb.WriteString(boardHintStyle.Render("tab/↑↓ navigate • enter next/submit • esc cancel"))
	return boardOverlayStyle.Render(sb.String())
}

func (m boardModel) viewHelpOverlay() string {
	help := []string{
		"Navigation:",
		"  h / ←        move to column on the left",
		"  l / →        move to column on the right",
		"  j / ↓        next card in column",
		"  k / ↑        previous card in column",
		"  pgdn/ctrl+d  page down within column",
		"  pgup/ctrl+u  page up within column",
		"  g / G        jump to first / last card in column",
		"",
		"Actions:",
		"  b            create branch (Bug: fix/KEY, others: feature/KEY)",
		"  m            move (transition) selected issue",
		"  w            log work on selected issue",
		"  o            open selected issue in browser",
		"  a            toggle mine-only filter",
		"  r            refresh board",
		"",
		"Misc:",
		"  ?            this help",
		"  q / esc      quit (or close overlay)",
	}
	body := boardTitleStyle.Render("Keybindings") + "\n\n" + strings.Join(help, "\n")
	return boardOverlayStyle.Render(body)
}

// ── Entry point ───────────────────────────────────────────────────────────────

// RunBoard launches the Kanban/Sprint TUI.
func RunBoard(title string, initialIssues []jira.Issue, cbs BoardCallbacks) error {
	m := boardModel{
		title:     title,
		cbs:       cbs,
		allIssues: initialIssues,
	}
	m.rebuildColumns()
	m.clampCursor()

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
