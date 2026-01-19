package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings" // Needed for level comparison

	"github.com/99designs/keyring"
)

// ProfileConfig represents a single regex replacement profile
type ProfileConfig struct {
	Name          string        `json:"name"`
	Enabled       bool          `json:"enabled"`
	Hotkey        string        `json:"hotkey"`
	ReverseHotkey string        `json:"reverse_hotkey,omitempty"`
	Replacements  []Replacement `json:"replacements"`
}

// Config holds the application configuration
type Config struct {
	// UseNotifications   bool              `json:"use_notifications"` // DEPRECATED: Use new fields below
	AdminNotificationLevel string            `json:"admin_notification_level"` // NEW: Controls verbosity ("None", "Error", "Warn", "Info")
	NotifyOnReplacement    bool              `json:"notify_on_replacement"`    // NEW: Toggle for replacement success notifications
	TemporaryClipboard     bool              `json:"temporary_clipboard"`
	AutomaticReversion     bool              `json:"automatic_reversion"`
	RevertHotkey           string            `json:"revert_hotkey"`
	Profiles               []ProfileConfig   `json:"profiles"`
	Secrets                map[string]string `json:"secrets,omitempty"` // Maps logical name -> "managed"

	// Performance and behavior settings
	PasteDelayMs          int `json:"paste_delay_ms,omitempty"`           // Delay before pasting (default: 400ms)
	RevertDelayMs         int `json:"revert_delay_ms,omitempty"`          // Delay before reverting (default: 300ms)
	RegexTimeoutMs        int `json:"regex_timeout_ms,omitempty"`         // Timeout for regex operations (default: 5000ms)
	DiffContextLines      int `json:"diff_context_lines,omitempty"`       // Context lines in diff viewer (default: 3)

	// Legacy support fields (for backward compatibility)
	Hotkey       string        `json:"hotkey,omitempty"`
	Replacements []Replacement `json:"replacements,omitempty"`

	// Non-JSON fields (runtime state)
	configPath      string
	keyringService  string            // e.g., "Clipboard Regex Replace"
	resolvedSecrets map[string]string // Runtime map {"logicalName": "actualValue"}
}

// Replacement represents one regex replacement rule
type Replacement struct {
	Regex        string `json:"regex"`
	ReplaceWith  string `json:"replace_with"`
	PreserveCase bool   `json:"preserve_case,omitempty"`
	ReverseWith  string `json:"reverse_with,omitempty"`
}

const DefaultKeyringService = "Clipboard Regex Replace" // Define AppName constant
const DefaultAdminNotificationLevel = "Warn"            // Define default level constant
const DefaultPasteDelayMs = 400                         // Default delay before pasting
const DefaultRevertDelayMs = 300                        // Default delay before reverting
const DefaultRegexTimeoutMs = 5000                      // Default regex timeout (5 seconds)
const DefaultDiffContextLines = 3                       // Default context lines in diff viewer

// GetConfigPath returns the path to the configuration file
func (c *Config) GetConfigPath() string {
	return c.configPath
}

// GetResolvedSecrets returns the map of loaded secrets.
func (c *Config) GetResolvedSecrets() map[string]string {
	if c.resolvedSecrets == nil {
		return make(map[string]string) // Return empty map if nil
	}
	return c.resolvedSecrets
}

// GetPasteDelay returns the configured paste delay or default if not set
func (c *Config) GetPasteDelay() int {
	if c.PasteDelayMs <= 0 {
		return DefaultPasteDelayMs
	}
	return c.PasteDelayMs
}

// GetRevertDelay returns the configured revert delay or default if not set
func (c *Config) GetRevertDelay() int {
	if c.RevertDelayMs <= 0 {
		return DefaultRevertDelayMs
	}
	return c.RevertDelayMs
}

// GetRegexTimeout returns the configured regex timeout or default if not set
func (c *Config) GetRegexTimeout() int {
	if c.RegexTimeoutMs <= 0 {
		return DefaultRegexTimeoutMs
	}
	return c.RegexTimeoutMs
}

// GetDiffContextLines returns the configured diff context lines or default if not set
func (c *Config) GetDiffContextLines() int {
	if c.DiffContextLines <= 0 {
		return DefaultDiffContextLines
	}
	return c.DiffContextLines
}

// Load reads and parses the configuration file with backward compatibility and loads secrets
func Load(configPath string) (*Config, error) {
	var config Config

	data, err := os.ReadFile(configPath)
	if err != nil {
		// If file not found, try creating default first, then re-read or return error if creation fails
		if os.IsNotExist(err) {
			log.Printf("Config file '%s' not found. Attempting to create default.", configPath)
			if createErr := CreateDefaultConfig(configPath); createErr != nil {
				return nil, fmt.Errorf("config file not found and failed to create default '%s': %w", configPath, createErr)
			}
			// Retry reading after creation
			data, err = os.ReadFile(configPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read config file '%s' even after creating default: %w", configPath, err)
			}
		} else {
			// Other read error
			return nil, fmt.Errorf("failed to read config file '%s': %w", configPath, err)
		}
	}

	// First unmarshal into the new structure
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file '%s': %w", configPath, err)
	}

	// Set default notification level if missing or empty after load
	if strings.TrimSpace(config.AdminNotificationLevel) == "" {
		log.Printf("AdminNotificationLevel not found or empty in config, setting default to '%s'", DefaultAdminNotificationLevel)
		config.AdminNotificationLevel = DefaultAdminNotificationLevel
		// Note: NotifyOnReplacement defaults to `false` (its zero value) if missing.
		// This requires users to explicitly add `"notify_on_replacement": true`
		// to their config if upgrading to re-enable replacement notifications.
	}

	// Store config path for future saves
	config.configPath = configPath
	config.keyringService = DefaultKeyringService // Assign keyring service name

	// --- Load Secrets ---
	config.resolvedSecrets = make(map[string]string)
	if config.Secrets != nil && len(config.Secrets) > 0 { // Check if map exists and is not empty
		log.Printf("Loading secrets from keyring for service '%s'...", config.keyringService)
		allowedBackends := []keyring.BackendType{ // Explicitly allow backends (optional but good practice)
			keyring.KeychainBackend,
			keyring.SecretServiceBackend,
			keyring.WinCredBackend,
			// keyring.KWalletBackend, // Enable if needed
			// keyring.PassBackend, // Enable if needed
		}
		kr, err := keyring.Open(keyring.Config{
			ServiceName:              config.keyringService,
			AllowedBackends:          allowedBackends,
			LibSecretCollectionName:  "login",               // Common on Linux, adjust if needed
			PassDir:                  "",                    // Path to pass directory if using PassBackend
			PassCmd:                  "",                    // Path to pass command if using PassBackend
			PassPrefix:               config.keyringService, // Prefix for pass entries
			WinCredPrefix:            config.keyringService, // Prefix for Windows Credential Manager entries
			KeychainName:             "",                    // Specific keychain name on macOS (usually empty)
			KeychainTrustApplication: true,                  // Allow access without prompt if app is trusted
			// KWalletAppID:             config.keyringService, // Set if using KWallet
			// KWalletFolder:            "", // Set if using KWallet
		})

		if err != nil {
			log.Printf("Warning: Failed to open keyring for service '%s': %v. Secrets will not be loaded.", config.keyringService, err)
			// Continue without secrets? Or return error? For now, continue with warning.
		} else {
			for name := range config.Secrets { // We only need the name from config
				item, err := kr.Get(name) // Get the Item struct
				if err == nil {
					config.resolvedSecrets[name] = string(item.Data) // Convert []byte to string
					log.Printf("Successfully loaded secret '%s'.", name)
				} else if err == keyring.ErrKeyNotFound {
					log.Printf("Warning: Secret '%s' not found in keychain for service '%s'. Rules using it may fail.", name, config.keyringService)
				} else {
					log.Printf("Error retrieving secret '%s' from keychain: %v", name, err)
					// Potentially return error here? Or just warn? Warn for now.
				}
			}
		}
	} else {
		log.Println("No secrets defined in config.json, skipping keyring load.")
	}
	// --- End Load Secrets ---

	// --- Validate Configuration ---
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}
	// --- End Validate Configuration ---

	// Handle backward compatibility - migrate from legacy format to profiles
	if config.Hotkey != "" && len(config.Replacements) > 0 && len(config.Profiles) == 0 {
		log.Println("Migrating legacy config format to profiles...")
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
		if err := config.Save(); err != nil { // Check error on save
			log.Printf("Warning: Failed to save migrated config: %v", err)
		} else {
			log.Println("Successfully saved migrated config.")
		}
	}

	return &config, nil
}

// Save writes the current configuration back to the config.json file
func (c *Config) Save() error {
	// Ensure Secrets map exists even if empty for consistent JSON output
	if c.Secrets == nil {
		c.Secrets = make(map[string]string)
	}

	// Ensure default notification level if empty before saving
	if strings.TrimSpace(c.AdminNotificationLevel) == "" {
		c.AdminNotificationLevel = DefaultAdminNotificationLevel
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	// Use 0600 permissions for potentially sensitive config file?
	// 0644 is readable by everyone, 0600 is only owner. Let's use 0600.
	return os.WriteFile(c.configPath, data, 0600)
}

// AddSecretReference adds/updates a secret reference in config and stores the value in keyring
func (c *Config) AddSecretReference(name, value string) error {
	// Use specific config matching Load for keyring access
	kr, err := keyring.Open(keyring.Config{
		ServiceName:              c.keyringService,
		LibSecretCollectionName:  "login",
		PassPrefix:               c.keyringService,
		WinCredPrefix:            c.keyringService,
		KeychainTrustApplication: true,
	})
	if err != nil {
		return fmt.Errorf("failed to open keyring for service '%s': %w", c.keyringService, err)
	}

	err = kr.Set(keyring.Item{
		Key:         name, // Use logical name as the Key/Username
		Data:        []byte(value),
		Label:       fmt.Sprintf("Secret for %s used by %s", name, c.keyringService),
		Description: "Managed by Clipboard Regex Replace", // Updated description
	})
	if err != nil {
		return fmt.Errorf("failed to store secret '%s' in keyring: %w", name, err)
	}

	if c.Secrets == nil {
		c.Secrets = make(map[string]string)
	}
	c.Secrets[name] = "managed" // Mark as managed in config

	// Only save the updated config (with the "managed" entry)
	// The actual secret value will be loaded during the next config reload.
	return c.Save()
}

// RemoveSecretReference removes a secret from config and keyring
func (c *Config) RemoveSecretReference(name string) error {
	// Use specific config matching Load for keyring access
	kr, err := keyring.Open(keyring.Config{
		ServiceName:              c.keyringService,
		LibSecretCollectionName:  "login",
		PassPrefix:               c.keyringService,
		WinCredPrefix:            c.keyringService,
		KeychainTrustApplication: true,
	})
	if err != nil {
		return fmt.Errorf("failed to open keyring for service '%s': %w", c.keyringService, err)
	}

	err = kr.Remove(name)
	// Log warning if not found, but proceed to remove from config anyway
	if err != nil && err != keyring.ErrKeyNotFound {
		log.Printf("Warning: Failed to delete secret '%s' from keyring (it might not exist or access denied): %v", name, err)
	} else if err == nil {
		log.Printf("Successfully deleted secret '%s' from keyring.", name)
	} else if err == keyring.ErrKeyNotFound {
		log.Printf("Secret '%s' was not found in keyring (already deleted or never existed). Removing from config.", name)
	}

	if c.Secrets != nil {
		delete(c.Secrets, name)
	}

	// Save the config with the secret reference removed
	return c.Save()
}

// GetSecretNames returns a slice of logical names of managed secrets.
func (c *Config) GetSecretNames() []string {
	names := make([]string, 0, len(c.Secrets))
	if c.Secrets != nil {
		for name := range c.Secrets {
			names = append(names, name)
		}
	}
	// Consider sorting the names alphabetically for consistent display
	// sort.Strings(names)
	return names
}

// CreateDefaultConfig creates a default configuration file if none exists
func CreateDefaultConfig(configPath string) error {
	// Check if file already exists
	if _, err := os.Stat(configPath); err == nil {
		return nil // File exists, don't overwrite
	} else if !os.IsNotExist(err) {
		// Different error accessing path?
		return fmt.Errorf("error checking config path '%s': %w", configPath, err)
	}

	log.Printf("Creating default configuration file at: %s", configPath)

	// Create default config
	defaultConfig := &Config{
		// UseNotifications:   true, // DEPRECATED
		AdminNotificationLevel: DefaultAdminNotificationLevel, // NEW Default
		NotifyOnReplacement:    true,                          // NEW Default
		TemporaryClipboard:     true,
		AutomaticReversion:     false,
		RevertHotkey:           "ctrl+shift+alt+r",      // Changed default revert hotkey
		Secrets:                make(map[string]string), // Initialize empty secrets map

		// Performance settings (omitted fields will use defaults)
		PasteDelayMs:     DefaultPasteDelayMs,     // 400ms - delay before pasting
		RevertDelayMs:    DefaultRevertDelayMs,    // 300ms - delay before reverting
		RegexTimeoutMs:   DefaultRegexTimeoutMs,   // 5000ms - timeout for regex operations
		DiffContextLines: DefaultDiffContextLines, // 3 - context lines in diff viewer

		Profiles: []ProfileConfig{
			{
				Name:    "General Cleanup",
				Enabled: true,
				Hotkey:  "ctrl+alt+v",
				Replacements: []Replacement{
					{
						Regex:       "\\s+", // Example: Trim extra whitespace
						ReplaceWith: " ",
					},
				},
			},
			// Add an example using a secret placeholder
			{
				Name:    "Example Secret Redaction",
				Enabled: false, // Disabled by default
				Hotkey:  "ctrl+alt+s",
				Replacements: []Replacement{
					{
						Regex:       "{{my_secret_placeholder}}", // User needs to add 'my_secret_placeholder' via Manage Secrets
						ReplaceWith: "[REDACTED_SECRET]",
					},
				},
			},
		},
	}

	// Convert to JSON
	data, err := json.MarshalIndent(defaultConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal default config to JSON: %w", err)
	}

	// Write to file using more restrictive permissions
	err = os.WriteFile(configPath, data, 0600) // Use 0600 permissions
	if err != nil {
		return fmt.Errorf("failed to write default config file '%s': %w", configPath, err)
	}

	log.Printf("Default configuration file created successfully.")
	return nil
}

// validateConfig validates the configuration for common errors
func validateConfig(cfg *Config) error {
	var validationErrors []string

	// Validate admin notification level
	validLevels := map[string]bool{"None": true, "Error": true, "Warn": true, "Info": true}
	if !validLevels[cfg.AdminNotificationLevel] {
		validationErrors = append(validationErrors, fmt.Sprintf("invalid AdminNotificationLevel '%s' (must be None, Error, Warn, or Info)", cfg.AdminNotificationLevel))
	}

	// Validate profiles
	if cfg.Profiles != nil {
		profileNames := make(map[string]bool)
		profileHotkeys := make(map[string][]string) // Track which profiles use which hotkeys

		for i, profile := range cfg.Profiles {
			profilePrefix := fmt.Sprintf("Profile[%d](%s)", i, profile.Name)

			// Check for empty profile name
			if strings.TrimSpace(profile.Name) == "" {
				validationErrors = append(validationErrors, fmt.Sprintf("%s: profile name cannot be empty", profilePrefix))
			}

			// Check for duplicate profile names
			if profileNames[profile.Name] {
				validationErrors = append(validationErrors, fmt.Sprintf("%s: duplicate profile name", profilePrefix))
			}
			profileNames[profile.Name] = true

			// Check for empty hotkey
			if strings.TrimSpace(profile.Hotkey) == "" {
				validationErrors = append(validationErrors, fmt.Sprintf("%s: hotkey cannot be empty", profilePrefix))
			} else {
				// Track hotkey usage
				profileHotkeys[profile.Hotkey] = append(profileHotkeys[profile.Hotkey], profile.Name)
			}

			// Validate regex patterns in replacements
			for j, replacement := range profile.Replacements {
				rulePrefix := fmt.Sprintf("%s.Replacement[%d]", profilePrefix, j)

				// Validate regex pattern
				if replacement.Regex != "" {
					// Try to compile the regex (without resolving placeholders)
					_, err := regexp.Compile(replacement.Regex)
					if err != nil {
						validationErrors = append(validationErrors, fmt.Sprintf("%s: invalid regex '%s': %v", rulePrefix, replacement.Regex, err))
					}
				}

				// Validate reverse_with if present
				if replacement.ReverseWith != "" {
					// Check if it's a valid regex (if used as regex in reverse mode)
					_, err := regexp.Compile(replacement.ReverseWith)
					if err != nil {
						// It's okay if reverse_with is not a valid regex (it might be a literal string)
						log.Printf("Note: %s.reverse_with '%s' is not a valid regex pattern (will be treated as literal): %v", rulePrefix, replacement.ReverseWith, err)
					}
				}
			}
		}

		// Warn about duplicate hotkeys (not an error, just a warning)
		for hotkey, profiles := range profileHotkeys {
			if len(profiles) > 1 {
				log.Printf("Warning: Hotkey '%s' is used by multiple profiles: %v. All matching profiles will be triggered.", hotkey, profiles)
			}
		}
	}

	// Return aggregated errors
	if len(validationErrors) > 0 {
		return fmt.Errorf("configuration validation errors:\n  - %s", strings.Join(validationErrors, "\n  - "))
	}

	log.Println("Configuration validation passed.")
	return nil
}