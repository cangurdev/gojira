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
