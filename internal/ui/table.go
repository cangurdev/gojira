package ui

import (
	"fmt"
	"os"
	"sort"
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

// PrintWorklogsTable prints worklogs grouped by day with daily totals.
func PrintWorklogsTable(worklogs []jira.WorklogWithIssue) {
	if len(worklogs) == 0 {
		fmt.Println("No worklogs found.")
		return
	}

	// Sort worklogs by started time
	sort.Slice(worklogs, func(i, j int) bool {
		return worklogs[i].Worklog.Started.Before(worklogs[j].Worklog.Started.Time)
	})

	// Group by day (YYYY-MM-DD key)
	type dayGroup struct {
		label   string
		entries []jira.WorklogWithIssue
		total   int
	}
	var days []dayGroup
	dayIndex := map[string]int{}

	for _, wl := range worklogs {
		key := wl.Worklog.Started.Format("2006-01-02")
		if _, ok := dayIndex[key]; !ok {
			dayIndex[key] = len(days)
			days = append(days, dayGroup{
				label: wl.Worklog.Started.Format("Mon, Jan 2"),
			})
		}
		idx := dayIndex[key]
		days[idx].entries = append(days[idx].entries, wl)
		days[idx].total += wl.Worklog.TimeSpentSeconds
	}

	grandTotal := 0
	for _, day := range days {
		h := day.total / 3600
		m := (day.total % 3600) / 60
		fmt.Printf("\n%s  —  %dh %dm\n", day.label, h, m)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		for _, wl := range day.entries {
			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n",
				wl.Worklog.Started.Format("15:04"),
				wl.IssueKey,
				truncate(wl.Summary, 45),
				wl.Worklog.TimeSpent)
		}
		w.Flush()
		grandTotal += day.total
	}

	h := grandTotal / 3600
	m := (grandTotal % 3600) / 60
	fmt.Printf("\nTotal: %dh %dm (%d worklogs)\n", h, m, len(worklogs))
}
