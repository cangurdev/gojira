package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"gojira/internal/config"
	"gojira/internal/jira"
	"gojira/internal/ui"
)

var summaryToday bool
var summaryDate string

var summaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Show logged work summary",
	Long: `Show a summary of your logged work.

Examples:
  gojira summary                          # This week's worklogs
  gojira summary -t                       # Today's worklogs
  gojira summary -d 2026-04-15            # A specific day
  gojira summary -d 2026-04-01:2026-04-15 # A date range

Flags:
  -t, --today        Show only today's worklogs
  -d, --date STRING  Show worklogs for a specific date or range (YYYY-MM-DD or YYYY-MM-DD:YYYY-MM-DD)`,
	RunE: runSummaryCommand,
}

func init() {
	summaryCmd.Flags().BoolVarP(&summaryToday, "today", "t", false, "Show only today's worklogs")
	summaryCmd.Flags().StringVarP(&summaryDate, "date", "d", "", "Date or date range (YYYY-MM-DD or YYYY-MM-DD:YYYY-MM-DD)")
}

func parseDateFlag(dateStr string) (from, to time.Time, title string, err error) {
	const layout = "2006-01-02"
	if strings.Contains(dateStr, ":") {
		parts := strings.SplitN(dateStr, ":", 2)
		from, err = time.ParseInLocation(layout, parts[0], time.Local)
		if err != nil {
			return from, to, "", fmt.Errorf("invalid start date %q: use YYYY-MM-DD", parts[0])
		}
		to, err = time.ParseInLocation(layout, parts[1], time.Local)
		if err != nil {
			return from, to, "", fmt.Errorf("invalid end date %q: use YYYY-MM-DD", parts[1])
		}
		title = fmt.Sprintf("%s - %s", from.Format("Jan 2"), to.Format("Jan 2, 2006"))
	} else {
		from, err = time.ParseInLocation(layout, dateStr, time.Local)
		if err != nil {
			return from, to, "", fmt.Errorf("invalid date %q: use YYYY-MM-DD", dateStr)
		}
		to = from
		title = from.Format("Mon Jan 2, 2006")
	}
	return from, to, title, nil
}

func runSummaryCommand(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	client := jira.NewClient(cfg.JiraURL, cfg.JiraEmail, cfg.JiraAPIToken)

	logFn := func(issueKey, timeSpent, startTime string) (*jira.WorklogResponse, error) {
		return client.AddWorklogWithStartTime(issueKey, normalizeTimeSpent(timeSpent), startTime, "")
	}

	if summaryDate != "" {
		from, to, title, err := parseDateFlag(summaryDate)
		if err != nil {
			return err
		}
		worklogs, err := client.GetUserWorklogsBetween(from, to)
		if err != nil {
			return fmt.Errorf("failed to fetch worklogs: %w", err)
		}
		if err := ui.RunSummaryTable(worklogs, title, logFn); err != nil {
			return fmt.Errorf("summary display error: %w", err)
		}
		return nil
	}

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
		title := fmt.Sprintf("Today (%s)", today.Format("Mon Jan 2"))
		if err := ui.RunSummaryTable(todayWorklogs, title, logFn); err != nil {
			return fmt.Errorf("summary display error: %w", err)
		}
	} else {
		now := time.Now()
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		monday := now.AddDate(0, 0, -(weekday - 1))
		title := fmt.Sprintf("This week (%s - %s)", monday.Format("Jan 2"), now.Format("Jan 2"))
		if err := ui.RunSummaryTable(worklogs, title, logFn); err != nil {
			return fmt.Errorf("summary display error: %w", err)
		}
	}

	return nil
}
