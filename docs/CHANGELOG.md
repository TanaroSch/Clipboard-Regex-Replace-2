## Changelog

### 1.7.3 (Current Version)

*   **Enhancement: Diff Viewer Improvements:**
    *   Added line numbers (original and modified) to the HTML diff view for easier navigation.
    *   Implemented context folding: Large blocks of unchanged text (more than 6 lines by default) are now collapsed into a "N lines hidden" marker to focus the view on actual modifications.
    *   Improved overall styling of the diff view for better readability.

### 1.7.2

*   **Feature: Granular Notification Control:**
    *   Replaced the single `use_notifications` setting with two more specific settings in `config.json`:
        *   `admin_notification_level` (string): Controls the verbosity of notifications for administrative actions (config reloads, errors, secret management, etc.). Valid levels:
            *   `"None"`: No admin notifications.
            *   `"Error"`: Only critical errors.
            *   `"Warn"`: Errors and warnings (Default).
            *   `"Info"`: Errors, warnings, and informational confirmations.
        *   `notify_on_replacement` (boolean): Specifically toggles the notification that appears after a successful clipboard replacement via hotkey (Default: `true` for new configs).
    *   **BREAKING CHANGE:** Users upgrading from v1.7.1 or earlier **must** update their `config.json`. The old `use_notifications` field is ignored. To restore previous behavior:
        *   Add `"admin_notification_level": "Warn"` (or desired level).
        *   Add `"notify_on_replacement": true`.
        If these fields are missing, admin notifications will default to "Warn", but **replacement notifications will be disabled** until `"notify_on_replacement": true` is added.

### 1.7.1

*   **Feature: Add Simple Rule:**
    *   Added "Add Simple Rule..." option to the system tray menu.
    *   Uses native dialogs (`zenity`) to prompt for profile selection, source text, replacement text, and case sensitivity.
    *   Automatically creates a 1:1 replacement rule (escaping source text for regex) in the selected profile and saves `config.json`.

### 1.7.0

*   **Feature: Secure Secret Management:**
    *   Store sensitive replacement values securely in the OS native credential store (keychain/credential manager).
    *   Added "Manage Secrets" submenu to system tray for adding, listing, and removing secrets via native dialogs (`zenity`).
    *   Introduced `secrets` map in `config.json` to declare managed secrets.
    *   Use `{{secret_name}}` placeholders in `regex`, `replace_with`, and `reverse_with` fields.
    *   **Note:** Requires manual application restart after adding/removing secrets.
*   **Dependency:** Added `github.com/99designs/keyring` and `github.com/ncruces/zenity`.
*   **Config:** Changed default file permissions for `config.json` to `0600` (owner read/write only). Added default config creation on first run if missing.

### 1.5.4

*   **Feature: Change Details Viewer:** Added a "View Last Change Details" option to the system tray menu. When clicked after a replacement, it opens an HTML report in the browser showing a summary and detailed diff of the changes.
*   **Refactor:** Improved diff generation logic for better accuracy and readability in HTML output.
*   **Fix:** Resolved issues with opening configuration file and diff report on Windows by using the ShellExecuteW API directly within the UI package.
*   **Refactor:** Moved platform-specific code (paste simulation, file opening) to dedicated files within relevant packages (`clipboard`, `ui`) using build tags.

### 1.5.3

*   **Open Configuration File:**
    *   Add option in system tray to quickly open ```config.json``` in the default text editor

### 1.5.2

*   **Major Code Refactoring**:
    *   Reorganized project into a proper Go package structure
    *   Improved platform-specific clipboard paste handling
    *   Enhanced error handling and logging
    *   Better separation of concerns between packages
    *   No functional changes, purely architectural improvements

### 1.5.1

*   **Global Revert Hotkey:**
    Added support for a dedicated global hotkey that reverts the clipboard to its original content when automatic reversion is disabled.

### 1.5.0

*   **Case-Preserving Replacements:**
    Added support for maintaining capitalization patterns when replacing text (lowercase, UPPERCASE, Title Case, PascalCase).
*   **Bidirectional Replacements:**
    Added `reverse_hotkey` to profiles for enabling reversible replacements.
*   **Custom Reverse Replacements:**
    Added `reverse_with` field to override the default text used in reverse replacements.

### 1.4.0

*   **Multiple Profile Support:**
    Added support for multiple named profiles, each with its own set of replacement rules and hotkey binding.
*   **Profile Management:**
    Profiles can be toggled on/off directly from the system tray.
*   **Rule Merging:**
    Profiles with the same hotkey have their replacement rules merged and applied sequentially.

### 1.3.1

*   **Fixed Original Clipboard Storage:**
    Fixed an issue where pressing the hotkey multiple times on already processed text would incorrectly overwrite the stored original clipboard content. The application now properly preserves the original clipboard text until either new content is copied or new replacements are performed.

### 1.3.0

*   **Dynamic Configuration Reloading:**
    Added ability to reload configuration without restarting the application.
*   **Automatic Clipboard Reversion:**
    Added option to automatically restore the original clipboard content immediately after pasting.
*   **Simplified Clipboard Management:**
    Streamlined the clipboard restoration interface to a single "Revert to Original" option in the system tray.

### 1.2.0
*   **Temporary Clipboard Storage:**
    Optionally store the original clipboard text before applying regex replacements. The replaced clipboard is pasted, and the original text is automatically restored after 10 seconds unless the user chooses to keep the replaced text.
*   **Interactive Options:**
    Added system tray menu items (and toast notification prompts on Windows) to allow users to revert to the original clipboard text or keep the replaced text.

### 1.1.0
*   Custom hotkey configuration.

### 1.0.0
*   Initial project.
*   Basic regex replacement.
*   Toast notification.

## License

This project is licensed under the MIT License. See the [../LICENSE](../LICENSE) file for details.