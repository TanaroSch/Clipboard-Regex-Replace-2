# Configuration (`config.json`)

Clipboard Regex Replace reads its configuration from an external `config.json` file located in the same directory as the executable.

This file allows you to define global settings, multiple rule profiles (each with their own hotkey), and references to securely stored secrets.

See the main [README.md](README.md) for a general overview and usage instructions.

## Example `config.json` Structure

This example demonstrates two profiles: one for general privacy redaction and another specifically for credentials using secure secret management.

```json
{
  "use_notifications": true,
  "temporary_clipboard": true,
  "automatic_reversion": false,
  "revert_hotkey": "ctrl+shift+alt+r",
  "secrets": {
    "my_api_key": "managed",
    "my_password": "managed"
  },
  "profiles": [
    {
      "name": "Privacy Redaction",
      "enabled": true,
      "hotkey": "ctrl+alt+v",
      "replacements": [
        {
          "regex": "\\b\\(?\\d{3}\\)?[-.\\s]?\\d{3}[-.\\s]?\\d{4}\\b",
          "replace_with": "REDACTED_PHONE"
        },
        {
          "regex": "(?i)(John Doe|Jane Smith)",
          "replace_with": "Redacted Name",
          "preserve_case": true
        }
      ]
    },
    {
      "name": "Credentials Redaction",
      "enabled": true,
      "hotkey": "ctrl+alt+c",
      "replacements": [
        {
          "regex": "{{my_api_key}}",
          "replace_with": "REDACTED_API_KEY"
        },
        {
          "regex": "{{my_password}}",
          "replace_with": "REDACTED_PASSWORD"
        }
      ]
    }
  ]
}
```

## Configuration Options Explained

*   **Global Settings (Top Level):**
    *   `use_notifications` (boolean): Enable/disable desktop notifications (default: `true`).
    *   `temporary_clipboard` (boolean): Store the original clipboard content before processing (default: `true`). Allows reverting.
    *   `automatic_reversion` (boolean): If `temporary_clipboard` is true, automatically revert to the original clipboard content shortly after pasting (default: `false`).
    *   `revert_hotkey` (string): Define a global hotkey (e.g., `"ctrl+shift+alt+r"`) to manually revert the clipboard if `temporary_clipboard` is true and `automatic_reversion` is false.
*   **`secrets` (Object):**
    *   Maps logical secret names (used in `{{...}}` placeholders) to the value `"managed"`. This tells the application to load the actual secret value from the OS keychain/credential store. See [FEATURES.md#secure-secret-management](FEATURES.md#secure-secret-management) for details.
*   **`profiles` (Array):**
    *   Contains one or more profile objects. Each profile defines a set of rules triggered by a specific hotkey. See [FEATURES.md#multiple-profile-support](FEATURES.md#multiple-profile-support) for details.
    *   **Profile Object:**
        *   `name` (string): A descriptive name shown in the system tray menu.
        *   `enabled` (boolean): Whether this profile is active and its hotkeys are registered (can be toggled via systray).
        *   `hotkey` (string): The hotkey combination (e.g., `"ctrl+alt+v"`) that triggers this profile's rules.
        *   `reverse_hotkey` (string, optional): A hotkey to trigger the *reverse* application of the rules in this profile. See [FEATURES.md#case-preserving-and-reversible-replacements](FEATURES.md#case-preserving-and-reversible-replacements).
        *   `replacements` (Array): An array of replacement rule objects.
        *   **Replacement Rule Object:**
            *   `regex` (string): The regular expression pattern to search for. Can contain `{{secret_name}}` placeholders.
            *   `replace_with` (string): The text to replace matches with. Can contain `{{secret_name}}` placeholders.
            *   `preserve_case` (boolean, optional): If `true`, attempt to maintain the capitalization pattern of the matched text during replacement (default: `false`). See [FEATURES.md#case-preserving-and-reversible-replacements](FEATURES.md#case-preserving-and-reversible-replacements).
            *   `reverse_with` (string, optional): Explicitly define the text to use when reversing this rule via the `reverse_hotkey`. If omitted, the app tries to derive it from the first alternative in the `regex`. Can contain `{{secret_name}}` placeholders. See [FEATURES.md#case-preserving-and-reversible-replacements](FEATURES.md#case-preserving-and-reversible-replacements).

> **Important Warning:** Replacements within a profile are processed sequentially in the order they appear in the `replacements` array. This means the order of your regex rules matters! Earlier replacements can affect the text that later replacements operate on.