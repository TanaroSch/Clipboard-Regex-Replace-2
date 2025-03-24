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

- **Temporary Clipboard Storage:**  
  Optionally store the original clipboard text before processing. You can choose to automatically revert to the original clipboard content after pasting or manually revert using the system tray menu option.

- **Dynamic Configuration Reloading:**  
  Reload configuration changes without restarting the application using the system tray menu.

- **Windows Toast Notifications:**  
  Displays a toast notification to show successful replacement and configuration changes.

- **System Tray Icon:**  
  Runs in the background with a system tray icon and provides a menu for quick actions like reloading configuration, reverting clipboard, and exiting the application.

- **Standalone Executable:**  
  Easily build and distribute a single EXE file on Windows (with external configuration files).

## Requirements

- [Go 1.16+](https://golang.org/dl/)
- A Windows machine for building the Windows executable (or cross-compilation setup)

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

The application reads its configuration from an external `config.json` file. As of v1.4.0, Clipboard Regex Replace supports multiple profiles (see the [Multiple Profile Support](#multiple-profile-support) section).

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

As of version 1.4.0, Clipboard Regex Replace supports multiple configuration profiles. Each profile can have its own set of replacement patterns and hotkey bindings.

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
- **replacements**: An array of regex replacement rules for this profile

### Using Profiles

1. **Triggering Specific Profiles**: Press the hotkey assigned to a profile to execute its replacements.
2. **Toggling Profiles**: Enable or disable profiles via the "Profiles" submenu in the system tray.
3. **Adding New Profiles**: Click "Add New Profile" in the system tray, then edit the config.json file to customize it.
4. **Shared Hotkeys**: Multiple enabled profiles can share the same hotkey. When that hotkey is pressed, all the replacement rules from those profiles will be applied in sequence.
5. **Restarting the Application**: If you experience menu duplication issues, use the "Restart Application" option in the system tray.

### Migration from Previous Versions

When you upgrade from an earlier version, your existing configuration will be automatically migrated to the new format. Your existing replacement rules will be placed in a "Default" profile that retains your original hotkey configuration.

## Usage

1. **Running the Application:**  
   You have two options for running the application:

   **Option 1:** During development, run it using Go:
   ```bash
   go run main.go keymap.go
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
   - If enabled, automatically revert to the original clipboard content after pasting or store it for manual reversion through the system tray menu.

3. **Reloading Configuration:**  
   If you update your `config.json` file while the application is running, you can apply the changes without restarting by right-clicking the system tray icon and selecting **Reload Configuration**.

4. **Exiting the Application:**  
   Right-click the system tray icon and select **Quit** to exit.

## Building for Windows

To build a Windows executable without a console window, run the following command:

```bash
go build -ldflags="-H=windowsgui" -o ClipboardRegexReplace.exe main.go keymap.go
```

Distribute the resulting `ClipboardRegexReplace.exe` along with the external files `config.json` and optionally `icon.png`. Optionally, a shortcut of the `ClipboardRegexReplace.exe` can be placed in the startup folder.

## Dependencies

- [github.com/atotto/clipboard](https://github.com/atotto/clipboard) – Clipboard access.
- [github.com/gen2brain/beeep](https://github.com/gen2brain/beeep) – Fallback notification library.
- [github.com/getlantern/systray](https://github.com/getlantern/systray) – System tray icon.
- [github.com/go-toast/toast](https://github.com/go-toast/toast) – Windows toast notifications.
- [golang.design/x/hotkey](https://pkg.go.dev/golang.design/x/hotkey) – Global hotkey registration.

## Changelog

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