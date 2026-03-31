package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"gojira/internal/config"
	"gojira/internal/jira"
	"gojira/internal/ui"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks from active sprint",
	Long:  `List all tasks assigned to the current user from the active sprint of a selected board.`,
	RunE:  runListCommand,
}

func runListCommand(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	// Create Jira client
	client := jira.NewClient(cfg.JiraURL, cfg.JiraEmail, cfg.JiraAPIToken)

	// Fetch board details for all configured boards
	var boards []jira.Board
	for _, boardID := range cfg.BoardIDs {
		board, err := client.GetBoard(boardID)
		if err != nil {
			return fmt.Errorf("failed to fetch board %d: %w", boardID, err)
		}
		boards = append(boards, *board)
	}

	// Select board (or use the only one)
	selectedBoard, err := ui.SelectBoard(boards)
	if err != nil {
		return fmt.Errorf("board selection error: %w", err)
	}

	fmt.Printf("\nFetching tasks from board: %s\n\n", selectedBoard.Name)

	// Get active sprint for the selected board
	sprint, err := client.GetActiveSprint(selectedBoard.ID)
	if err != nil {
		return fmt.Errorf("failed to get active sprint: %w", err)
	}

	fmt.Printf("Active sprint: %s\n\n", sprint.Name)

	// Get current user to filter by assignee
	currentUser, err := client.GetCurrentUser()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	// Get all issues from the sprint
	issues, err := client.GetSprintIssues(sprint.ID)
	if err != nil {
		return fmt.Errorf("failed to get sprint issues: %w", err)
	}

	// Filter issues assigned to the current user
	var myIssues []jira.Issue
	for _, issue := range issues {
		if issue.Fields.Assignee != nil && issue.Fields.Assignee.AccountID == currentUser.AccountID {
			myIssues = append(myIssues, issue)
		}
	}

	// Display issues in table format
	if len(myIssues) == 0 {
		fmt.Println("No issues assigned to you in this sprint.")
		return nil
	}

	fmt.Printf("Issues assigned to %s:\n\n", currentUser.DisplayName)
	ui.PrintIssuesTable(myIssues)

	return nil
}
