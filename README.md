# Clipboard Regex Replace

Clipboard Regex Replace is a fast, standalone clipboard filtering application written in Go. It automatically applies a series of regex-based replacements to your clipboard text when you press a global hotkey, then updates your clipboard and simulates a paste action. Additionally, it provides Windows toast notifications, a system tray icon for easy management, a detailed diff view for analyzing changes, and secure storage for sensitive replacement values.

> **Note:** This implementation is a major upgrade compared to the initial Python implementation in [Clipboard-Regex-Replace](https://github.com/TanaroSch/Cliboard-Regex-Replace). It's designed to be lightweight, efficient, and easy to distribute as a single executable (with only external configuration).

## Features

- **Global Hotkey Trigger:**
  Press a configurable hotkey (default: `Ctrl+Alt+V`) to process the clipboard text.

- **Regex-based Filtering:**
  Define multiple regex replacement rules in an external configuration file (`config.json`).

- **Secure Secret Management:**
  Store sensitive values (like passwords or API keys) securely in your OS's native credential store (Windows Credential Manager, macOS Keychain, Linux Secret Service/Keyring) instead of plain text in `config.json`. Manage secrets directly via the system tray menu.

- **Secret Placeholders:**
  Reference stored secrets within your regex replacement rules using `{{secret_name}}` syntax.

- **Clipboard Automation:**
  Automatically updates your clipboard content and simulates a paste.

- **Multiple Profile Support:**
  Create and manage multiple sets of replacement rules with different hotkeys.

- **Case-Preserving Replacements:**
  Maintain capitalization patterns when replacing text (e.g., lowercase, UPPERCASE, Title Case, PascalCase).

- **Bidirectional Replacements:**
  Configure reverse hotkeys to switch back from replaced text to original text (supports secret placeholders).

- **Temporary Clipboard Storage:**
  Optionally store the original clipboard text before processing. You can choose to automatically revert to the original clipboard content after pasting or manually revert using the system tray menu.

- **Global Revert Hotkey:**
  Configure a dedicated hotkey to quickly revert to the original clipboard content when automatic reversion is disabled.

- **Dynamic Configuration Reloading:**
  Reload configuration changes *except* for newly added/removed secrets without restarting the application using the system tray menu. (**Note:** Activating new secrets requires a manual application restart.)

- **Open Configuration File:**
  Quickly open your `config.json` file in the default text editor directly from the system tray menu.

- **Windows Toast Notifications:**
  Displays a toast notification to show successful replacement and configuration changes. The notification now prompts to view changes via the systray.

- **System Tray Icon:**
  Runs in the background with a system tray icon and provides a menu for quick actions like managing secrets, opening the configuration file, reloading configuration, viewing last changes, reverting clipboard, and exiting the application.

- **Change Details Viewer:**
  After a replacement occurs, view a detailed HTML report showing a summary of changes and a line-by-line diff of the original versus modified text, opened in your default web browser.

- **Standalone Executable:**
  Easily build and distribute a single EXE file on Windows (with external configuration files).

## Requirements

- [Go 1.16+](https://golang.org/dl/)
- A Windows, macOS, or Linux machine with a supported native credential store for the Secure Secret Management feature.
  - Windows: Credential Manager
  - macOS: Keychain Access
  - Linux: GNOME Keyring, KWallet, or other service implementing the Secret Service API.
- For building: A suitable build environment for your target OS.

## Project Structure

The project now follows a modern Go project structure:

```
clipboard-regex-replace/
├── cmd/
│   └── clipregex/          # Main entry point
├── internal/               # Internal packages
│   ├── app/                # Application core
│   ├── clipboard/          # Clipboard handling
│   ├── config/             # Configuration
│   ├── diffutil/           # Diff generation utilities
│   ├── hotkey/             # Hotkey management
│   ├── resources/          # Embedded resources
│   └── ui/                 # User interface
├── dist/                   # Distribution builds
├── assets/                 # External assets
├── go.mod
├── go.sum
├── config.json.example
├── icon.png                # External icon for notifications
└── README.md
```

## Installation

1.  **Clone the Repository:**

    ```bash
    git clone https://github.com/TanaroSch/Clipboard-Regex-Replace-2.git
    cd Clipboard-Regex-Replace-2
    ```

2.  **Download Dependencies:**

    The repository uses Go modules. The required dependencies will be fetched automatically when you build or run the project.

    ```bash
    go mod tidy
    ```

## Configuration

The application reads its configuration from an external `config.json` file located in the same directory as the executable.

The configuration supports global settings, multiple profiles, and secure secret management.

### Example `config.json` Structure:

```json
{
  "use_notifications": true,
  "temporary_clipboard": true,
  "automatic_reversion": false,
  "revert_hotkey": "ctrl+shift+alt+r",
  "secrets": {
    "my_api_key": "managed",
    "personal_email": "managed"
  },
  "profiles": [
    {
      "name": "General Cleanup",
      "enabled": true,
      "hotkey": "ctrl+alt+v",
      "replacements": [
        {
          "regex": "\\s+",
          "replace_with": " "
        }
      ]
    },
    {
      "name": "API Key Redaction",
      "enabled": true,
      "hotkey": "ctrl+alt+1",
      "replacements": [
        {
          "regex": "{{my_api_key}}",
          "replace_with": "[REDACTED_API_KEY]"
        }
      ]
    },
    {
      "name": "Email Obfuscation",
      "enabled": true,
      "hotkey": "ctrl+alt+2",
      "reverse_hotkey": "ctrl+shift+alt+2",
      "replacements": [
        {
          "regex": "(?i)(my private email address|my personal email)",
          "replace_with": "{{personal_email}}",
          "preserve_case": false,
          "reverse_with": "my personal email"
        }
      ]
    }
  ]
}
```

> **Important Warning:** Replacements are processed sequentially in the order they appear within a profile. This means the order of your regex rules matters! Earlier replacements can affect the text that later replacements operate on. Consider this carefully when organizing your replacement rules to avoid unexpected results.

## Secure Secret Management

To avoid storing sensitive data (like passwords, API keys, emails) directly in `config.json`, you can use the secure secret management feature.

### How it Works

1.  **Storage:** Secrets are stored securely in your operating system's native credential store (e.g., Windows Credential Manager, macOS Keychain, Linux Secret Service/Keyring) under the service name `LLMClipboardFilter2`.
2.  **Configuration:** In `config.json`, you define a logical name for each secret in the top-level `secrets` map. The value should always be `"managed"`.
    ```json
    "secrets": {
      "my_api_key": "managed",
      "ssh_password": "managed"
    }
    ```
3.  **Usage in Rules:** Reference your secrets within the `regex` or `replace_with` (or `reverse_with`) fields of your replacement rules using double curly braces: `{{logical_name}}`.
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
4.  **Management:** Add, list, and remove secrets using the "Manage Secrets" submenu in the system tray. This will interact with your OS credential store.
5.  **Activation:** **A full manual restart of the application is required after adding or removing secrets** for the changes to take effect in replacement rules. Reloading configuration is *not* sufficient for activating new secrets.

### Managing Secrets via System Tray

-   **Add/Update Secret...**: Prompts you for a "Logical Name" (which you'll use in `{{...}}` placeholders) and the actual "Secret Value". The value is stored securely in the OS keychain/credential store, and the logical name is added to the `secrets` map in `config.json`. It can optionally create a basic replacement rule for the new secret.
-   **List Secret Names**: Shows a notification and logs the logical names of all secrets currently managed by the application (reads from the `secrets` map in `config.json`). It does *not* display the secret values.
-   **Remove Secret...**: Prompts you to select one of the managed logical names. Upon confirmation, it removes the secret from the OS keychain/credential store and removes its entry from the `secrets` map in `config.json`. **This action cannot be undone.**

## Multiple Profile Support

Clipboard Regex Replace supports multiple configuration profiles. Each profile can have its own set of replacement patterns and hotkey bindings.

### Features

- **Independent Profiles**: Create multiple distinct sets of replacement rules
- **Per-Profile Hotkeys**: Assign different hotkeys to different profiles
- **Dynamic Profile Toggling**: Enable or disable profiles via the system tray
- **Rule Merging**: Profiles with the same hotkey will have their rules merged during execution
- **Backward Compatibility**: Existing config.json files are automatically migrated

### How to Configure Multiple Profiles

Edit your `config.json` file to use the format shown in the [Configuration](#configuration) section, defining profiles within the `profiles` array.

### Profile Options

- **name**: A descriptive name for the profile (displayed in the system tray)
- **enabled**: Whether the profile is active (can be toggled in the tray)
- **hotkey**: The hotkey combination that triggers this profile
- **reverse_hotkey**: Optional hotkey for reverse replacements (bidirectional mode)
- **replacements**: An array of regex replacement rules for this profile, potentially using `{{secret_name}}` placeholders.

### Using Profiles

1.  **Triggering Specific Profiles**: Press the hotkey assigned to a profile to execute its replacements.
2.  **Toggling Profiles**: Enable or disable profiles via the "Profiles" submenu in the system tray.
3.  **Adding New Profiles**: Click "➕ Add New Profile" in the system tray, then edit the `config.json` file to customize it.
4.  **Shared Hotkeys**: Multiple enabled profiles can share the same hotkey. When that hotkey is pressed, all the replacement rules from those profiles will be applied in sequence.
5.  **Bidirectional Replacements**: Set up a `reverse_hotkey` to enable going from replaced text back to original text.
6.  **Restarting the Application**: Use the "Restart Application" option in the system tray if you experience menu duplication issues after config changes.

### Migration from Previous Versions

When you upgrade from a version before v1.4.0, your existing configuration will be automatically migrated to the new format. Your existing replacement rules will be placed in a "Default" profile that retains your original hotkey configuration.

## Case-Preserving and Reversible Replacements

Clipboard Regex Replace supports case-preserving and bidirectional replacements. Secret placeholders work with these features.

### Case Preservation

With case preservation, the application maintains the capitalization pattern when replacing text:

```json
{
  "regex": "(?i)(JohnDoe)",
  "replace_with": "GithubUser",
  "preserve_case": true
}
```

This will maintain the case pattern:

- `johndoe` → `githubuser` (lowercase preserved)
- `JOHNDOE` → `GITHUBUSER` (UPPERCASE preserved)
- `JohnDoe` → `GithubUser` (PascalCase preserved)
- `Johndoe` → `Githubuser` (First letter capitalized preserved)

### Bidirectional Replacements

You can add a `reverse_hotkey` to profiles to enable bidirectional replacements:

```json
{
  "name": "Privacy - Bidirectional",
  "enabled": true,
  "hotkey": "ctrl+alt+v",
  "reverse_hotkey": "shift+alt+v",
  "replacements": [
    {
      "regex": "(?i)({{real_name_alt1}}|{{real_name_alt2}}|John)", // Can mix secrets and text
      "replace_with": "{{github_user}}",
      "preserve_case": true
    }
  ]
}
```
Assuming `{{real_name_alt1}}` resolves to `JohnDoe_T`, `{{real_name_alt2}}` to `JohnDoe`, and `{{github_user}}` to `GithubUser`:
- Pressing `ctrl+alt+v` performs normal replacements (`JohnDoe` → `GithubUser`)
- Pressing `shift+alt+v` performs reverse replacements (`GithubUser` → `JohnDoe_T`, using the first resolved pattern by default).

### Custom Reverse Replacements

Override the default reverse text using the `reverse_with` field. Placeholders are supported here too.

```json
{
  "regex": "(?i)({{real_name_alt1}}|{{real_name_alt2}}|John)",
  "replace_with": "{{github_user}}",
  "preserve_case": true,
  "reverse_with": "{{real_name_alt2}}" // Reverse to the resolved value of real_name_alt2
}
```
Now, `shift+alt+v` would reverse `GithubUser` to `JohnDoe`.

## Usage

1.  **Running the Application:**
    - **Development:** `go run cmd/clipregex/main.go`
    - **Production:** Double-click the compiled `.exe` (or run it from terminal/startup).

2.  **Managing Secrets (First Time / Updates):**
    - Right-click the system tray icon.
    - Select **Manage Secrets** -> **Add/Update Secret...**.
    - Follow the prompts to enter a logical name and the secret value.
    - Optionally create a basic replacement rule.
    - **Important:** **Manually restart the application** for the new secret to be usable in replacements.

3.  **Triggering Clipboard Processing:**
    - Copy some text.
    - Press a configured hotkey (e.g., `Ctrl+Alt+V`).
    - The application reads the clipboard, resolves any `{{...}}` placeholders using secrets from the OS keychain, applies matching regex rules, updates the clipboard, simulates paste, and shows notifications.

4.  **Viewing Changes:**
    - Right-click the system tray icon after a replacement.
    - Select **View Last Change Details**. Opens a diff report in your browser.

5.  **Using Reverse Replacements (if configured):**
    - Copy text containing previously replaced content.
    - Press the configured reverse hotkey.
    - The application resolves secrets needed for the reverse mapping and applies the rules.

6.  **Reverting to Original Clipboard:**
    - Use the global revert hotkey (if configured) or the **Revert to Original** systray menu item (if temporary clipboard is enabled).

7.  **Editing Configuration:**
    - Right-click the systray icon -> **Open Config File**.

8.  **Reloading Configuration:**
    - Right-click the systray icon -> **Reload Configuration**. Applies changes from `config.json` (like modified rules, profile toggles) **except** for newly added or removed secrets.

9.  **Restarting Application:**
    - Right-click the systray icon -> **Restart Application**. Recommended after making significant profile changes.

10. **Exiting the Application:**
    - Right-click the systray icon -> **Quit**.

## Building for Windows

To build a Windows executable without a console window:

```bash
go build -ldflags="-H=windowsgui" -o dist/ClipboardRegexReplace.exe cmd/clipregex/main.go
```

For distribution, include the following files:

- `ClipboardRegexReplace.exe`
- `config.json.example` (rename to `config.json`)
- `icon.png` (optional, for higher quality notifications)

## Dependencies

- [github.com/99designs/keyring](https://github.com/99designs/keyring) - Secure secret storage using OS credential manager.
- [github.com/atotto/clipboard](https://github.com/atotto/clipboard) – Clipboard access.
- [github.com/gen2brain/beeep](https://github.com/gen2brain/beeep) – Fallback notification library.
- [github.com/getlantern/systray](https://github.com/getlantern/systray) – System tray icon.
- [github.com/go-toast/toast](https://github.com/go-toast/toast) – Windows toast notifications.
- [github.com/ncruces/zenity](https://github.com/ncruces/zenity) - Cross-platform native dialogs for secret management.
- [github.com/sergi/go-diff/diffmatchpatch](https://github.com/sergi/go-diff) – Text differencing library.
- [golang.design/x/hotkey](https://pkg.go.dev/golang.design/x/hotkey) – Global hotkey registration.


## Changelog

### 1.7.0 (Current Version)

-   **Feature: Secure Secret Management:**
    -   Store sensitive replacement values securely in the OS native credential store (keychain/credential manager).
    -   Added "Manage Secrets" submenu to system tray for adding, listing, and removing secrets via native dialogs (`zenity`).
    -   Introduced `secrets` map in `config.json` to declare managed secrets.
    -   Use `{{secret_name}}` placeholders in `regex`, `replace_with`, and `reverse_with` fields.
    -   **Note:** Requires manual application restart after adding/removing secrets.
-   **Dependency:** Added `github.com/99designs/keyring` and `github.com/ncruces/zenity`.
-   **Config:** Changed default file permissions for `config.json` to `0600` (owner read/write only). Added default config creation on first run if missing.

### 1.5.4

-   **Feature: Change Details Viewer:** Added a "View Last Change Details" option to the system tray menu. When clicked after a replacement, it opens an HTML report in the browser showing a summary and detailed diff of the changes.
-   **Refactor:** Improved diff generation logic for better accuracy and readability in HTML output.
-   **Fix:** Resolved issues with opening configuration file and diff report on Windows by using the ShellExecuteW API directly within the UI package.
-   **Refactor:** Moved platform-specific code (paste simulation, file opening) to dedicated files within relevant packages (`clipboard`, `ui`) using build tags.

### 1.5.3

-   **Open Configuration File:**
    - Add option in system tray to quickly open ```config.json``` in the default text editor

### 1.5.2

-   **Major Code Refactoring**:
    - Reorganized project into a proper Go package structure
    - Improved platform-specific clipboard paste handling
    - Enhanced error handling and logging
    - Better separation of concerns between packages
    - No functional changes, purely architectural improvements

### 1.5.1

-   **Global Revert Hotkey:**
    Added support for a dedicated global hotkey that reverts the clipboard to its original content when automatic reversion is disabled.

### 1.5.0

-   **Case-Preserving Replacements:**
    Added support for maintaining capitalization patterns when replacing text (lowercase, UPPERCASE, Title Case, PascalCase).
-   **Bidirectional Replacements:**
    Added `reverse_hotkey` to profiles for enabling reversible replacements.
-   **Custom Reverse Replacements:**
    Added `reverse_with` field to override the default text used in reverse replacements.

### 1.4.0

-   **Multiple Profile Support:**
    Added support for multiple named profiles, each with its own set of replacement rules and hotkey binding.
-   **Profile Management:**
    Profiles can be toggled on/off directly from the system tray.
-   **Rule Merging:**
    Profiles with the same hotkey have their replacement rules merged and applied sequentially.

### 1.3.1

-   **Fixed Original Clipboard Storage:**
    Fixed an issue where pressing the hotkey multiple times on already processed text would incorrectly overwrite the stored original clipboard content. The application now properly preserves the original clipboard text until either new content is copied or new replacements are performed.

### 1.3.0

-   **Dynamic Configuration Reloading:**
    Added ability to reload configuration without restarting the application.
-   **Automatic Clipboard Reversion:**
    Added option to automatically restore the original clipboard content immediately after pasting.
-   **Simplified Clipboard Management:**
    Streamlined the clipboard restoration interface to a single "Revert to Original" option in the system tray.

### 1.2.0
-   **Temporary Clipboard Storage:**
    Optionally store the original clipboard text before applying regex replacements. The replaced clipboard is pasted, and the original text is automatically restored after 10 seconds unless the user chooses to keep the replaced text.
-   **Interactive Options:**
    Added system tray menu items (and toast notification prompts on Windows) to allow users to revert to the original clipboard text or keep the replaced text.

### 1.1.0
-   Custom hotkey configuration.

### 1.0.0
-   Initial project.
-   Basic regex replacement.
-   Toast notification.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.