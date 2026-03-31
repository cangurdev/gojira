package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// ConfigDir returns the gojira config directory path
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".config", "gojira"), nil
}

// Config holds the Jira configuration loaded from environment variables
type Config struct {
	JiraURL      string
	JiraEmail    string
	JiraAPIToken string
	BoardIDs     []int
}

// Load reads configuration from .env file and environment variables
func Load() (*Config, error) {
	// Load .env file from config directory
	configDir, err := ConfigDir()
	if err == nil {
		_ = godotenv.Load(filepath.Join(configDir, ".env"))
	}

	cfg := &Config{
		JiraURL:      os.Getenv("JIRA_URL"),
		JiraEmail:    os.Getenv("JIRA_EMAIL"),
		JiraAPIToken: os.Getenv("JIRA_API_TOKEN"),
	}

	// Validate required fields
	if cfg.JiraURL == "" {
		return nil, fmt.Errorf("JIRA_URL is required in .env file or environment")
	}
	if cfg.JiraEmail == "" {
		return nil, fmt.Errorf("JIRA_EMAIL is required in .env file or environment")
	}
	if cfg.JiraAPIToken == "" {
		return nil, fmt.Errorf("JIRA_API_TOKEN is required in .env file or environment")
	}

	// Parse board IDs
	boardIDsStr := os.Getenv("JIRA_BOARD_IDS")
	if boardIDsStr == "" {
		return nil, fmt.Errorf("JIRA_BOARD_IDS is required in .env file or environment")
	}

	// Split by comma and parse each ID
	boardIDsParts := strings.Split(boardIDsStr, ",")
	for _, idStr := range boardIDsParts {
		idStr = strings.TrimSpace(idStr)
		if idStr == "" {
			continue
		}
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return nil, fmt.Errorf("invalid board ID '%s': must be a number", idStr)
		}
		cfg.BoardIDs = append(cfg.BoardIDs, id)
	}

	if len(cfg.BoardIDs) == 0 {
		return nil, fmt.Errorf("at least one board ID is required in JIRA_BOARD_IDS")
	}

	return cfg, nil
}
