package cmd

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"gojira/internal/config"
	"gojira/internal/git"
	"gojira/internal/jira"
	"gojira/internal/ui"
)

var boardCmd = &cobra.Command{
	Use:   "board [project]",
	Short: "Interactive Kanban/Sprint board TUI",
	Long: `Show a full-screen Kanban board of the active sprint (scrum) or the whole board (kanban).
Issues are grouped by status into columns. Navigate with h/l (columns) and j/k (rows).

Actions:
  b   create branch for selected issue
  m   move (transition) selected issue
  w   log work on selected issue
  o   open selected issue in browser
  a   toggle mine-only filter
  r   refresh
  ?   help
  q   quit`,
	Args: cobra.MaximumNArgs(1),
	RunE: runBoardCommand,
}

func runBoardCommand(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	client := jira.NewClient(cfg.JiraURL, cfg.JiraEmail, cfg.JiraAPIToken)

	// Fetch boards
	var boards []jira.Board
	for _, id := range cfg.BoardIDs {
		b, err := client.GetBoard(id)
		if err != nil {
			return fmt.Errorf("failed to fetch board %d: %w", id, err)
		}
		boards = append(boards, *b)
	}

	// Select board
	var selected *jira.Board
	if len(args) == 1 {
		projectKey := strings.ToUpper(args[0])
		for i, b := range boards {
			if strings.ToUpper(b.Location.ProjectKey) == projectKey || strings.ToUpper(b.Name) == projectKey {
				selected = &boards[i]
				break
			}
		}
		if selected == nil {
			return fmt.Errorf("no board found for project %q", args[0])
		}
	} else {
		selected, err = ui.SelectBoard(boards)
		if err != nil {
			return fmt.Errorf("board selection error: %w", err)
		}
	}

	currentUser, err := client.GetCurrentUser()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	// Fetch issues based on board type
	isKanban := strings.ToLower(selected.Type) == "kanban"
	title := selected.Name

	fetchIssues := func() ([]jira.Issue, error) {
		if isKanban {
			return client.GetBoardIssues(selected.ID)
		}
		sprint, err := client.GetActiveSprint(selected.ID)
		if err != nil {
			return nil, err
		}
		return client.GetSprintIssues(sprint.ID)
	}

	// Initial fetch + title enrichment with sprint name
	initialIssues, err := fetchIssues()
	if err != nil {
		return fmt.Errorf("failed to fetch issues: %w", err)
	}
	if !isKanban {
		if sprint, err := client.GetActiveSprint(selected.ID); err == nil {
			title = fmt.Sprintf("%s — %s", selected.Name, sprint.Name)
		}
	}

	cbs := ui.BoardCallbacks{
		FetchIssues:      fetchIssues,
		FetchTransitions: client.GetTransitions,
		DoTransition:     client.DoTransition,
		AddWorklog: func(issueKey, timeSpent, startTime, description string) (*jira.WorklogResponse, error) {
			return client.AddWorklogWithStartTime(issueKey, normalizeTimeSpent(timeSpent), startTime, description)
		},
		CreateBranch: func(issue jira.Issue) (string, error) {
			prefix := "feature"
			if strings.EqualFold(issue.Fields.IssueType.Name, "bug") {
				prefix = "fix"
			}

			branchName := fmt.Sprintf("%s/%s", prefix, strings.ToUpper(issue.Key))
			if err := git.CreateAndCheckoutBranch(branchName); err != nil {
				return "", err
			}

			return branchName, nil
		},
		OpenInBrowser: func(issueKey string) {
			url := fmt.Sprintf("%s/browse/%s", strings.TrimRight(cfg.JiraURL, "/"), issueKey)
			_ = openURL(url)
		},
		CurrentUserID: currentUser.AccountID,
	}

	return ui.RunBoard(title, initialIssues, cbs)
}

func openURL(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default: // linux, freebsd, etc.
		cmd = "xdg-open"
		args = []string{url}
	}
	return exec.Command(cmd, args...).Start()
}
