package cmd

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"gojira/internal/config"
	"gojira/internal/git"
	"gojira/internal/jira"
)

var logStartTime string
var logIssueKey string

var logCmd = &cobra.Command{
	Use:   "l [time_spent]",
	Short: "Quick worklog from git branch",
	Long: `Log work to the issue from current git branch.
Start time is calculated as current time minus time_spent.

Example:
  gojira l 30m                          # Log 30m, started 30m ago
  gojira l 1h                           # Log 1h, started 1h ago
  gojira l 2h30m                        # Log 2h 30m, started 2h 30m ago
  gojira l 1h --start 10:45             # Log 1h, started at 10:45 today
  gojira l 1h --start "2026-02-03 10:45"  # Log 1h, started at given date/time
  gojira l 1h --issue PROJ-123          # Log to specific issue instead of git branch

Time format examples:
  - 1h (1 hour)
  - 30m (30 minutes)
  - 1h 30m or 1h30m (1 hour 30 minutes)
  - 2d (2 days)
  - 2d 4h (2 days 4 hours)

Flags:
  --start   - Override start time (e.g., 10:45 or "2026-02-03 10:45"); ignores time_spent for start calculation
  --issue   - Override issue key (e.g., PROJ-123); if not provided, uses git branch`,
	Args: cobra.ExactArgs(1),
	RunE: runLogCommand,
}

func init() {
	logCmd.Flags().StringVar(&logStartTime, "start", "", "Override start time (e.g., 10:45 or \"2026-02-03 10:45\")")
	logCmd.Flags().StringVar(&logIssueKey, "issue", "", "Override issue key (e.g., PROJ-123)")
}

func runLogCommand(cmd *cobra.Command, args []string) error {
	timeSpent := args[0]

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	// Determine issue key: --issue flag takes priority, otherwise use git branch
	var issueKey string
	if logIssueKey != "" {
		issueKey = logIssueKey
	} else {
		issueKey, err = git.GetIssueKeyFromBranch()
		if err != nil {
			return fmt.Errorf("failed to get issue key from git branch: %w", err)
		}
	}

	// Validate issue key format
	if !isValidIssueKey(issueKey) {
		return fmt.Errorf("invalid issue key format: %s (expected format: PROJ-123)", issueKey)
	}

	// Determine start time
	now := time.Now()
	var startTime string
	var displayStart time.Time

	if logStartTime != "" {
		if t, err := time.ParseInLocation("2006-01-02 15:04", logStartTime, time.Local); err == nil {
			startTime = t.Format("2006-01-02T15:04:05.000-0700")
			displayStart = t
		} else if t, err := time.ParseInLocation("15:04", logStartTime, time.Local); err == nil {
			fullTime := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, time.Local)
			startTime = fullTime.Format("2006-01-02T15:04:05.000-0700")
			displayStart = fullTime
		} else {
			return fmt.Errorf("invalid --start format %q: use HH:MM or \"YYYY-MM-DD HH:MM\"", logStartTime)
		}
	} else {
		// Calculate start time as current time minus time spent
		duration, err := parseJiraDuration(timeSpent)
		if err != nil {
			return fmt.Errorf("invalid time format: %w", err)
		}
		displayStart = now.Add(-duration)
		startTime = displayStart.Format("2006-01-02T15:04:05.000-0700")
	}

	// Create Jira client
	client := jira.NewClient(cfg.JiraURL, cfg.JiraEmail, cfg.JiraAPIToken)

	// Add worklog with empty description
	response, err := client.AddWorklogWithStartTime(issueKey, timeSpent, startTime, "")
	if err != nil {
		return fmt.Errorf("failed to log work: %w", err)
	}

	// Display success message
	fmt.Printf("✓ Logged %s to %s\n", response.TimeSpent, issueKey)
	fmt.Printf("  Started: %s\n", displayStart.Format("15:04"))
	fmt.Printf("  Worklog ID: %s\n", response.ID)

	return nil
}

// isValidIssueKey checks if the issue key matches the expected format
func isValidIssueKey(key string) bool {
	// Match pattern like PROJ-123 (one or more uppercase letters, dash, one or more digits)
	pattern := `^[A-Z]+-[0-9]+$`
	matched, _ := regexp.MatchString(pattern, key)
	return matched
}

// parseJiraDuration parses Jira time format (e.g., "30m", "1h", "1h 30m", "2d 4h") to time.Duration
func parseJiraDuration(s string) (time.Duration, error) {
	var total time.Duration

	// Match patterns like: 1w, 2d, 3h, 4m
	pattern := regexp.MustCompile(`(\d+)([wdhm])`)
	matches := pattern.FindAllStringSubmatch(s, -1)

	if len(matches) == 0 {
		return 0, fmt.Errorf("invalid time format: %s (expected format: 30m, 1h, 1h 30m, 2d, etc.)", s)
	}

	for _, match := range matches {
		value, err := strconv.Atoi(match[1])
		if err != nil {
			return 0, fmt.Errorf("invalid number: %s", match[1])
		}

		unit := match[2]
		switch unit {
		case "w":
			// 1 week = 5 work days
			total += time.Duration(value) * 5 * 8 * time.Hour
		case "d":
			// 1 day = 8 work hours
			total += time.Duration(value) * 8 * time.Hour
		case "h":
			total += time.Duration(value) * time.Hour
		case "m":
			total += time.Duration(value) * time.Minute
		default:
			return 0, fmt.Errorf("unknown time unit: %s", unit)
		}
	}

	return total, nil
}
