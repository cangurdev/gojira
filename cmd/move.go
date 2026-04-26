package cmd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"gojira/internal/config"
	"gojira/internal/git"
	"gojira/internal/jira"
	"gojira/internal/ui"
)

var issueKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]+-\d+$`)

var moveCmd = &cobra.Command{
	Use:   "move [issue-key] [status]",
	Short: "Transition an issue to a new status",
	Long: `Move a Jira issue to a new status.

If status is omitted, the command loads the configured board columns and lets you
pick the target column interactively. If issue-key is omitted, it is inferred
from the current git branch.

Examples:
  gojira move PROJ-123             # choose a board column interactively
  gojira move PROJ-123 progress    # → In Progress
  gojira move PROJ-123 review      # → In Review
  gojira move                      # branch: feature/PROJ-123 → choose a board column
  gojira move progress             # branch: feature/PROJ-123 → In Progress

The status argument is matched case-insensitively against available transitions.
If no match is found, available transitions are listed. Interactive column
selection matches the chosen board column against the issue's available
transitions.`,
	Args: cobra.RangeArgs(0, 2),
	RunE: runMoveCommand,
}

func runMoveCommand(cmd *cobra.Command, args []string) error {
	var issueKey, statusQuery string

	switch len(args) {
	case 2:
		issueKey = strings.ToUpper(args[0])
		statusQuery = args[1]
	case 1:
		if looksLikeIssueKey(args[0]) {
			issueKey = strings.ToUpper(args[0])
		} else {
			var err error
			issueKey, err = git.GetIssueKeyFromBranch()
			if err != nil {
				return fmt.Errorf("no issue key provided and %w", err)
			}
			statusQuery = args[0]
			fmt.Printf("Branch issue: %s\n", issueKey)
		}
	default:
		var err error
		issueKey, err = git.GetIssueKeyFromBranch()
		if err != nil {
			return fmt.Errorf("no issue key provided and %w", err)
		}
		fmt.Printf("Branch issue: %s\n", issueKey)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	client := jira.NewClient(cfg.JiraURL, cfg.JiraEmail, cfg.JiraAPIToken)

	var matched *jira.Transition
	if statusQuery != "" {
		matched, err = client.TransitionIssue(issueKey, statusQuery)
	} else {
		matched, err = selectAndTransitionIssue(client, cfg, issueKey)
	}
	if err != nil {
		return err
	}

	fmt.Printf("✓ %s → %s\n", issueKey, matched.Name)
	return nil
}

func looksLikeIssueKey(value string) bool {
	return issueKeyPattern.MatchString(strings.ToUpper(strings.TrimSpace(value)))
}

func selectAndTransitionIssue(client *jira.Client, cfg *config.Config, issueKey string) (*jira.Transition, error) {
	boards, err := loadBoards(client, cfg.BoardIDs)
	if err != nil {
		return nil, err
	}

	board, err := selectBoardForIssue(issueKey, boards)
	if err != nil {
		return nil, err
	}

	boardConfig, err := client.GetBoardConfiguration(board.ID)
	if err != nil {
		return nil, err
	}

	transitions, err := client.GetTransitions(issueKey)
	if err != nil {
		return nil, err
	}

	matches := jira.MatchTransitionsToBoardColumns(boardConfig.ColumnConfig.Columns, transitions, "")
	if len(matches) == 0 {
		return nil, fmt.Errorf("no matching target columns available")
	}

	columns := make([]jira.BoardColumn, 0, len(matches))
	for _, match := range matches {
		columns = append(columns, match.Column)
	}

	column, err := ui.SelectBoardColumn(columns)
	if err != nil {
		return nil, err
	}

	for _, match := range matches {
		if match.Column.Name == column.Name {
			transition := match.Transition
			if err := client.DoTransition(issueKey, transition.ID); err != nil {
				return nil, err
			}
			return &transition, nil
		}
	}

	return nil, fmt.Errorf("no transition from %s matches selected board column %q", issueKey, column.Name)
}

func loadBoards(client *jira.Client, boardIDs []int) ([]jira.Board, error) {
	boards := make([]jira.Board, 0, len(boardIDs))
	for _, id := range boardIDs {
		board, err := client.GetBoard(id)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch board %d: %w", id, err)
		}
		boards = append(boards, *board)
	}
	return boards, nil
}

func selectBoardForIssue(issueKey string, boards []jira.Board) (*jira.Board, error) {
	projectKey := strings.SplitN(strings.ToUpper(issueKey), "-", 2)[0]

	var matching []jira.Board
	for _, board := range boards {
		if strings.EqualFold(board.Location.ProjectKey, projectKey) {
			matching = append(matching, board)
		}
	}

	switch len(matching) {
	case 0:
		return ui.SelectBoard(boards)
	case 1:
		return &matching[0], nil
	default:
		return ui.SelectBoard(matching)
	}
}
