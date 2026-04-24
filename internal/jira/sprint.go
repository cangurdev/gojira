package jira

import (
	"fmt"
	"slices"
	"strings"
)

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
	path := fmt.Sprintf("/rest/agile/1.0/sprint/%d/issue?fields=key,summary,status,assignee,issuetype", sprintID)

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

// GetTransitions retrieves available transitions for an issue
func (c *Client) GetTransitions(issueKey string) ([]Transition, error) {
	path := fmt.Sprintf("/rest/api/3/issue/%s/transitions", issueKey)

	var response TransitionsResponse
	if err := c.doRequest("GET", path, nil, &response); err != nil {
		return nil, fmt.Errorf("failed to get transitions for %s: %w", issueKey, err)
	}

	return response.Transitions, nil
}

// DoTransition performs a transition by ID (no name matching)
func (c *Client) DoTransition(issueKey, transitionID string) error {
	body := map[string]interface{}{
		"transition": map[string]string{"id": transitionID},
	}
	path := fmt.Sprintf("/rest/api/3/issue/%s/transitions", issueKey)
	if err := c.doRequest("POST", path, body, nil); err != nil {
		return fmt.Errorf("failed to transition %s: %w", issueKey, err)
	}
	return nil
}

// TransitionIssue moves an issue to the transition that matches the given name (case-insensitive partial match)
func (c *Client) TransitionIssue(issueKey, transitionName string) (*Transition, error) {
	transitions, err := c.GetTransitions(issueKey)
	if err != nil {
		return nil, err
	}

	aliases := map[string]string{
		"review": "ready for qa",
	}

	query := strings.ToLower(transitionName)
	if mapped, ok := aliases[query]; ok {
		query = mapped
	}

	for _, t := range transitions {
		if strings.Contains(strings.ToLower(t.Name), query) {
			body := map[string]interface{}{
				"transition": map[string]string{"id": t.ID},
			}
			path := fmt.Sprintf("/rest/api/3/issue/%s/transitions", issueKey)
			if err := c.doRequest("POST", path, body, nil); err != nil {
				return nil, fmt.Errorf("failed to transition %s: %w", issueKey, err)
			}
			return &t, nil
		}
	}

	// No match — list available transitions in the error
	names := make([]string, len(transitions))
	for i, t := range transitions {
		names[i] = t.Name
	}
	return nil, fmt.Errorf("no transition matching %q found for %s\nAvailable: %s",
		transitionName, issueKey, strings.Join(names, ", "))
}

// TransitionIssueToBoardColumn moves an issue to the transition that targets
// one of the statuses configured in the selected board column.
func (c *Client) TransitionIssueToBoardColumn(issueKey string, column BoardColumn) (*Transition, error) {
	transitions, err := c.GetTransitions(issueKey)
	if err != nil {
		return nil, err
	}

	matches := MatchTransitionsToBoardColumns([]BoardColumn{column}, transitions, "")
	if len(matches) > 0 {
		transition := matches[0].Transition
		if err := c.DoTransition(issueKey, transition.ID); err != nil {
			return nil, err
		}
		return &transition, nil
	}

	statusNames := make([]string, 0, len(column.Statuses))
	for _, status := range column.Statuses {
		if status.Name != "" {
			statusNames = append(statusNames, strings.ToLower(status.Name))
		}
	}

	available := make([]string, 0, len(transitions))
	for _, transition := range transitions {
		label := transition.Name
		if transition.To.Name != "" {
			label = fmt.Sprintf("%s → %s", transition.Name, transition.To.Name)
		}
		available = append(available, label)
	}

	return nil, fmt.Errorf(
		"no transition from %s matches board column %q (statuses: %s)\nAvailable: %s",
		issueKey,
		column.Name,
		strings.Join(statusNames, ", "),
		strings.Join(available, ", "),
	)
}

// MatchTransitionsToBoardColumns returns board columns that have a matching
// available transition. currentStatusName can be provided to exclude the
// current column from the result.
func MatchTransitionsToBoardColumns(columns []BoardColumn, transitions []Transition, currentStatusName string) []BoardColumnTransition {
	currentStatusName = strings.ToLower(strings.TrimSpace(currentStatusName))
	matches := make([]BoardColumnTransition, 0, len(columns))
	usedTransitionIDs := make(map[string]bool)

	for _, column := range columns {
		statusIDs := make([]string, 0, len(column.Statuses))
		statusNames := make([]string, 0, len(column.Statuses))
		isCurrentColumn := false

		for _, status := range column.Statuses {
			if status.ID != "" {
				statusIDs = append(statusIDs, status.ID)
			}
			if status.Name != "" {
				normalized := strings.ToLower(status.Name)
				statusNames = append(statusNames, normalized)
				if currentStatusName != "" && normalized == currentStatusName {
					isCurrentColumn = true
				}
			}
		}

		if isCurrentColumn {
			continue
		}

		matched := false
		for _, transition := range transitions {
			if usedTransitionIDs[transition.ID] {
				continue
			}
			if transition.To.ID != "" && slices.Contains(statusIDs, transition.To.ID) {
				matches = append(matches, BoardColumnTransition{Column: column, Transition: transition})
				usedTransitionIDs[transition.ID] = true
				matched = true
				break
			}
		}
		if matched {
			continue
		}

		for _, transition := range transitions {
			if usedTransitionIDs[transition.ID] {
				continue
			}
			targetStatusName := strings.ToLower(transition.To.Name)
			if targetStatusName != "" && slices.Contains(statusNames, targetStatusName) {
				matches = append(matches, BoardColumnTransition{Column: column, Transition: transition})
				usedTransitionIDs[transition.ID] = true
				matched = true
				break
			}
		}
		if matched {
			continue
		}

		columnName := strings.ToLower(column.Name)
		if columnName == "" {
			continue
		}

		for _, transition := range transitions {
			if usedTransitionIDs[transition.ID] {
				continue
			}
			transitionName := strings.ToLower(transition.Name)
			if transitionName == columnName || strings.Contains(transitionName, columnName) {
				matches = append(matches, BoardColumnTransition{Column: column, Transition: transition})
				usedTransitionIDs[transition.ID] = true
				break
			}
		}
	}

	return matches
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
