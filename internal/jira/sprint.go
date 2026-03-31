package jira

import "fmt"

// GetActiveSprint retrieves the active sprint for a board
func (c *Client) GetActiveSprint(boardID int) (*Sprint, error) {
	path := fmt.Sprintf("/rest/agile/1.0/board/%d/sprint?state=active", boardID)

	var response SprintsResponse
	if err := c.doRequest("GET", path, nil, &response); err != nil {
		return nil, fmt.Errorf("failed to get active sprint for board %d: %w", boardID, err)
	}

	if len(response.Values) == 0 {
		return nil, fmt.Errorf("no active sprint found for board %d", boardID)
	}

	// Return the first active sprint
	return &response.Values[0], nil
}

// GetSprintIssues retrieves all issues in a sprint
func (c *Client) GetSprintIssues(sprintID int) ([]Issue, error) {
	path := fmt.Sprintf("/rest/agile/1.0/sprint/%d/issue?fields=key,summary,status,assignee", sprintID)

	var response IssuesResponse
	if err := c.doRequest("GET", path, nil, &response); err != nil {
		return nil, fmt.Errorf("failed to get issues for sprint %d: %w", sprintID, err)
	}

	return response.Issues, nil
}

// GetCurrentUser retrieves the current authenticated user
func (c *Client) GetCurrentUser() (*CurrentUser, error) {
	path := "/rest/api/3/myself"

	var user CurrentUser
	if err := c.doRequest("GET", path, nil, &user); err != nil {
		return nil, fmt.Errorf("failed to get current user: %w", err)
	}

	return &user, nil
}

// SearchIssuesByJQL searches for issues using JQL (Jira Query Language)
func (c *Client) SearchIssuesByJQL(jql string) ([]Issue, error) {
	path := "/rest/api/3/search/jql"

	requestBody := map[string]interface{}{
		"jql":    jql,
		"fields": []string{"key", "summary", "status"},
	}

	var response IssuesResponse
	if err := c.doRequest("POST", path, requestBody, &response); err != nil {
		return nil, fmt.Errorf("failed to search issues with JQL: %w", err)
	}

	return response.Issues, nil
}
