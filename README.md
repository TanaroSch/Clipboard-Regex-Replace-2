# Clipboard Regex Replace

Clipboard Regex Replace is a fast, standalone clipboard filtering application written in Go. It automatically applies a series of regex-based replacements to your clipboard text when you press a global hotkey, then updates your clipboard and simulates a paste action. Additionally, it provides Windows toast notifications and a system tray icon for easy management.

> **Note:** This implementation is a major upgrade compared to the initial Python implementation in [Clipboard-Regex-Replace](https://github.com/TanaroSch/Cliboard-Regex-Replace). It's designed to be lightweight, efficient, and easy to distribute as a single executable (with only external configuration).

## Features

- **Global Hotkey Trigger:**  
  Press a configurable hotkey (default: `Ctrl+Alt+V`) to process the clipboard text.

- **Regex-based Filtering:**  
  Define multiple regex replacement rules in an external configuration file (`config.json`).

- **Clipboard Automation:**  
  Automatically updates your clipboard content and simulates a paste.

- **Multiple Profile Support:**  
  Create and manage multiple sets of replacement rules with different hotkeys.

- **Case-Preserving Replacements:**  
  Maintain capitalization patterns when replacing text (e.g., lowercase, UPPERCASE, Title Case, PascalCase).

- **Bidirectional Replacements:**  
  Configure reverse hotkeys to switch back from replaced text to original text.

- **Temporary Clipboard Storage:**  
  Optionally store the original clipboard text before processing. You can choose to automatically revert to the original clipboard content after pasting or manually revert using the system tray menu.

- **Global Revert Hotkey:**  
  Configure a dedicated hotkey to quickly revert to the original clipboard content when automatic reversion is disabled.

- **Dynamic Configuration Reloading:**  
  Reload configuration changes without restarting the application using the system tray menu.

- **Open Configuration File:**
  Quickly open your `config.json` file in the default text editor directly from the system tray menu.

- **Windows Toast Notifications:**  
  Displays a toast notification to show successful replacement and configuration changes.

- **System Tray Icon:**  
  Runs in the background with a system tray icon and provides a menu for quick actions like opening the configuration file, reloading configuration, reverting clipboard, and exiting the application.

- **Standalone Executable:**  
  Easily build and distribute a single EXE file on Windows (with external configuration files).

## Requirements

- [Go 1.16+](https://golang.org/dl/)
- A Windows machine for building the Windows executable (or cross-compilation setup)

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

1. **Clone the Repository:**

   ```bash
   git clone https://github.com/TanaroSch/Clipboard-Regex-Replace-2.git
   cd Clipboard-Regex-Replace-2
   ```

2. **Download Dependencies:**

   The repository uses Go modules. The required dependencies will be fetched automatically when you build or run the project.

   ```bash
   go mod tidy
   ```

## Configuration

The application reads its configuration from an external `config.json` file. As of v1.5.0, Clipboard Regex Replace supports multiple profiles (see the [Multiple Profile Support](#multiple-profile-support) section).

For backward compatibility, the original configuration format is still supported:

```json
{
  "hotkey": "ctrl+alt+v",
  "use_notifications": true,
  "temporary_clipboard": true,
  "automatic_reversion": true,
  "replacements": [
    {
      "regex": "(?i)mypassword",
      "replace_with": "redacted_password"
    }
  ]
}
```

However, this format will be automatically migrated to the new multi-profile format when the application starts.

> **Important Warning:** Replacements are processed sequentially in the order they appear in the configuration file. This means the order of your regex rules matters! Earlier replacements can affect the text that later replacements operate on. Consider this carefully when organizing your replacement rules to avoid unexpected results.

## Multiple Profile Support

Clipboard Regex Replace supports multiple configuration profiles. Each profile can have its own set of replacement patterns and hotkey bindings.

### Features

- **Independent Profiles**: Create multiple distinct sets of replacement rules
- **Per-Profile Hotkeys**: Assign different hotkeys to different profiles
- **Dynamic Profile Toggling**: Enable or disable profiles via the system tray
- **Rule Merging**: Profiles with the same hotkey will have their rules merged during execution
- **Backward Compatibility**: Existing config.json files are automatically migrated

### How to Configure Multiple Profiles

Edit your `config.json` file to use the new format:

```json
{
  "use_notifications": true,
  "temporary_clipboard": true,
  "automatic_reversion": true,
  "revert_hotkey": "ctrl+shift+alt+r",
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
      "name": "Code Formatting",
      "enabled": true,
      "hotkey": "ctrl+alt+c",
      "replacements": [
        {
          "regex": "\\t",
          "replace_with": "    "
        }
      ]
    }
  ]
}
```

### Profile Options

- **name**: A descriptive name for the profile (displayed in the system tray)
- **enabled**: Whether the profile is active (can be toggled in the tray)
- **hotkey**: The hotkey combination that triggers this profile
- **reverse_hotkey**: Optional hotkey for reverse replacements (bidirectional mode)
- **replacements**: An array of regex replacement rules for this profile

### Using Profiles

1. **Triggering Specific Profiles**: Press the hotkey assigned to a profile to execute its replacements.
2. **Toggling Profiles**: Enable or disable profiles via the "Profiles" submenu in the system tray.
3. **Adding New Profiles**: Click "Add New Profile" in the system tray, then edit the config.json file to customize it.
4. **Shared Hotkeys**: Multiple enabled profiles can share the same hotkey. When that hotkey is pressed, all the replacement rules from those profiles will be applied in sequence.
5. **Bidirectional Replacements**: Set up a `reverse_hotkey` to enable going from replaced text back to original text.
6. **Restarting the Application**: If you experience menu duplication issues, use the "Restart Application" option in the system tray.

### Migration from Previous Versions

When you upgrade from an earlier version, your existing configuration will be automatically migrated to the new format. Your existing replacement rules will be placed in a "Default" profile that retains your original hotkey configuration.

## Case-Preserving and Reversible Replacements

Clipboard Regex Replace supports case-preserving and bidirectional replacements.

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
      "regex": "(?i)(JohnDoe_T|JohnDoe|John)",
      "replace_with": "GithubUser",
      "preserve_case": true
    }
  ]
}
```

When using bidirectional replacements:

- Pressing `ctrl+alt+v` performs normal replacements (`JohnDoe` → `GithubUser`)
- Pressing `shift+alt+v` performs reverse replacements (`GithubUser` → `JohnDoe_T`)

### Custom Reverse Replacements

By default, reverse replacements use the first pattern from the regex alternation. You can override this behavior with the `reverse_with` field:

```json
{
  "regex": "(?i)(JohnDoe_T|JohnDoe|John)",
  "replace_with": "GithubUser",
  "preserve_case": true,
  "reverse_with": "JohnDoe"
}
```

This specifies that `GithubUser` should be reversed to `JohnDoe` (not `JohnDoe_T` which would be the default).

## Usage

1. **Running the Application:**  
   You have two options for running the application:

   **Option 1:** During development, run it using Go:

   ```bash
   go run cmd/clipregex/main.go
   ```

   **Option 2:** Run the pre-compiled executable:

   - Simply double-click the `ClipboardRegexReplace.exe` file
   - Or create a shortcut to the executable and place it in your startup folder for automatic launch when Windows starts

   Either way, this will launch the application, register the hotkey, and show the system tray icon.

2. **Triggering Clipboard Processing:**  
   Copy some text, then press the configured hotkey (e.g., `Ctrl+Alt+V`). The application will:

   - Read your clipboard text.
   - Apply the configured regex replacements from all enabled profiles with matching hotkey.
   - Update the clipboard.
   - Simulate a paste action.
   - Display a toast notification (on Windows) indicating the number of replacements performed.
   - If enabled, automatically revert to the original clipboard content after pasting or store it for manual reversion through the system tray menu or revert hotkey.

3. **Using Reverse Replacements (if configured):**  
   Copy some text that contains previously replaced content, then press the reverse hotkey (e.g., `Shift+Alt+V`). The application will:

   - Replace any instances of replacement text with the original text.
   - Maintain case patterns if case preservation is enabled.

4. **Reverting to Original Clipboard:**  
   If automatic reversion is disabled but temporary clipboard is enabled, you can:

   - Press the configured revert hotkey (e.g., `Ctrl+Shift+Alt+R`)
   - Or right-click the system tray icon and select **Revert to Original**

5. **Editing Configuration:**
   To modify your `config.json` file, right-click the system tray icon and select **Open Config File**. This will attempt to open the file in your default text editor.

6. **Reloading Configuration:**
   If you update your `config.json` file while the application is running, you can apply the changes without restarting by right-clicking the system tray icon and selecting **Reload Configuration**.

7. **Exiting the Application:**  
   Right-click the system tray icon and select **Quit** to exit.

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

- [github.com/atotto/clipboard](https://github.com/atotto/clipboard) – Clipboard access.
- [github.com/gen2brain/beeep](https://github.com/gen2brain/beeep) – Fallback notification library.
- [github.com/getlantern/systray](https://github.com/getlantern/systray) – System tray icon.
- [github.com/go-toast/toast](https://github.com/go-toast/toast) – Windows toast notifications.
- [golang.design/x/hotkey](https://pkg.go.dev/golang.design/x/hotkey) – Global hotkey registration.

## Changelog

### 1.5.3

- **Open Configuration File:**
  - Add option in system tray to quickly open ```config.json``` in the default text editor

### 1.5.2

- **Major Code Refactoring**:
  - Reorganized project into a proper Go package structure
  - Improved platform-specific clipboard paste handling
  - Enhanced error handling and logging
  - Better separation of concerns between packages
  - No functional changes, purely architectural improvements

### 1.5.1

- **Global Revert Hotkey:**
  Added support for a dedicated global hotkey that reverts the clipboard to its original content when automatic reversion is disabled.

### 1.5.0

- **Case-Preserving Replacements:**
  Added support for maintaining capitalization patterns when replacing text (lowercase, UPPERCASE, Title Case, PascalCase).
- **Bidirectional Replacements:**
  Added `reverse_hotkey` to profiles for enabling reversible replacements.
- **Custom Reverse Replacements:**
  Added `reverse_with` field to override the default text used in reverse replacements.

### 1.4.0

- **Multiple Profile Support:**
  Added support for multiple named profiles, each with its own set of replacement rules and hotkey binding.
- **Profile Management:**
  Profiles can be toggled on/off directly from the system tray.
- **Rule Merging:**
  Profiles with the same hotkey have their replacement rules merged and applied sequentially.

### 1.3.1

- **Fixed Original Clipboard Storage:**  
  Fixed an issue where pressing the hotkey multiple times on already processed text would incorrectly overwrite the stored original clipboard content. The application now properly preserves the original clipboard text until either new content is copied or new replacements are performed.

### 1.3.0

- **Dynamic Configuration Reloading:**  
  Added ability to reload configuration without restarting the application.
- **Automatic Clipboard Reversion:**  
  Added option to automatically restore the original clipboard content immediately after pasting.
- **Simplified Clipboard Management:**  
  Streamlined the clipboard restoration interface to a single "Revert to Original" option in the system tray.

### 1.2.0
- **Temporary Clipboard Storage:**  
  Optionally store the original clipboard text before applying regex replacements. The replaced clipboard is pasted, and the original text is automatically restored after 10 seconds unless the user chooses to keep the replaced text.
- **Interactive Options:**  
  Added system tray menu items (and toast notification prompts on Windows) to allow users to revert to the original clipboard text or keep the replaced text.
  
### 1.1.0
- Custom hotkey configuration.
  
### 1.0.0
- Initial project.
- Basic regex replacement.
- Toast notification.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
