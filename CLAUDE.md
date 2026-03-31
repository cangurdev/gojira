# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Run Commands

```bash
# Build the binary
go build -o bin/gojira

# Run without building
go run main.go

# Run specific command
go run main.go list
go run main.go log PROJ-123 "2h"
```

## Architecture

This is a Go CLI tool for interacting with Jira Cloud, built with Cobra for command handling.

### Package Structure

- **cmd/** - Cobra command definitions (`list`, `log`). Each command loads config, creates a Jira client, and orchestrates the workflow.
- **internal/jira/** - Jira API client using `net/http` with Basic Auth. The `Client` struct handles all HTTP requests via `doRequest()` method.
- **internal/config/** - Loads configuration from `.env` file using godotenv. Required env vars: `JIRA_URL`, `JIRA_EMAIL`, `JIRA_API_TOKEN`, `JIRA_BOARD_IDS`.
- **internal/ui/** - Terminal UI utilities for table output and interactive prompts.

### Key Patterns

- All Jira API methods are on the `Client` struct in `internal/jira/`
- Commands follow the pattern: load config -> create client -> call API methods -> display results
- Error handling wraps errors with context using `fmt.Errorf("...: %w", err)`
