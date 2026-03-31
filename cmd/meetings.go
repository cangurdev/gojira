package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"gojira/internal/config"
	"gojira/internal/jira"
)

var meetingsStartTime string
var meetingsDay int
var meetingsMonth int

var meetingsCmd = &cobra.Command{
	Use:   "m [board_key] [type] [time_spent]",
	Short: "Quick worklog from templates",
	Long: `Log work quickly using templates from templates.yaml.

Example:
  gojira m proj d                         		# Use time_spent and start_time from yaml
  gojira m proj d 5m                      		# Override time with 5m, use start_time from yaml
  gojira m proj r 1h                    		# Override time with 1h, use start_time from yaml
  gojira m proj d --start 10:45           		# Override start time (today's date)
  gojira m proj d --start "2026-02-03 10:45"  	# Override start time with full date
  gojira m proj d --day 15                		# Use day 15 of current month, hour from template
  gojira m proj d --day 15 --start 10:45  		# Use day 15 of current month at 10:45
  gojira m proj d --month 2 --day 3       		# Use February 3 of current year, hour from template
  gojira m proj d --month 2 --day 3 --start 10:45  # Use February 3 at 10:45

Arguments:
  board_key   - Board key from templates.yaml (e.g., proj, atom)
  type        - Meeting type (e.g., d=daily, r=refinement, sr=sprint review)
  time_spent  - (Optional) Time to log (e.g., 5m, 1h, 1h 30m). If not provided, uses yaml value

Flags:
  -s, --start     - Override start time (e.g., 10:45 or "2026-02-03 10:45")
  -d, --day       - Override day of month (e.g., 15); uses template start_time for hour
  -m, --month     - Override month (e.g., 2 for February); uses current year`,
	Args: cobra.RangeArgs(2, 3),
	RunE: runMeetingsCommand,
}

func init() {
	meetingsCmd.Flags().StringVarP(&meetingsStartTime, "start", "s", "", "Override start time (e.g., 10:45 or \"2026-02-03 10:45\")")
	meetingsCmd.Flags().IntVarP(&meetingsDay, "day", "d", 0, "Override day of month (e.g., 15)")
	meetingsCmd.Flags().IntVarP(&meetingsMonth, "month", "m", 0, "Override month of current year (e.g., 2 for February)")
}

func runMeetingsCommand(cmd *cobra.Command, args []string) error {
	boardKey := strings.ToUpper(args[0])
	meetingType := strings.ToLower(args[1])

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	// Load templates
	templates, err := config.LoadTemplates()
	if err != nil {
		return fmt.Errorf("failed to load templates: %w", err)
	}

	// Find matching template
	var matchedTemplate *jira.Template
	for i := range templates {
		t := &templates[i]
		if strings.ToUpper(t.BoardKey) == boardKey && strings.ToLower(t.Type) == meetingType {
			matchedTemplate = t
			break
		}
	}

	if matchedTemplate == nil {
		return fmt.Errorf("no template found for board_key=%s and type=%s", boardKey, meetingType)
	}

	// Determine time_spent: use arg if provided, otherwise use template value
	var timeSpent string
	if len(args) >= 3 {
		timeSpent = args[2]
	} else {
		timeSpent = matchedTemplate.TimeSpent
		if timeSpent == "" {
			return fmt.Errorf("no time_spent provided and template has no default time_spent")
		}
	}

	// Create Jira client
	client := jira.NewClient(cfg.JiraURL, cfg.JiraEmail, cfg.JiraAPIToken)

	// Determine start time
	now := time.Now()
	var startTime string

	// resolve effective month: --month flag or current month
	effectiveMonth := now.Month()
	if meetingsMonth != 0 {
		effectiveMonth = time.Month(meetingsMonth)
	}

	// --start with full date takes full priority; --day/--month are ignored in that case
	if meetingsStartTime != "" {
		if t, err := time.ParseInLocation("2006-01-02 15:04", meetingsStartTime, time.Local); err == nil {
			startTime = t.Format("2006-01-02T15:04:05.000-0700")
		} else if t, err := time.ParseInLocation("15:04", meetingsStartTime, time.Local); err == nil {
			day := now.Day()
			if meetingsDay != 0 {
				day = meetingsDay
			}
			fullTime := time.Date(now.Year(), effectiveMonth, day, t.Hour(), t.Minute(), 0, 0, time.Local)
			startTime = fullTime.Format("2006-01-02T15:04:05.000-0700")
		} else {
			return fmt.Errorf("invalid --start format %q: use HH:MM or \"YYYY-MM-DD HH:MM\"", meetingsStartTime)
		}
	} else if meetingsDay != 0 || meetingsMonth != 0 {
		// --day or --month provided: use current year, effective month+day, template start_time for hour
		day := now.Day()
		if meetingsDay != 0 {
			day = meetingsDay
		}
		hour, minute := now.Hour(), now.Minute()
		if matchedTemplate.StartTime != "" {
			if t, err := time.ParseInLocation("15:04", matchedTemplate.StartTime, time.Local); err == nil {
				hour, minute = t.Hour(), t.Minute()
			}
		}
		fullTime := time.Date(now.Year(), effectiveMonth, day, hour, minute, 0, 0, time.Local)
		startTime = fullTime.Format("2006-01-02T15:04:05.000-0700")
	} else if matchedTemplate.StartTime != "" {
		if t, err := time.ParseInLocation("15:04", matchedTemplate.StartTime, time.Local); err == nil {
			fullTime := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, time.Local)
			startTime = fullTime.Format("2006-01-02T15:04:05.000-0700")
		} else {
			startTime = now.Format("2006-01-02T15:04:05.000-0700")
		}
	} else {
		startTime = now.Format("2006-01-02T15:04:05.000-0700")
	}

	// Add worklog
	response, err := client.AddWorklogWithStartTime(
		matchedTemplate.IssueKey,
		timeSpent,
		startTime,
		matchedTemplate.Description,
	)
	if err != nil {
		return fmt.Errorf("failed to log work: %w", err)
	}

	// Display success message
	fmt.Printf("✓ Logged %s to %s (%s)\n", response.TimeSpent, matchedTemplate.IssueKey, matchedTemplate.Name)
	fmt.Printf("  Description: %s\n", matchedTemplate.Description)
	fmt.Printf("  Worklog ID: %s\n", response.ID)

	return nil
}
