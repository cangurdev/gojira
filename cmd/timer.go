package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"gojira/internal/config"
	"gojira/internal/git"
	"gojira/internal/jira"
	"gojira/internal/ui"
)

type timerState struct {
	IssueKey        string    `json:"issue"`
	StartedAt       time.Time `json:"started_at"`                 // original first start; never changes
	ResumedAt       time.Time `json:"resumed_at,omitzero"`        // when the current run began; zero when paused
	AccumulatedMs   int64     `json:"accumulated_ms,omitempty"`   // completed run durations before current run
	Paused          bool      `json:"paused,omitempty"`
	IsMeeting       bool      `json:"is_meeting,omitempty"`
	MeetingBoardKey string    `json:"meeting_board_key,omitempty"`
	MeetingType     string    `json:"meeting_type,omitempty"`
}

func (s *timerState) elapsed() time.Duration {
	acc := time.Duration(s.AccumulatedMs) * time.Millisecond
	if s.Paused {
		return acc
	}
	runStart := s.ResumedAt
	if runStart.IsZero() {
		runStart = s.StartedAt
	}
	return acc + time.Since(runStart)
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
			status := "running"
			if existing.Paused {
				status = "paused"
			}
			return fmt.Errorf("timer already %s for %s (%s elapsed) — run 'gojira timer stop' first", status, existing.IssueKey, formatElapsed(existing.elapsed()))
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

		now := time.Now()
		state := &timerState{
			IssueKey:  issueKey,
			StartedAt: now,
			ResumedAt: now,
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
		runStart := state.ResumedAt
		if runStart.IsZero() {
			runStart = state.StartedAt
		}
		accumulated := time.Duration(state.AccumulatedMs) * time.Millisecond
		return ui.RunTimerStatus(state.IssueKey, state.StartedAt, runStart, accumulated, state.Paused)
	},
}

var timerPauseCmd = &cobra.Command{
	Use:   "pause",
	Short: "Pause the running timer",
	RunE: func(cmd *cobra.Command, args []string) error {
		state, err := loadTimer()
		if err != nil {
			return err
		}
		if state == nil {
			return fmt.Errorf("no active timer running")
		}
		if state.Paused {
			return fmt.Errorf("timer already paused for %s (%s elapsed)", state.IssueKey, formatElapsed(state.elapsed()))
		}

		runStart := state.ResumedAt
		if runStart.IsZero() {
			runStart = state.StartedAt
		}
		state.AccumulatedMs += time.Since(runStart).Milliseconds()
		state.ResumedAt = time.Time{}
		state.Paused = true

		if err := saveTimer(state); err != nil {
			return err
		}
		fmt.Printf("⏸  Timer paused for %s at %s (%s elapsed)\n", state.IssueKey, time.Now().Format("15:04"), formatElapsed(state.elapsed()))
		return nil
	},
}

var timerResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume a paused timer",
	RunE: func(cmd *cobra.Command, args []string) error {
		state, err := loadTimer()
		if err != nil {
			return err
		}
		if state == nil {
			return fmt.Errorf("no active timer running")
		}
		if !state.Paused {
			return fmt.Errorf("timer is already running for %s (%s elapsed)", state.IssueKey, formatElapsed(state.elapsed()))
		}

		state.ResumedAt = time.Now()
		state.Paused = false

		if err := saveTimer(state); err != nil {
			return err
		}
		fmt.Printf("▶  Timer resumed for %s at %s (%s elapsed)\n", state.IssueKey, state.ResumedAt.Format("15:04"), formatElapsed(state.elapsed()))
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

		elapsed := state.elapsed()
		if elapsed < time.Minute {
			if err := deleteTimer(); err != nil {
				return err
			}
			fmt.Println("Timer stopped (less than 1 minute elapsed, nothing logged).")
			return nil
		}

		timeSpent := durationToJira(elapsed)
		startTime := state.StartedAt.Format("2006-01-02T15:04:05.000-0700")

		var issueKey, description, label string

		if state.IsMeeting {
			templates, err := config.LoadTemplates()
			if err != nil {
				return fmt.Errorf("failed to load templates: %w", err)
			}
			var matched *jira.Template
			for i := range templates {
				t := &templates[i]
				if strings.ToUpper(t.BoardKey) == strings.ToUpper(state.MeetingBoardKey) &&
					strings.ToLower(t.Type) == strings.ToLower(state.MeetingType) {
					matched = t
					break
				}
			}
			if matched == nil {
				return fmt.Errorf("template not found for board_key=%s type=%s", state.MeetingBoardKey, state.MeetingType)
			}
			issueKey = matched.IssueKey
			description = matched.Description
			label = fmt.Sprintf("%s (%s)", matched.Name, matched.IssueKey)
		} else {
			issueKey = state.IssueKey
			label = issueKey
		}

		choice, err := ui.RunTimerConfirm(label, state.StartedAt, timeSpent)
		if err != nil {
			return fmt.Errorf("confirm prompt error: %w", err)
		}
		if choice != ui.TimerStopLog {
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
		response, err := client.AddWorklogWithStartTime(issueKey, timeSpent, startTime, description)
		if err != nil {
			return fmt.Errorf("failed to log work: %w", err)
		}

		if err := deleteTimer(); err != nil {
			return err
		}

		fmt.Printf("✓ Logged %s to %s\n", response.TimeSpent, label)
		return nil
	},
}

var timerMeetingCmd = &cobra.Command{
	Use:   "meeting [board_key] [type]",
	Short: "Start timer for a meeting template",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		boardKey := args[0]
		meetingType := args[1]

		existing, err := loadTimer()
		if err != nil {
			return err
		}
		if existing != nil {
			status := "running"
			if existing.Paused {
				status = "paused"
			}
			return fmt.Errorf("timer already %s (%s elapsed) — run 'gojira timer stop' first", status, formatElapsed(existing.elapsed()))
		}

		// Validate template exists
		templates, err := config.LoadTemplates()
		if err != nil {
			return fmt.Errorf("failed to load templates: %w", err)
		}
		var matched *jira.Template
		for i := range templates {
			t := &templates[i]
			if strings.ToUpper(t.BoardKey) == strings.ToUpper(boardKey) &&
				strings.ToLower(t.Type) == strings.ToLower(meetingType) {
				matched = t
				break
			}
		}
		if matched == nil {
			return fmt.Errorf("no template found for board_key=%s type=%s", boardKey, meetingType)
		}

		now := time.Now()
		state := &timerState{
			IssueKey:        matched.IssueKey,
			StartedAt:       now,
			ResumedAt:       now,
			IsMeeting:       true,
			MeetingBoardKey: boardKey,
			MeetingType:     meetingType,
		}
		if err := saveTimer(state); err != nil {
			return err
		}

		fmt.Printf("Timer started for %s (%s) at %s\n", matched.Name, matched.IssueKey, state.StartedAt.Format("15:04"))
		return nil
	},
}

func init() {
	timerCmd.AddCommand(timerStartCmd)
	timerCmd.AddCommand(timerStatusCmd)
	timerCmd.AddCommand(timerStopCmd)
	timerCmd.AddCommand(timerPauseCmd)
	timerCmd.AddCommand(timerResumeCmd)
	timerCmd.AddCommand(timerMeetingCmd)
}
