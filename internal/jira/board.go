package jira

import (
	"fmt"
	"net/url"
)

// GetBoard retrieves board details by board ID
func (c *Client) GetBoard(boardID int) (*Board, error) {
	path := fmt.Sprintf("/rest/agile/1.0/board/%d", boardID)

	var board Board
	if err := c.doRequest("GET", path, nil, &board); err != nil {
		return nil, fmt.Errorf("failed to get board %d: %w", boardID, err)
	}

	return &board, nil
}

// GetBoardConfiguration retrieves a board's column configuration.
func (c *Client) GetBoardConfiguration(boardID int) (*BoardConfiguration, error) {
	path := fmt.Sprintf("/rest/agile/1.0/board/%d/configuration", boardID)

	var config BoardConfiguration
	if err := c.doRequest("GET", path, nil, &config); err != nil {
		return nil, fmt.Errorf("failed to get board configuration %d: %w", boardID, err)
	}

	return &config, nil
}

// GetBoardIssues retrieves all non-done issues on a board (no assignee filter).
// Uses statusCategory to exclude anything Jira classifies as done (Done, Closed,
// Resolved, etc.), so kanban boards don't render a huge Done column.
func (c *Client) GetBoardIssues(boardID int) ([]Issue, error) {
	jql := url.QueryEscape("statusCategory != Done")
	path := fmt.Sprintf("/rest/agile/1.0/board/%d/issue?fields=key,summary,status,assignee,issuetype&maxResults=200&jql=%s", boardID, jql)

	var response IssuesResponse
	if err := c.doRequest("GET", path, nil, &response); err != nil {
		return nil, fmt.Errorf("failed to get issues for board %d: %w", boardID, err)
	}

	return response.Issues, nil
}
