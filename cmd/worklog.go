package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"gojira/internal/config"
	"gojira/internal/jira"
	"gojira/internal/ui"
)

var worklogDateFlag string

var worklogCmd = &cobra.Command{
	Use:   "worklog",
	Short: "Interactive worklog editor",
	Long: `Show your worklogs in an interactive table and edit or delete them.

By default shows this week. Use -d to pick a specific day or range.

Keys:
  ↑/↓   navigate
  e     edit selected worklog (time, start, description)
  d     delete selected worklog (with confirmation)
  r     refresh
  q     quit`,
	Aliases: []string{"wl"},
	RunE:    runWorklogCommand,
}

func init() {
	worklogCmd.Flags().StringVarP(&worklogDateFlag, "date", "d", "", "Date or date range (YYYY-MM-DD or YYYY-MM-DD:YYYY-MM-DD)")
}

func runWorklogCommand(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	client := jira.NewClient(cfg.JiraURL, cfg.JiraEmail, cfg.JiraAPIToken)

	var title string
	var fetcher func() ([]jira.WorklogWithIssue, error)

	if worklogDateFlag != "" {
		from, to, t, err := parseDateFlag(worklogDateFlag)
		if err != nil {
			return err
		}
		title = t
		fetcher = func() ([]jira.WorklogWithIssue, error) {
			return client.GetUserWorklogsBetween(from, to)
		}
	} else {
		now := time.Now()
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		monday := now.AddDate(0, 0, -(weekday - 1))
		title = fmt.Sprintf("Worklogs — %s to %s", monday.Format("Jan 2"), now.Format("Jan 2"))
		fetcher = func() ([]jira.WorklogWithIssue, error) {
			return client.GetUserWorklogsForWeek()
		}
	}

	worklogs, err := fetcher()
	if err != nil {
		return fmt.Errorf("failed to fetch worklogs: %w", err)
	}

	if len(worklogs) == 0 {
		fmt.Println("No worklogs found.")
		return nil
	}

	cbs := ui.WorklogCallbacks{
		Refresh: fetcher,
		Update: func(issueKey, worklogID, timeSpent, started, description string) (*jira.WorklogResponse, error) {
			return client.UpdateWorklog(issueKey, worklogID, normalizeTimeSpent(timeSpent), started, description)
		},
		Delete: client.DeleteWorklog,
	}

	return ui.RunWorklogEditor(title, worklogs, cbs)
}
