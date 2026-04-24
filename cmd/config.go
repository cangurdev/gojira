package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"gojira/internal/config"
	"gojira/internal/ui"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configure Jira credentials interactively",
	Long: `Create or update ~/.config/gojira/.env interactively.

You will be prompted for:
  - JIRA_URL
  - JIRA_EMAIL
  - JIRA_API_TOKEN
  - JIRA_BOARD_IDS`,
	RunE: runConfigCommand,
}

func runConfigCommand(cmd *cobra.Command, args []string) error {
	configDir, err := config.ConfigDir()
	if err != nil {
		return fmt.Errorf("failed to resolve config directory: %w", err)
	}

	envPath := filepath.Join(configDir, ".env")
	existing, _ := godotenv.Read(envPath)

	jiraURL, err := ui.PromptInputWithDefault("Jira URL", strings.TrimSpace(existing["JIRA_URL"]))
	if err != nil {
		return err
	}
	if jiraURL == "" {
		return fmt.Errorf("JIRA_URL cannot be empty")
	}

	jiraEmail, err := ui.PromptInputWithDefault("Jira Email", strings.TrimSpace(existing["JIRA_EMAIL"]))
	if err != nil {
		return err
	}
	if jiraEmail == "" {
		return fmt.Errorf("JIRA_EMAIL cannot be empty")
	}

	jiraToken, err := ui.PromptInputWithDefault("Jira API Token", strings.TrimSpace(existing["JIRA_API_TOKEN"]))
	if err != nil {
		return err
	}
	if jiraToken == "" {
		return fmt.Errorf("JIRA_API_TOKEN cannot be empty")
	}

	boardIDsInput, err := ui.PromptInputWithDefault("Jira Board IDs (comma-separated)", strings.TrimSpace(existing["JIRA_BOARD_IDS"]))
	if err != nil {
		return err
	}
	boardIDs, err := parseAndNormalizeBoardIDs(boardIDsInput)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	content := strings.Join([]string{
		"JIRA_URL=" + strings.TrimSpace(jiraURL),
		"JIRA_EMAIL=" + strings.TrimSpace(jiraEmail),
		"JIRA_API_TOKEN=" + strings.TrimSpace(jiraToken),
		"JIRA_BOARD_IDS=" + boardIDs,
		"",
	}, "\n")

	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("failed to write %s: %w", envPath, err)
	}

	fmt.Printf("✓ Configuration saved: %s\n", envPath)
	return nil
}

func parseAndNormalizeBoardIDs(input string) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", fmt.Errorf("JIRA_BOARD_IDS cannot be empty")
	}

	parts := strings.Split(input, ",")
	normalized := make([]string, 0, len(parts))

	for _, part := range parts {
		idStr := strings.TrimSpace(part)
		if idStr == "" {
			continue
		}
		if _, err := strconv.Atoi(idStr); err != nil {
			return "", fmt.Errorf("invalid board ID '%s': must be a number", idStr)
		}
		normalized = append(normalized, idStr)
	}

	if len(normalized) == 0 {
		return "", fmt.Errorf("at least one board ID is required in JIRA_BOARD_IDS")
	}

	return strings.Join(normalized, ","), nil
}