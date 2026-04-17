package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	selectTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	selectCursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	selectItemStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	selectSelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
)

type selectModel struct {
	label    string
	items    []string
	cursor   int
	choice   int // -1 = cancelled
	maxShow  int
	offset   int
}

func newSelectModel(label string, items []string) selectModel {
	maxShow := 10
	if len(items) < maxShow {
		maxShow = len(items)
	}
	return selectModel{
		label:   label,
		items:   items,
		cursor:  0,
		choice:  -1,
		maxShow: maxShow,
	}
}

func (m selectModel) Init() tea.Cmd { return nil }

func (m selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset--
				}
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
				if m.cursor >= m.offset+m.maxShow {
					m.offset++
				}
			}
		case "enter", " ":
			m.choice = m.cursor
			return m, tea.Quit
		case "ctrl+c", "esc", "q":
			m.choice = -1
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m selectModel) View() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(selectTitleStyle.Render(m.label))
	sb.WriteString("\n\n")

	end := m.offset + m.maxShow
	if end > len(m.items) {
		end = len(m.items)
	}

	for i := m.offset; i < end; i++ {
		if i == m.cursor {
			sb.WriteString(selectCursorStyle.Render("▶ " + m.items[i]))
		} else {
			sb.WriteString(selectItemStyle.Render("  " + m.items[i]))
		}
		sb.WriteString("\n")
	}

	if len(m.items) > m.maxShow {
		sb.WriteString("\n")
		sb.WriteString(timerHintStyle.Render(fmt.Sprintf("%d/%d", m.cursor+1, len(m.items))))
	}

	sb.WriteString("\n")
	sb.WriteString(timerHintStyle.Render("↑/↓ navigate • enter select • esc cancel"))
	sb.WriteString("\n")
	return sb.String()
}

// runSelect shows an interactive list and returns the selected index, or -1 if cancelled.
func runSelect(label string, items []string) (int, error) {
	m := newSelectModel(label, items)
	result, err := tea.NewProgram(m).Run()
	if err != nil {
		return -1, err
	}
	return result.(selectModel).choice, nil
}
