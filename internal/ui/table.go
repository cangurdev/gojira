package ui

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"gojira/internal/jira"
)

// PrintIssuesTable prints issues in a formatted table
func PrintIssuesTable(issues []jira.Issue) {
	if len(issues) == 0 {
		fmt.Println("No issues found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	// Print header
	fmt.Fprintln(w, "KEY\tSUMMARY\tSTATUS")
	fmt.Fprintln(w, "---\t-------\t------")

	// Print rows
	for _, issue := range issues {
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			issue.Key,
			truncate(issue.Fields.Summary, 60),
			issue.Fields.Status.Name)
	}

	w.Flush()
}

// truncate truncates a string to maxLen characters and adds "..." if truncated
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// PrintWorklogsTable prints worklogs in a formatted table grouped by project
func PrintWorklogsTable(worklogs []jira.WorklogWithIssue) {
	if len(worklogs) == 0 {
		fmt.Println("No worklogs found for the current week.")
		return
	}

	// Group worklogs by project
	projectGroups := make(map[string][]jira.WorklogWithIssue)
	projectOrder := []string{}

	for _, wl := range worklogs {
		// Extract project key (everything before the dash)
		parts := strings.SplitN(wl.IssueKey, "-", 2)
		projectKey := parts[0]

		if _, exists := projectGroups[projectKey]; !exists {
			projectOrder = append(projectOrder, projectKey)
		}
		projectGroups[projectKey] = append(projectGroups[projectKey], wl)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	grandTotalSeconds := 0

	// Print each project group
	for _, projectKey := range projectOrder {
		projectWorklogs := projectGroups[projectKey]
		projectTotalSeconds := 0

		// Print project header
		fmt.Printf("\n%s\n", projectKey)
		fmt.Fprintln(w, "DATE\tTIME\tISSUE\tSUMMARY\tTIME SPENT")
		fmt.Fprintln(w, "----\t----\t-----\t-------\t----------")

		// Print worklogs for this project
		for _, wl := range projectWorklogs {
			date := wl.Worklog.Started.Format("Mon 01/02")
			time := wl.Worklog.Started.Format("15:04")

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				date,
				time,
				wl.IssueKey,
				truncate(wl.Summary, 50),
				wl.Worklog.TimeSpent)

			projectTotalSeconds += wl.Worklog.TimeSpentSeconds
		}

		w.Flush()

		// Print project subtotal
		hours := projectTotalSeconds / 3600
		minutes := (projectTotalSeconds % 3600) / 60
		fmt.Printf("  Subtotal: %dh %dm\n", hours, minutes)

		grandTotalSeconds += projectTotalSeconds
	}

	// Print grand total
	hours := grandTotalSeconds / 3600
	minutes := (grandTotalSeconds % 3600) / 60
	fmt.Printf("\nTotal: %dh %dm (%d worklogs)\n", hours, minutes, len(worklogs))
}
