# gojira

A command-line interface tool for interacting with Jira, built with Go using the Cobra library and standard `net/http` package.

## Features

- **List Sprint Tasks**: View all tasks assigned to you from the active sprint of a selected board
- **Log Work**: Log time spent on specific issues
- **Move Issues**: Transition issues to a new status using partial name matching
- **Interactive Board Selection**: Automatically prompts for board selection when multiple boards are configured
- **Table Output**: Clean, formatted table display for issue lists

## Prerequisites

- Go 1.16 or higher
- A Jira Cloud account
- Jira API token (see Configuration section)

## Installation

### Build from Source

```bash
# Clone or navigate to the project directory
cd gojira

# Build and install binary + templates in one step
make install
```

This will:
1. Build the binary and copy it to `~/go/bin/gojira`
2. Copy `templates.yaml` to `~/.config/gojira/templates.yaml`

Make sure `~/go/bin` is in your PATH:

```bash
export PATH=$PATH:$(HOME)/go/bin
```

## Configuration

### Step 1: Create a Jira API Token

1. Log in to your Jira Cloud account
2. Go to [https://id.atlassian.com/manage-profile/security/api-tokens](https://id.atlassian.com/manage-profile/security/api-tokens)
3. Click "Create API token"
4. Give it a label (e.g., "gojira")
5. Copy the generated token (you won't be able to see it again)

### Step 2: Find Your Board IDs

1. Navigate to your Jira board in a web browser
2. Look at the URL: `https://your-domain.atlassian.net/jira/software/projects/PROJ/boards/123`
3. The number at the end (e.g., `123`) is your board ID
4. Collect all board IDs you want to access

### Step 3: Configure Templates

Templates define your recurring meetings and worklogs. The tool looks for `templates.yaml` in:

1. `~/.config/gojira/templates.yaml` (global, recommended)
2. `./templates.yaml` (current directory, fallback)

`make install` copies the file automatically. To edit it afterwards:

```bash
$EDITOR ~/.config/gojira/templates.yaml
```

Configure your boards, issue keys, and meeting types:

```yaml
boards:
  MYBOARD:
    - name: "My Board Daily"
      issue_key: MYBOARD-1
      type: d
      start_time: "09:30"
      time_spent: 5m
      description: Daily
```

### Step 4: Configure Environment Variables

Create a `.env` file in the directory where you'll run the tool:

```bash
cp .env.example .env
```

Edit the `.env` file with your details:

```env
JIRA_URL=https://your-domain.atlassian.net
JIRA_EMAIL=your-email@example.com
JIRA_API_TOKEN=your_api_token_here
JIRA_BOARD_IDS=123,456,789
```

**Configuration Options:**

- `JIRA_URL`: Your Jira Cloud instance URL (without trailing slash)
- `JIRA_EMAIL`: The email address associated with your Jira account
- `JIRA_API_TOKEN`: The API token you generated
- `JIRA_BOARD_IDS`: Comma-separated list of board IDs (e.g., `123` or `123,456,789`)

## Usage

### List Tasks

List all tasks assigned to you from the active sprint:

```bash
gojira list
```

If you have multiple boards configured, you'll be prompted to select one:

```
Multiple boards found. Please select one:
1. Team A Board (ID: 123)
2. Team B Board (ID: 456)

Enter number: 1

Fetching tasks from board: Team A Board

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

Transition an issue to a new status using a partial name match:

```bash
gojira move <issue-key> <status>
```

If the issue key is omitted, it is inferred from the current git branch name.

**Examples:**

```bash
# Move to "In Progress"
gojira move PROJ-123 progress

# Move to "In Review"
gojira move PROJ-123 review

# Infer issue key from branch (e.g. feature/PROJ-123)
gojira move test
```

The status argument is matched case-insensitively. If no match is found, the available transitions are listed.

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

This project is provided as-is for personal and commercial use.

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.
