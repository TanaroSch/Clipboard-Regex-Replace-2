package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

// ProfileConfig represents a single regex replacement profile
type ProfileConfig struct {
	Name          string        `json:"name"`                     // Display name for the profile
	Enabled       bool          `json:"enabled"`                  // Whether the profile is active
	Hotkey        string        `json:"hotkey"`                   // Hotkey combination to trigger this profile
	ReverseHotkey string        `json:"reverse_hotkey,omitempty"` // Optional hotkey for reverse replacements
	Replacements  []Replacement `json:"replacements"`             // Regex replacement rules for this profile
}

// Config holds the application configuration
type Config struct {
	UseNotifications   bool            `json:"use_notifications"`   // Whether to show notifications
	TemporaryClipboard bool            `json:"temporary_clipboard"` // Whether to store original clipboard
	AutomaticReversion bool            `json:"automatic_reversion"` // Whether to revert clipboard after paste
	RevertHotkey       string          `json:"revert_hotkey"`       // Hotkey for manual reversion
	Profiles           []ProfileConfig `json:"profiles"`            // List of replacement profiles

	// Legacy support fields (for backward compatibility)
	Hotkey       string        `json:"hotkey,omitempty"`       // Legacy hotkey field
	Replacements []Replacement `json:"replacements,omitempty"` // Legacy replacements field

	// Non-JSON fields (runtime state)
	configPath string
}

// Replacement represents one regex replacement rule
type Replacement struct {
	Regex        string `json:"regex"`
	ReplaceWith  string `json:"replace_with"`
	PreserveCase bool   `json:"preserve_case,omitempty"` // Case preservation flag
	ReverseWith  string `json:"reverse_with,omitempty"`  // Optional override for reverse replacement
}

// GetConfigPath returns the path to the configuration file
func (c *Config) GetConfigPath() string {
	return c.configPath
}

// Load reads and parses the configuration file with backward compatibility
func Load(configPath string) (*Config, error) {
	var config Config

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	
	// Store config path for future saves
	config.configPath = configPath

	// Handle backward compatibility - migrate from legacy format to profiles
	if config.Hotkey != "" && len(config.Replacements) > 0 && len(config.Profiles) == 0 {
		// Convert old format to new format with a "Default" profile
		config.Profiles = []ProfileConfig{
			{
				Name:         "Default",
				Enabled:      true,
				Hotkey:       config.Hotkey,
				Replacements: config.Replacements,
			},
		}

		// Clear legacy fields to avoid confusion
		config.Hotkey = ""
		config.Replacements = nil

		// Save the migrated config
		config.Save()
	}

	return &config, nil
}

// Save writes the current configuration back to the config.json file
func (c *Config) Save() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(c.configPath, data, 0644)
}

// CreateDefaultConfig creates a default configuration file if none exists
func CreateDefaultConfig(configPath string) error {
	// Check if file already exists
	if _, err := os.Stat(configPath); err == nil {
		return nil // File exists, don't overwrite
	}

	// Create default config
	defaultConfig := &Config{
		UseNotifications:   true,
		TemporaryClipboard: true,
		AutomaticReversion: false,
		RevertHotkey:       "ctrl+alt+r",
		Profiles: []ProfileConfig{
			{
				Name:    "General Cleanup",
				Enabled: true,
				Hotkey:  "ctrl+alt+v",
				Replacements: []Replacement{
					{
						Regex:       "\\s+",
						ReplaceWith: " ",
					},
				},
			},
		},
	}

	// Convert to JSON
	data, err := json.MarshalIndent(defaultConfig, "", "  ")
	if err != nil {
		return err
	}

	// Write to file
	return ioutil.WriteFile(configPath, data, 0644)
}