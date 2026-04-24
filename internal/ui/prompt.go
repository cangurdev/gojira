package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"gojira/internal/jira"
)

// SelectBoard prompts the user to select a board from a list using arrow keys
func SelectBoard(boards []jira.Board) (*jira.Board, error) {
	if len(boards) == 1 {
		return &boards[0], nil
	}

	items := make([]string, len(boards))
	for i, board := range boards {
		items[i] = fmt.Sprintf("%s (ID: %d)", board.Name, board.ID)
	}

	idx, err := runSelect("Select a board", items)
	if err != nil {
		return nil, fmt.Errorf("board selection cancelled: %w", err)
	}
	if idx < 0 {
		return nil, fmt.Errorf("board selection cancelled")
	}
	return &boards[idx], nil
}

// SelectBoardColumn prompts the user to select a board column from a list.
func SelectBoardColumn(columns []jira.BoardColumn) (*jira.BoardColumn, error) {
	if len(columns) == 0 {
		return nil, fmt.Errorf("no board columns available")
	}

	items := make([]string, len(columns))
	for i, column := range columns {
		statusNames := make([]string, 0, len(column.Statuses))
		for _, status := range column.Statuses {
			if status.Name != "" {
				statusNames = append(statusNames, status.Name)
			}
		}

		items[i] = column.Name
		if len(statusNames) > 0 {
			items[i] = fmt.Sprintf("%s (%s)", column.Name, strings.Join(statusNames, ", "))
		}
	}

	idx, err := runSelect("Select a target column", items)
	if err != nil {
		return nil, fmt.Errorf("column selection cancelled: %w", err)
	}
	if idx < 0 {
		return nil, fmt.Errorf("column selection cancelled")
	}
	return &columns[idx], nil
}

// SelectTemplate prompts the user to select a template from a list
func SelectTemplate(templates []jira.Template) (*jira.Template, error) {
	if len(templates) == 0 {
		return nil, fmt.Errorf("no templates available")
	}

	items := make([]string, len(templates))
	for i, t := range templates {
		items[i] = fmt.Sprintf("%s (%s, %s)", t.Name, t.IssueKey, t.TimeSpent)
	}

	idx, err := runSelect("Select a template", items)
	if err != nil {
		return nil, fmt.Errorf("template selection cancelled: %w", err)
	}
	if idx < 0 {
		return nil, fmt.Errorf("template selection cancelled")
	}
	return &templates[idx], nil
}

// WorklogType represents the type of worklog entry
type WorklogType int

const (
	WorklogTypeIssue WorklogType = iota + 1
	WorklogTypeMeeting
	WorklogTypeCustom
)

// SelectWorklogType prompts the user to select between issue, meeting, and custom using arrow keys
func SelectWorklogType() (WorklogType, error) {
	items := []string{"Issue", "Meeting", "Custom"}

	idx, err := runSelect("Select worklog type", items)
	if err != nil {
		return 0, fmt.Errorf("worklog type selection cancelled: %w", err)
	}
	if idx < 0 {
		return 0, fmt.Errorf("worklog type selection cancelled")
	}
	return WorklogType(idx + 1), nil
}

// SelectMeeting prompts the user to select a meeting from a list using arrow keys
func SelectMeeting(meetings []jira.Meeting) (*jira.Meeting, error) {
	if len(meetings) == 0 {
		return nil, fmt.Errorf("no meetings available for this board")
	}

	items := make([]string, len(meetings))
	for i, m := range meetings {
		items[i] = fmt.Sprintf("%s (%s)", m.Name, m.IssueKey)
	}

	idx, err := runSelect("Select a meeting", items)
	if err != nil {
		return nil, fmt.Errorf("meeting selection cancelled: %w", err)
	}
	if idx < 0 {
		return nil, fmt.Errorf("meeting selection cancelled")
	}
	return &meetings[idx], nil
}

// PromptInput prompts the user for text input with a given prompt
func PromptInput(prompt string) (string, error) {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}
	return strings.TrimSpace(input), nil
}

// PromptInputWithDefault prompts the user for text input with a default value
func PromptInputWithDefault(prompt, defaultValue string) (string, error) {
	fmt.Printf("%s [%s]: ", prompt, defaultValue)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue, nil
	}
	return input, nil
}
