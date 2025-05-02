# Feature Details

This document provides in-depth explanations of some key features of Clipboard Regex Replace.

See the main [README.md](../README.md) for a general overview and usage instructions.
See [CONFIGURATION.md](CONFIGURATION.md) for details on the `config.json` file format.

## Secure Secret Management

To avoid storing sensitive data (like passwords, API keys, emails) directly in `config.json`, you can use the secure secret management feature.

### How it Works

1.  **Storage:** Secrets are stored securely in your operating system's native credential store (e.g., Windows Credential Manager, macOS Keychain, Linux Secret Service/Keyring) under the service name `Clipboard Regex Replace`.
2.  **Configuration:** In `config.json`, you define a logical name for each secret in the top-level `secrets` map. The value must always be `"managed"`.
    ```json
    "secrets": {
      "my_api_key": "managed",
      "ssh_password": "managed"
    }
    ```
3.  **Usage in Rules:** Reference your secrets within the `regex`, `replace_with`, or `reverse_with` fields of your replacement rules using double curly braces: `{{logical_name}}`. The application will substitute these placeholders with the actual secret values fetched from the OS store at runtime before applying the rule.
    ```json
    {
      "regex": "{{ssh_password}}",        // Use secret as the text to find
      "replace_with": "[SSH_PASSWORD]"
    },
    {
      "regex": "My Contact Email",
      "replace_with": "{{personal_email}}" // Use secret as the replacement text
    }
    ```
4.  **Management:** Add, list, and remove secrets using the "Manage Secrets" submenu in the system tray. This interacts directly with your OS credential store via native dialogs.
5.  **Activation:** **A full manual restart of the application is required after adding or removing secrets** via the systray menu for the application to load the new secret values and make them available for use in replacement rules. Simply reloading the configuration is *not* sufficient for activating new secrets.

### Managing Secrets via System Tray

The "Manage Secrets" submenu in the system tray provides these actions:

*   **Add/Update Secret...**:
    *   Prompts for a "Logical Name" (used in `{{...}}` placeholders, e.g., `my_api_key`).
    *   Prompts for the actual "Secret Value".
    *   Stores the value securely in the OS keychain/credential store under the provided logical name.
    *   Adds the entry (`"logical_name": "managed"`) to the `secrets` map in `config.json`.
    *   Optionally offers to create a basic replacement rule using the new secret (e.g., replace `{{my_api_key}}` with `[REDACTED]`).
    *   **Requires application restart after use.**
*   **List Secret Names**:
    *   Shows a notification (if admin level >= Info) and logs the logical names of all secrets currently declared in the `secrets` map in `config.json`.
    *   It does *not* display the actual secret values.
*   **Remove Secret...**:
    *   Prompts you to select one of the managed logical names from a list.
    *   Asks for confirmation.
    *   Removes the secret entry from the OS keychain/credential store.
    *   Removes the corresponding entry from the `secrets` map in `config.json`.
    *   **This action cannot be undone.**
    *   **Requires application restart after use.**

## Adding Simple Rules via System Tray

For common cases where you just want to replace one specific piece of text with another (without needing complex regex patterns), you can use the "Add Simple Rule..." option in the system tray menu.

### How it Works

1.  **Select Profile:** You'll first be asked via a native dialog to choose which existing profile (defined in your `config.json`) you want to add the rule to.
2.  **Enter Source Text:** Provide the exact text you want to find and replace. Any special characters you enter here will be automatically escaped to be treated literally when converted to a regex pattern (e.g., `.` becomes `\.`).
3.  **Enter Replacement Text:** Provide the text you want to replace the source text with. This can be left empty if you want to simply delete the source text.
4.  **Case Sensitivity:** You'll be asked (Yes/No) if the rule should be case-insensitive.
    *   **Yes:** The rule will match the source text regardless of case (e.g., "source" would match "source", "Source", "SOURCE"). This adds the `(?i)` flag to the beginning of the generated regex.
    *   **No:** The rule will only match the source text with the exact casing you entered.
5.  **Rule Added:** The application constructs the appropriate regex rule (e.g., `(?i)My\ literal\ text` or `My\ literal\ text`) and adds it as a new entry to the end of the selected profile's `replacements` list in your `config.json` file. The `preserve_case` option is automatically set to `false` for these simple rules.
6.  **Reload Config:** After the rule is added and `config.json` is saved, you must use the "Reload Configuration" menu item (or restart the application) for the new rule to become active and its hotkey bindings (if any) to be updated.

This provides a quick, user-friendly way to add basic replacements without manually editing the `config.json` file or worrying about regex syntax for simple cases. For more complex patterns involving groups, lookarounds, or character classes, you'll still need to edit the configuration file directly.

## Multiple Profile Support

Clipboard Regex Replace allows you to organize your replacement rules into multiple distinct profiles. This is useful for grouping related rules or having different sets of transformations for different tasks, each triggered by its own hotkey.

### Features

*   **Independent Profiles**: Create multiple named sets of replacement rules within the `profiles` array in `config.json`.
*   **Per-Profile Hotkeys**: Assign different hotkeys (`hotkey` and optional `reverse_hotkey`) to each profile.
*   **Dynamic Profile Toggling**: Enable or disable entire profiles on-the-fly using the "Profiles" submenu in the system tray. Checkmarks indicate enabled profiles. Toggling requires a configuration reload (triggered automatically by the menu action) to update hotkey registrations.
*   **Rule Merging for Shared Hotkeys**: If multiple *enabled* profiles share the *same* `hotkey` (or `reverse_hotkey`), all their `replacements` will be executed sequentially in the order the profiles appear in the `config.json` file when that hotkey is pressed.
*   **Backward Compatibility**: If you load a `config.json` from a version prior to v1.4.0 (which didn't have profiles), it will be automatically migrated. Your existing rules and hotkey will be placed into a single profile named "Default".

### How to Configure Multiple Profiles

Edit your `config.json` file. Instead of defining `hotkey` and `replacements` at the top level, define them within objects inside the `profiles` array. See the example in [CONFIGURATION.md](CONFIGURATION.md).

### Profile Options

See the `Profile Object` description in [CONFIGURATION.md#configuration-options-explained](CONFIGURATION.md#configuration-options-explained).

### Using Profiles

1.  **Triggering Specific Profiles**: Press the `hotkey` assigned to an enabled profile to execute its replacements.
2.  **Toggling Profiles**: Right-click the systray icon, go to the "Profiles" submenu, and click on a profile name to toggle its `enabled` state (✓ = enabled). This automatically saves the config and triggers a reload.
3.  **Adding New Profiles**: Use the "➕ Add New Profile" option in the "Profiles" submenu. This adds a basic template profile to your `config.json`. You'll then need to edit the file manually (using "Open Config File") to customize the name, hotkey, and rules, followed by a "Reload Configuration" or "Restart Application".
4.  **Bidirectional Replacements**: If a profile has a `reverse_hotkey` defined, pressing that key will attempt to reverse the replacements defined in that profile.

### Migration from Previous Versions

When upgrading from a version before v1.4.0, your existing configuration file will be automatically backed up (as `config.json.bak`) and then converted to the new format. Your existing replacement rules will be placed in a profile named "Default", retaining your original hotkey configuration.

## Case-Preserving and Reversible Replacements

Clipboard Regex Replace offers advanced options for controlling the case of replaced text and for reversing replacements. Secret placeholders `{{...}}` work seamlessly with these features.

### Case Preservation

By setting `"preserve_case": true` on a replacement rule, the application attempts to maintain the capitalization pattern of the text matched by the `regex` when inserting the `replace_with` text.

**Example:**

```json
{
  "regex": "(?i)(johndoe)", // Case-insensitive match
  "replace_with": "GithubUser",
  "preserve_case": true
}
```

**Behavior:**

*   Input: `johndoe` → Output: `githubuser` (all lowercase preserved)
*   Input: `JOHNDOE` → Output: `GITHUBUSER` (all uppercase preserved)
*   Input: `JohnDoe` → Output: `GithubUser` (PascalCase/TitleCase preserved)
*   Input: `Johndoe` → Output: `Githubuser` (First letter capitalized preserved)

The detection tries to identify common patterns (all lower, all upper, title case, first upper). If none match clearly, it typically defaults to matching the case of the first letter of the source match.

### Bidirectional Replacements

You can make a profile's replacements reversible by adding a `reverse_hotkey`. When this hotkey is pressed, the application attempts to find occurrences of the `replace_with` text and change them back to the *original* text derived from the `regex`.

**Example:**

```json
{
  "name": "Privacy - Bidirectional",
  "enabled": true,
  "hotkey": "ctrl+alt+v",
  "reverse_hotkey": "shift+alt+v",
  "replacements": [
    {
      // Match different variations of a name (using secrets and literal text)
      "regex": "(?i)({{real_name_alt1}}|{{real_name_alt2}}|John)",
      "replace_with": "{{github_user}}", // Replace with a username secret
      "preserve_case": true
    }
    // ... other rules in the profile ...
  ]
}
```

Assume the secrets resolve as:
`{{real_name_alt1}}` → `JohnDoe_T`
`{{real_name_alt2}}` → `JohnDoe`
`{{github_user}}` → `GithubUser`

**Behavior:**

*   **Press `ctrl+alt+v` (Forward):**
    *   Finds `JohnDoe_T`, `JohnDoe`, or `John` (case-insensitively).
    *   Replaces them with `GithubUser` (preserving case). E.g., `JohnDoe` becomes `GithubUser`.
*   **Press `shift+alt+v` (Reverse):**
    *   Finds `GithubUser` (case-insensitively if `preserve_case` was true).
    *   Replaces it back to the *first* pattern listed in the original regex's alternatives that resolved successfully, which is `JohnDoe_T` in this case (preserving case). E.g., `GithubUser` becomes `JohnDoe_T`.

### Custom Reverse Replacements (`reverse_with`)

By default, the reverse operation uses the first alternative from the forward `regex`. You can explicitly control the text used for reversal by adding the `reverse_with` field to a rule. Placeholders are supported here too.

**Example:**

```json
{
  "regex": "(?i)({{real_name_alt1}}|{{real_name_alt2}}|John)",
  "replace_with": "{{github_user}}",
  "preserve_case": true,
  // Explicitly reverse back to the value of the 'real_name_alt2' secret
  "reverse_with": "{{real_name_alt2}}"
}
```

**Behavior (Reverse - `shift+alt+v`):**

*   Finds `GithubUser` (case-insensitively).
*   Replaces it with the resolved value of `{{real_name_alt2}}`, which is `JohnDoe` (preserving case). E.g., `GithubUser` becomes `JohnDoe`.

This allows precise control over bidirectional mappings, especially when the forward `regex` contains multiple patterns or complex structures.