package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"gojira/internal/config"
	"gojira/internal/jira"
	"gojira/internal/ui"
)

var summaryToday bool

var summaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Show logged work summary",
	Long: `Show a summary of your logged work.

Examples:
  gojira summary           # This week's worklogs
  gojira summary -t        # Today's worklogs

Flags:
  -t, --today  - Show only today's worklogs`,
	RunE: runSummaryCommand,
}

func init() {
	summaryCmd.Flags().BoolVarP(&summaryToday, "today", "t", false, "Show only today's worklogs")
}

func runSummaryCommand(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	client := jira.NewClient(cfg.JiraURL, cfg.JiraEmail, cfg.JiraAPIToken)

	worklogs, err := client.GetUserWorklogsForWeek()
	if err != nil {
		return fmt.Errorf("failed to fetch worklogs: %w", err)
	}

	if summaryToday {
		today := time.Now()
		var todayWorklogs []jira.WorklogWithIssue
		for _, wl := range worklogs {
			started := wl.Worklog.Started.Time
			if started.Year() == today.Year() &&
				started.Month() == today.Month() &&
				started.Day() == today.Day() {
				todayWorklogs = append(todayWorklogs, wl)
			}
		}
		fmt.Printf("Today (%s)\n", today.Format("Mon Jan 2"))
		ui.PrintWorklogsTable(todayWorklogs)
	} else {
		now := time.Now()
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		monday := now.AddDate(0, 0, -(weekday - 1))
		fmt.Printf("This week (%s - %s)\n", monday.Format("Jan 2"), now.Format("Jan 2"))
		ui.PrintWorklogsTable(worklogs)
	}

	return nil
}
