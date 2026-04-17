package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"gojira/internal/jira"
)

var (
	issueHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	issueBaseStyle   = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("8"))

	statusColors = map[string]string{
		"to do":       "8",
		"in progress": "3",
		"in review":   "4",
		"done":        "2",
	}
)

func colorStatus(s string) string {
	color, ok := statusColors[strings.ToLower(s)]
	if !ok {
		color = "7"
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(s)
}

type issuesModel struct {
	table  table.Model
	title  string
	issues []jira.Issue
}

func (m issuesModel) Init() tea.Cmd { return nil }

func (m issuesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m issuesModel) View() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(issueHeaderStyle.Render(m.title))
	sb.WriteString("\n\n")
	sb.WriteString(issueBaseStyle.Render(m.table.View()))
	sb.WriteString("\n\n")
	sb.WriteString(timerHintStyle.Render(fmt.Sprintf("%d issues  •  ↑/↓ scroll  •  q quit", len(m.issues))))
	sb.WriteString("\n")
	return sb.String()
}

// RunIssuesTable renders an interactive Bubble Tea table for the given issues.
func RunIssuesTable(issues []jira.Issue, title string) error {
	if len(issues) == 0 {
		fmt.Println("No issues found.")
		return nil
	}

	columns := []table.Column{
		{Title: "Key",     Width: 14},
		{Title: "Summary", Width: 52},
		{Title: "Status",  Width: 16},
	}

	rows := make([]table.Row, len(issues))
	for i, issue := range issues {
		rows[i] = table.Row{
			issue.Key,
			truncate(issue.Fields.Summary, 52),
			issue.Fields.Status.Name,
		}
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.Bold(true).Foreground(lipgloss.Color("6"))
	s.Selected = s.Selected.Foreground(lipgloss.Color("0")).Background(lipgloss.Color("6"))
	t.SetStyles(s)

	m := issuesModel{
		table:  t,
		title:  title,
		issues: issues,
	}

	_, err := tea.NewProgram(m).Run()
	return err
}
