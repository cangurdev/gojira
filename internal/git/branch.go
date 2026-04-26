package git

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// CreateAndCheckoutBranch creates a new local branch and checks it out.
func CreateAndCheckoutBranch(branchName string) error {
	branchName = strings.TrimSpace(branchName)
	if branchName == "" {
		return fmt.Errorf("branch name cannot be empty")
	}

	cmd := exec.Command("git", "switch", "-c", branchName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create branch %q: %s", branchName, strings.TrimSpace(string(output)))
	}

	return nil
}

// GetCurrentBranch returns the current git branch name
func GetCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// ExtractIssueKey extracts the issue key from a branch name
// Supports patterns like: feature/OS-111, fix/OS-123, bugfix/PROJ-456
func ExtractIssueKey(branchName string) (string, error) {
	// Pattern to match issue keys like OS-111, PROJ-123
	pattern := `([A-Z]+-[0-9]+)`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(branchName)
	if len(matches) < 2 {
		return "", fmt.Errorf("no issue key found in branch name: %s", branchName)
	}
	return matches[1], nil
}

// GetIssueKeyFromBranch gets the current branch and extracts the issue key
func GetIssueKeyFromBranch() (string, error) {
	branch, err := GetCurrentBranch()
	if err != nil {
		return "", err
	}
	return ExtractIssueKey(branch)
}
