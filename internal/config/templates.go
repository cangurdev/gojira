package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"gojira/internal/jira"
)

func templatesPath() string {
	if home, err := os.UserHomeDir(); err == nil {
		global := filepath.Join(home, ".config", "gojira", "templates.yaml")
		if _, err := os.Stat(global); err == nil {
			return global
		}
	}
	return "templates.yaml"
}

// LoadTemplates reads and parses templates from ~/.config/gojira/templates.yaml
// falling back to templates.yaml in the current directory.
func LoadTemplates() ([]jira.Template, error) {
	data, err := os.ReadFile(templatesPath())
	if err != nil {
		return nil, fmt.Errorf("failed to read templates.yaml: %w", err)
	}

	var cfg jira.TemplatesConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse templates.yaml: %w", err)
	}

	if len(cfg.Boards) == 0 {
		return nil, fmt.Errorf("no templates found in templates.yaml")
	}

	var templates []jira.Template
	for boardKey, items := range cfg.Boards {
		for i := range items {
			items[i].BoardKey = boardKey
			templates = append(templates, items[i])
		}
	}

	return templates, nil
}

// LoadMeetings reads templates and converts them to Meeting structs for the interactive log command
func LoadMeetings() ([]jira.Meeting, error) {
	templates, err := LoadTemplates()
	if err != nil {
		return nil, err
	}

	meetings := make([]jira.Meeting, len(templates))
	for i, t := range templates {
		meetings[i] = jira.Meeting{
			Name:        t.Name,
			IssueKey:    t.IssueKey,
			Description: t.Description,
		}
	}

	return meetings, nil
}
