package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"gojira/internal/config"
	"gojira/internal/git"
	"gojira/internal/jira"
)

var moveCmd = &cobra.Command{
	Use:   "move [issue-key] <status>",
	Short: "Transition an issue to a new status",
	Long: `Move a Jira issue to a new status using a partial name match.

If issue-key is omitted, it is inferred from the current git branch.

Examples:
  gojira move PROJ-123 progress    # → In Progress
  gojira move PROJ-123 review      # → In Review
  gojira move progress             # branch: feature/PROJ-123 → In Progress

The status argument is matched case-insensitively against available transitions.
If no match is found, available transitions are listed.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runMoveCommand,
}

func runMoveCommand(cmd *cobra.Command, args []string) error {
	var issueKey, statusQuery string

	if len(args) == 2 {
		issueKey = strings.ToUpper(args[0])
		statusQuery = args[1]
	} else {
		var err error
		issueKey, err = git.GetIssueKeyFromBranch()
		if err != nil {
			return fmt.Errorf("no issue key provided and %w", err)
		}
		statusQuery = args[0]
		fmt.Printf("Branch issue: %s\n", issueKey)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	client := jira.NewClient(cfg.JiraURL, cfg.JiraEmail, cfg.JiraAPIToken)

	matched, err := client.TransitionIssue(issueKey, statusQuery)
	if err != nil {
		return err
	}

	fmt.Printf("✓ %s → %s\n", issueKey, matched.Name)
	return nil
}
