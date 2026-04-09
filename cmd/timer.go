package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"gojira/internal/config"
	"gojira/internal/git"
	"gojira/internal/jira"
)

type timerState struct {
	IssueKey  string    `json:"issue"`
	StartedAt time.Time `json:"started_at"`
}

func timerFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".gojira")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "timer.json"), nil
}

func loadTimer() (*timerState, error) {
	path, err := timerFilePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var state timerState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func saveTimer(state *timerState) error {
	path, err := timerFilePath()
	if err != nil {
		return err
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func deleteTimer() error {
	path, err := timerFilePath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func formatElapsed(d time.Duration) string {
	d = d.Round(time.Minute)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 && m > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	} else if h > 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dm", m)
}

func durationToJira(d time.Duration) string {
	d = d.Round(time.Minute)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 && m > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	} else if h > 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dm", m)
}

var timerCmd = &cobra.Command{
	Use:   "timer",
	Short: "Track time spent on an issue",
}

var timerStartCmd = &cobra.Command{
	Use:   "start [issue-key]",
	Short: "Start timer for an issue (uses git branch if no issue key given)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		existing, err := loadTimer()
		if err != nil {
			return err
		}
		if existing != nil {
			elapsed := time.Since(existing.StartedAt)
			return fmt.Errorf("timer already running for %s (%s elapsed) — run 'gojira timer stop' first", existing.IssueKey, formatElapsed(elapsed))
		}

		var issueKey string
		if len(args) == 1 {
			issueKey = args[0]
		} else {
			issueKey, err = git.GetIssueKeyFromBranch()
			if err != nil {
				return fmt.Errorf("failed to get issue key from git branch: %w", err)
			}
		}

		if !isValidIssueKey(issueKey) {
			return fmt.Errorf("invalid issue key format: %s (expected format: PROJ-123)", issueKey)
		}

		state := &timerState{
			IssueKey:  issueKey,
			StartedAt: time.Now(),
		}
		if err := saveTimer(state); err != nil {
			return err
		}

		fmt.Printf("Timer started for %s at %s\n", issueKey, state.StartedAt.Format("15:04"))
		return nil
	},
}

var timerStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current timer status",
	RunE: func(cmd *cobra.Command, args []string) error {
		state, err := loadTimer()
		if err != nil {
			return err
		}
		if state == nil {
			fmt.Println("No active timer.")
			return nil
		}
		elapsed := time.Since(state.StartedAt)
		fmt.Printf("Timer running for %s — %s elapsed (started at %s)\n",
			state.IssueKey, formatElapsed(elapsed), state.StartedAt.Format("15:04"))
		return nil
	},
}

var timerStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop timer and log the time to Jira",
	RunE: func(cmd *cobra.Command, args []string) error {
		state, err := loadTimer()
		if err != nil {
			return err
		}
		if state == nil {
			return fmt.Errorf("no active timer running")
		}

		elapsed := time.Since(state.StartedAt)
		if elapsed < time.Minute {
			if err := deleteTimer(); err != nil {
				return err
			}
			fmt.Println("Timer stopped (less than 1 minute elapsed, nothing logged).")
			return nil
		}

		timeSpent := durationToJira(elapsed)
		startTime := state.StartedAt.Format("2006-01-02T15:04:05.000-0700")

		fmt.Printf("Issue:   %s\n", state.IssueKey)
		fmt.Printf("Started: %s\n", state.StartedAt.Format("15:04"))
		fmt.Printf("Elapsed: %s\n", timeSpent)
		fmt.Print("\nLog this time to Jira? [y/N] ")

		var answer string
		fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			if err := deleteTimer(); err != nil {
				return err
			}
			fmt.Println("Timer stopped, nothing logged.")
			return nil
		}

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("configuration error: %w", err)
		}

		client := jira.NewClient(cfg.JiraURL, cfg.JiraEmail, cfg.JiraAPIToken)
		response, err := client.AddWorklogWithStartTime(state.IssueKey, timeSpent, startTime, "")
		if err != nil {
			return fmt.Errorf("failed to log work: %w", err)
		}

		if err := deleteTimer(); err != nil {
			return err
		}

		fmt.Printf("✓ Logged %s to %s\n", response.TimeSpent, state.IssueKey)
		return nil
	},
}

func init() {
	timerCmd.AddCommand(timerStartCmd)
	timerCmd.AddCommand(timerStatusCmd)
	timerCmd.AddCommand(timerStopCmd)
}
