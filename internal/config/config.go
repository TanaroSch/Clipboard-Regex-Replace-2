package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"

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
	UseNotifications   bool              `json:"use_notifications"`
	TemporaryClipboard bool              `json:"temporary_clipboard"`
	AutomaticReversion bool              `json:"automatic_reversion"`
	RevertHotkey       string            `json:"revert_hotkey"`
	Profiles           []ProfileConfig   `json:"profiles"`
	Secrets            map[string]string `json:"secrets,omitempty"` // Maps logical name -> "managed"

	// Legacy support fields (for backward compatibility)
	Hotkey       string        `json:"hotkey,omitempty"`
	Replacements []Replacement `json:"replacements,omitempty"`

	// Non-JSON fields (runtime state)
	configPath      string
	keyringService  string            // e.g., "LLMClipboardFilter2"
	resolvedSecrets map[string]string // Runtime map {"logicalName": "actualValue"}
}

// Replacement represents one regex replacement rule
type Replacement struct {
	Regex        string `json:"regex"`
	ReplaceWith  string `json:"replace_with"`
	PreserveCase bool   `json:"preserve_case,omitempty"`
	ReverseWith  string `json:"reverse_with,omitempty"`
}

const DefaultKeyringService = "LLMClipboardFilter2" // Define AppName constant

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

// Load reads and parses the configuration file with backward compatibility and loads secrets
func Load(configPath string) (*Config, error) {
	var config Config

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		// If file not found, try creating default first, then re-read or return error if creation fails
		if os.IsNotExist(err) {
			log.Printf("Config file '%s' not found. Attempting to create default.", configPath)
			if createErr := CreateDefaultConfig(configPath); createErr != nil {
				return nil, fmt.Errorf("config file not found and failed to create default '%s': %w", configPath, createErr)
			}
			// Retry reading after creation
			data, err = ioutil.ReadFile(configPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read config file '%s' even after creating default: %w", configPath, err)
			}
		} else {
			// Other read error
			return nil, fmt.Errorf("failed to read config file '%s': %w", configPath, err)
		}
	}

	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file '%s': %w", configPath, err)
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
				// *** FIX IS HERE ***
				item, err := kr.Get(name) // Get the Item struct
				if err == nil {
					config.resolvedSecrets[name] = string(item.Data) // Convert []byte to string
					log.Printf("Successfully loaded secret '%s'.", name)
					// *** END FIX ***
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

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	// Use 0600 permissions for potentially sensitive config file?
	// 0644 is readable by everyone, 0600 is only owner. Let's use 0600.
	return ioutil.WriteFile(c.configPath, data, 0600)
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
		Description: "Managed by LLMClipboardFilter2",
	})
	if err != nil {
		return fmt.Errorf("failed to store secret '%s' in keyring: %w", name, err)
	}

	if c.Secrets == nil {
		c.Secrets = make(map[string]string)
	}
	c.Secrets[name] = "managed" // Mark as managed in config

	// ---- REMOVED THIS LINE ----
	// // Update runtime map immediately
	// if c.resolvedSecrets == nil {
	// 	c.resolvedSecrets = make(map[string]string)
	// }
	// c.resolvedSecrets[name] = value
	// ---- END REMOVED BLOCK ----

	// Only save the updated config (with the "managed" entry)
	// The actual secret value will be loaded during the next config reload.
	return c.Save()
}

// RemoveSecretReference also needs the same correction (remove direct manipulation of resolvedSecrets)
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
	// ---- REMOVED THIS BLOCK ----
	// // Remove from runtime map too
	// if c.resolvedSecrets != nil {
	// 	delete(c.resolvedSecrets, name)
	// }
	// ---- END REMOVED BLOCK ----

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
		UseNotifications:   true,
		TemporaryClipboard: true,
		AutomaticReversion: false,
		RevertHotkey:       "ctrl+shift+alt+r",      // Changed default revert hotkey
		Secrets:            make(map[string]string), // Initialize empty secrets map
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
	err = ioutil.WriteFile(configPath, data, 0600) // Use 0600 permissions
	if err != nil {
		return fmt.Errorf("failed to write default config file '%s': %w", configPath, err)
	}

	log.Printf("Default configuration file created successfully.")
	return nil
}
