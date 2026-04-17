# gojira

A modern, interactive CLI tool for Jira with a focus on speed and productivity.

## Features

- **Interactive Board**: Full-screen Kanban/Sprint board TUI with move and worklog support.
- **Worklog Summary**: Interactive pivot table to view, edit, and delete your worklogs.
- **Smart Logging**: Quick worklog from git branch or predefined templates (daily, meetings).
- **Timers**: Built-in timer to track and log time spent on issues.
- **Transitions**: Fast issue status updates with partial matching.

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

- `h/l`: Switch columns
- `j/k`: Navigate issues
- `m`: Move (transition) issue
- `w`: Log work
- `a`: Toggle "Mine Only" filter
- `o`: Open in browser
- `r`: Refresh

## License

MIT
