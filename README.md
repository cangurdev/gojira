# gojira

A modern, interactive CLI tool for Jira with a focus on speed and productivity.

## Features

- **List Sprint Tasks**: View all tasks assigned to you from the active sprint of a selected board
- **Log Work**: Log time spent on specific issues
- **Move Issues**: Transition issues to a new status with partial matching or interactive board column selection
- **Interactive Board Selection**: Automatically prompts for board selection when multiple boards are configured
- **Table Output**: Clean, formatted table display for issue lists

## Prerequisites

- Go 1.16 or higher
- A Jira Cloud account
- Jira API token (see Configuration section)

## Installation

 Run the install command:
   ```bash
   make install
   ```

This builds the binary and sets up your `templates.yaml` in `~/.config/gojira/templates.yaml`. Ensure `~/go/bin` is in your `PATH`.

## Configuration

Create a `.env` file in your workspace:

```env
JIRA_URL=https://your-domain.atlassian.net
JIRA_EMAIL=your-email@example.com
JIRA_API_TOKEN=your_api_token
JIRA_BOARD_IDS=123,456
```

1. **API Token**: Create one at [id.atlassian.com](https://id.atlassian.com/manage-profile/security/api-tokens).
2. **Board IDs**: Find the ID at the end of your Jira board URL.
3. **Templates**: Edit `~/.config/gojira/templates.yaml` to define recurring meetings.

### Templates (`templates.yaml`)

Define recurring worklogs for different boards. The `gojira m` command uses these to quickly log meetings:

```yaml
boards:
  PROJECT_KEY:
    - name: "Daily Standup"
      issue_key: PROJ-1
      type: d
      start_time: "09:30"
      time_spent: 15m
      description: Daily standup meeting
```

- `type`: Used as a shortcut (e.g., `gojira m d` logs the Daily).
- `start_time`: Used if no start time is provided during logging.
- `time_spent`: Default duration (e.g., `15m`, `1h`).

## Commands

| Command | Description |
| :--- | :--- |
| `gojira board` | **Main interface**. Full-screen interactive board. |
| `gojira summary` | Interactive worklog manager (edit/delete/view). |
| `gojira timer` | Start/stop timer for the current issue. |
| `gojira log` | Log work manually or inferred from git branch. |
| `gojira m` | Log work from predefined meeting templates. |
| `gojira move` | Fast issue status transition. |

## Shortcuts (Board TUI)

Active sprint: Sprint 42

Issues assigned to John Doe:

KEY         SUMMARY                                                      STATUS
---         -------                                                      ------
PROJ-101    Implement user authentication                                In Progress
PROJ-102    Fix login bug                                                To Do
PROJ-105    Add password reset feature                                   In Review
```

### Log Work

Log time spent on a specific issue:

```bash
gojira log <issue-key> <time-spent>
```

**Time Format:**

The tool accepts Jira's native time format with spaces:

- `1h` - 1 hour
- `30m` - 30 minutes
- `1h 30m` - 1 hour 30 minutes
- `2d` - 2 days
- `2d 4h` - 2 days 4 hours
- `1w 2d 4h 30m` - 1 week, 2 days, 4 hours, 30 minutes

**Examples:**

```bash
# Log 2 hours
gojira log PROJ-101 "2h"

# Log 1 hour 30 minutes
gojira log PROJ-102 "1h 30m"

# Log 2 days
gojira log PROJ-103 "2d"

# Log complex time
gojira log PROJ-104 "1d 4h 30m"
```

**Output:**

```
Successfully logged 2h to PROJ-101
Worklog ID: 12345
```

### Move Issue

Transition an issue to a new status, or choose a board column interactively:

```bash
gojira move [issue-key] [status]
```

If the issue key is omitted, it is inferred from the current git branch name. If
the status is omitted, the tool asks you to pick one of the selected board's
columns and moves the issue to a matching transition for that column.

**Examples:**

```bash
# Select a target board column interactively
gojira move PROJ-123

# Move to "In Progress"
gojira move PROJ-123 progress

# Move to "In Review"
gojira move PROJ-123 review

# Infer issue key from branch and select column interactively
gojira move

# Infer issue key from branch (e.g. feature/PROJ-123)
gojira move test
```

The status argument is matched case-insensitively. If no match is found, the available transitions are listed. Interactive column selection uses the board's configured columns and mapped statuses.

**Output:**

```
✓ PROJ-123 → In Progress
```

### Help

Get help on available commands:

```bash
gojira --help
gojira list --help
gojira log --help
gojira move --help
```

## Project Structure

```
gojira/
├── .env.example              # Template for environment variables
├── .gitignore               # Git ignore rules
├── go.mod                   # Go module definition
├── go.sum                   # Go dependencies checksums
├── main.go                  # Application entry point
├── README.md                # This file
├── cmd/
│   ├── root.go             # Root command setup
│   ├── list.go             # List command implementation
│   ├── log.go              # Log command implementation
│   └── move.go             # Move command implementation
├── internal/
│   ├── config/
│   │   └── config.go       # Configuration loading
│   ├── jira/
│   │   ├── client.go       # HTTP client with Basic Auth
│   │   ├── types.go        # Jira API response types
│   │   ├── board.go        # Board API methods
│   │   ├── sprint.go       # Sprint API methods
│   │   └── worklog.go      # Worklog API methods
│   └── ui/
│       ├── table.go        # Table formatting
│       └── prompt.go       # Interactive prompts
└── bin/
    └── gojira            # Compiled binary
```

## Error Handling

The tool provides clear error messages for common issues:

- **Missing configuration**: "JIRA_URL is required in .env file or environment"
- **Invalid credentials**: "jira api error (status 401): ..."
- **No active sprint**: "no active sprint found for board X"
- **Invalid issue key**: "invalid issue key format: XYZ (expected format: PROJ-123)"
- **Board not found**: "jira api error (status 404): ..."

## Security Notes

- **Never commit your `.env` file** to version control (it's in `.gitignore` by default)
- Keep your API token secure and don't share it
- The tool uses HTTPS for all Jira API communications
- API tokens can be revoked at any time from your Atlassian account settings

## Dependencies

- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [godotenv](https://github.com/joho/godotenv) - Environment variable loading
- Standard library packages only for HTTP communication

## Troubleshooting

### "No active sprint found"

Your board doesn't have an active sprint. Make sure you have started a sprint in Jira.

### "No issues found"

Either there are no issues in the active sprint, or none are assigned to you. Check your Jira board.

### Authentication errors

- Verify your email address matches your Jira account
- Regenerate your API token if it's expired
- Ensure your Jira URL is correct (should be `https://your-domain.atlassian.net`)

### Board ID errors

Double-check your board IDs by looking at the URL when viewing the board in Jira.

## License

MIT
