package jira

import (
	"strings"
	"time"
)

// JiraTime is a custom time type that handles Jira's timestamp format
type JiraTime struct {
	time.Time
}

// UnmarshalJSON implements custom JSON unmarshaling for Jira timestamps
func (jt *JiraTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "null" || s == "" {
		jt.Time = time.Time{}
		return nil
	}

	// Try multiple timestamp formats
	formats := []string{
		"2006-01-02T15:04:05.000-0700",   // Jira format: 2026-01-21T08:30:00.000+0300
		"2006-01-02T15:04:05.000Z0700",   // Alternative format
		time.RFC3339,                      // Standard RFC3339: 2026-01-21T08:30:00+03:00
		"2006-01-02T15:04:05Z07:00",      // Another common format
	}

	var err error
	for _, format := range formats {
		jt.Time, err = time.Parse(format, s)
		if err == nil {
			return nil
		}
	}

	return err
}

// Board represents a Jira board
type Board struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Location struct {
		ProjectKey string `json:"projectKey"`
	} `json:"location"`
}

// Sprint represents a Jira sprint
type Sprint struct {
	ID        int      `json:"id"`
	Name      string   `json:"name"`
	State     string   `json:"state"`
	BoardID   int      `json:"originBoardId"`
	StartDate JiraTime `json:"startDate"`
	EndDate   JiraTime `json:"endDate"`
}

// SprintsResponse represents the response from sprint queries
type SprintsResponse struct {
	MaxResults int      `json:"maxResults"`
	StartAt    int      `json:"startAt"`
	IsLast     bool     `json:"isLast"`
	Values     []Sprint `json:"values"`
}

// Issue represents a Jira issue
type Issue struct {
	ID     string      `json:"id"`
	Key    string      `json:"key"`
	Fields IssueFields `json:"fields"`
}

// IssueFields contains issue field data
type IssueFields struct {
	Summary  string  `json:"summary"`
	Status   Status  `json:"status"`
	Assignee *User   `json:"assignee"` // Pointer for nullable field
}

// Status represents an issue status
type Status struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// User represents a Jira user
type User struct {
	AccountID    string `json:"accountId"`
	EmailAddress string `json:"emailAddress"`
	DisplayName  string `json:"displayName"`
}

// IssuesResponse represents the response from issue queries
type IssuesResponse struct {
	Expand     string  `json:"expand"`
	StartAt    int     `json:"startAt"`
	MaxResults int     `json:"maxResults"`
	Total      int     `json:"total"`
	Issues     []Issue `json:"issues"`
}

// CurrentUser represents the current user from /myself endpoint
type CurrentUser struct {
	AccountID    string `json:"accountId"`
	EmailAddress string `json:"emailAddress"`
	DisplayName  string `json:"displayName"`
}

// ADFDoc represents an Atlassian Document Format document
type ADFDoc struct {
	Type    string       `json:"type"`
	Version int          `json:"version"`
	Content []ADFContent `json:"content"`
}

// ADFContent represents a content block in ADF
type ADFContent struct {
	Type    string       `json:"type"`
	Content []ADFContent `json:"content,omitempty"`
	Text    string       `json:"text,omitempty"`
}

// WorklogInput represents the input for creating a worklog
type WorklogInput struct {
	TimeSpent string  `json:"timeSpent"`
	Started   string  `json:"started"` // ISO 8601 format
	Comment   *ADFDoc `json:"comment,omitempty"`
}

// WorklogResponse represents the response from creating a worklog
type WorklogResponse struct {
	ID               string `json:"id"`
	IssueID          string `json:"issueId"`
	TimeSpent        string `json:"timeSpent"`
	TimeSpentSeconds int    `json:"timeSpentSeconds"`
}

// Worklog represents a worklog entry
type Worklog struct {
	ID               string   `json:"id"`
	IssueID          string   `json:"issueId"`
	Author           User     `json:"author"`
	TimeSpent        string   `json:"timeSpent"`
	TimeSpentSeconds int      `json:"timeSpentSeconds"`
	Started          JiraTime `json:"started"`
	Comment          *ADFDoc  `json:"comment,omitempty"`
}

// WorklogsResponse represents the response from fetching worklogs
type WorklogsResponse struct {
	StartAt    int       `json:"startAt"`
	MaxResults int       `json:"maxResults"`
	Total      int       `json:"total"`
	Worklogs   []Worklog `json:"worklogs"`
}

// ErrorResponse represents a Jira API error response
type ErrorResponse struct {
	ErrorMessages []string          `json:"errorMessages"`
	Errors        map[string]string `json:"errors"`
}

// Transition represents a Jira issue transition
type Transition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// TransitionsResponse represents the response from the transitions endpoint
type TransitionsResponse struct {
	Transitions []Transition `json:"transitions"`
}

// Template represents a predefined worklog template
type Template struct {
	Name        string `yaml:"name"`
	BoardKey    string `yaml:"board_key"`
	Type        string `yaml:"type"`
	IssueKey    string `yaml:"issue_key"`
	TimeSpent   string `yaml:"time_spent"`
	StartTime   string `yaml:"start_time"` // Format: "HH:MM"
	Description string `yaml:"description"`
}

// TemplatesConfig represents the configuration file containing templates
type TemplatesConfig struct {
	Boards map[string][]Template `yaml:"boards"`
}

// Meeting represents a meeting entry for a specific board
type Meeting struct {
	Name        string `yaml:"name"`
	IssueKey    string `yaml:"issue_key"`
	Description string `yaml:"description"`
}

// BoardMeetings represents meetings for a specific board
type BoardMeetings struct {
	BoardID  int       `yaml:"board_id"`
	Meetings []Meeting `yaml:"meetings"`
}

// MeetingsConfig represents the configuration file containing board-specific meetings
type MeetingsConfig struct {
	Boards []BoardMeetings `yaml:"boards"`
}

// SimpleMeetingsConfig represents a flat list of meetings without board association
type SimpleMeetingsConfig struct {
	Meetings []Meeting `yaml:"meetings"`
}
