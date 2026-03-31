package jira

import (
	"fmt"
	"time"
)

// AddWorklog logs work to a Jira issue
func (c *Client) AddWorklog(issueKey, timeSpent string, description string) (*WorklogResponse, error) {
	path := fmt.Sprintf("/rest/api/3/issue/%s/worklog", issueKey)

	// Use current time as the start time in ISO 8601 format
	now := time.Now().Format("2006-01-02T15:04:05.000-0700")

	input := WorklogInput{
		TimeSpent: timeSpent,
		Started:   now,
	}

	// Only add comment if description is provided
	if description != "" {
		input.Comment = &ADFDoc{
			Type:    "doc",
			Version: 1,
			Content: []ADFContent{
				{
					Type: "paragraph",
					Content: []ADFContent{
						{
							Type: "text",
							Text: description,
						},
					},
				},
			},
		}
	}

	var response WorklogResponse
	if err := c.doRequest("POST", path, input, &response); err != nil {
		return nil, fmt.Errorf("failed to add worklog to issue %s: %w", issueKey, err)
	}

	return &response, nil
}

// AddWorklogWithStartTime logs work to a Jira issue with a custom start time
func (c *Client) AddWorklogWithStartTime(issueKey, timeSpent, startedTime, description string) (*WorklogResponse, error) {
	path := fmt.Sprintf("/rest/api/3/issue/%s/worklog", issueKey)

	input := WorklogInput{
		TimeSpent: timeSpent,
		Started:   startedTime,
	}

	// Only add comment if description is provided
	if description != "" {
		input.Comment = &ADFDoc{
			Type:    "doc",
			Version: 1,
			Content: []ADFContent{
				{
					Type: "paragraph",
					Content: []ADFContent{
						{
							Type: "text",
							Text: description,
						},
					},
				},
			},
		}
	}

	var response WorklogResponse
	if err := c.doRequest("POST", path, input, &response); err != nil {
		return nil, fmt.Errorf("failed to add worklog to issue %s: %w", issueKey, err)
	}

	return &response, nil
}

// GetIssueWorklogs retrieves all worklogs for a specific issue
func (c *Client) GetIssueWorklogs(issueKey string) ([]Worklog, error) {
	path := fmt.Sprintf("/rest/api/3/issue/%s/worklog", issueKey)

	var response WorklogsResponse
	if err := c.doRequest("GET", path, nil, &response); err != nil {
		return nil, fmt.Errorf("failed to get worklogs for issue %s: %w", issueKey, err)
	}

	return response.Worklogs, nil
}

// WorklogWithIssue represents a worklog with its associated issue information
type WorklogWithIssue struct {
	Worklog  Worklog
	IssueKey string
	Summary  string
}

// GetUserWorklogsForWeek retrieves all worklogs for the current user for the current week
func (c *Client) GetUserWorklogsForWeek() ([]WorklogWithIssue, error) {
	// Get current user
	currentUser, err := c.GetCurrentUser()
	if err != nil {
		return nil, fmt.Errorf("failed to get current user: %w", err)
	}

	// Calculate start of week (Monday)
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 { // Sunday
		weekday = 7
	}
	startOfWeek := now.AddDate(0, 0, -(weekday - 1))
	startOfWeek = time.Date(startOfWeek.Year(), startOfWeek.Month(), startOfWeek.Day(), 0, 0, 0, 0, startOfWeek.Location())

	// Build JQL query to find issues where the user logged work this week
	startOfWeekStr := startOfWeek.Format("2006-01-02")
	jql := fmt.Sprintf("worklogAuthor = currentUser() AND worklogDate >= '%s'", startOfWeekStr)

	// Search for issues
	issues, err := c.SearchIssuesByJQL(jql)
	if err != nil {
		return nil, fmt.Errorf("failed to search issues: %w", err)
	}

	// Collect worklogs from all issues
	var weeklyWorklogs []WorklogWithIssue

	for _, issue := range issues {
		worklogs, err := c.GetIssueWorklogs(issue.Key)
		if err != nil {
			// Log error but continue with other issues
			fmt.Printf("Warning: failed to get worklogs for issue %s: %v\n", issue.Key, err)
			continue
		}

		// Filter worklogs by current user and date range
		for _, worklog := range worklogs {
			if worklog.Author.AccountID == currentUser.AccountID &&
				worklog.Started.After(startOfWeek.Add(-time.Second)) {
				weeklyWorklogs = append(weeklyWorklogs, WorklogWithIssue{
					Worklog:  worklog,
					IssueKey: issue.Key,
					Summary:  issue.Fields.Summary,
				})
			}
		}
	}

	return weeklyWorklogs, nil
}
