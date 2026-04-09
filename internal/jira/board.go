package jira

import "fmt"

// GetBoard retrieves board details by board ID
func (c *Client) GetBoard(boardID int) (*Board, error) {
	path := fmt.Sprintf("/rest/agile/1.0/board/%d", boardID)

	var board Board
	if err := c.doRequest("GET", path, nil, &board); err != nil {
		return nil, fmt.Errorf("failed to get board %d: %w", boardID, err)
	}

	return &board, nil
}

// GetBoardIssuesForCurrentUser retrieves issues on a board assigned to the current user, excluding done status
func (c *Client) GetBoardIssuesForCurrentUser(boardID int, accountID string) ([]Issue, error) {
	jql := fmt.Sprintf(`assignee = "%s" AND status != Done`, accountID)
	path := fmt.Sprintf("/rest/agile/1.0/board/%d/issue?fields=key,summary,status,assignee&jql=%s", boardID, jql)

	var response IssuesResponse
	if err := c.doRequest("GET", path, nil, &response); err != nil {
		return nil, fmt.Errorf("failed to get issues for board %d: %w", boardID, err)
	}

	return response.Issues, nil
}
