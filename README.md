# Clipboard Regex Replace

Clipboard Regex Replace is a fast, standalone clipboard filtering application written in Go. It automatically applies a series of regex-based replacements to your clipboard text when you press a global hotkey, then updates your clipboard and simulates a paste action. Additionally, it provides Windows toast notifications and a system tray icon for easy management.

> **Note:** This implementation is a major upgrade compared to the initial Python implementation in [Clipboard-Regex-Replace](https://github.com/TanaroSch/Cliboard-Regex-Replace). It’s designed to be lightweight, efficient, and easy to distribute as a single executable (with only external configuration).

## Features

- **Global Hotkey Trigger:**  
  Press a configurable hotkey (default: `Ctrl+Alt+V`) to process the clipboard text.

- **Regex-based Filtering:**  
  Define multiple regex replacement rules in an external configuration file (`config.json`).

- **Clipboard Automation:**  
  Automatically updates your clipboard content and simulates a paste.

- **Temporary Clipboard Storage:**  
  Optionally store the original clipboard text before processing and automatically revert it after pasting. You can also choose to keep the replaced text using the system tray menu options or Windows toast notification actions (if supported).

- **Windows Toast Notifications:**  
  Displays a toast notification to show successful replacement and notify about temporary clipboard reversion.

- **System Tray Icon:**  
  Runs in the background with a system tray icon and provides a menu for quick exit and managing clipboard reversion.

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

The application reads its configuration from an external `config.json` file. Create a file named `config.json` in the same folder as the executable with contents similar to the following (or rename `config.json.example`):

```json
{
  "hotkey": "ctrl+alt+v",
  "use_notifications": true,
  "temporary_clipboard": true,
  "reversion_timeout": 10,
  "replacements": [
    {
      "regex": "(?i)mypassword",
      "replace_with": "redacted_password"
    },
    {
      "regex": "(?i)(surname|name)",
      "replace_with": "my_name"
    }
  ]
}
```

- **hotkey:** The global hotkey to trigger the filtering.
- **use_notifications:** Set to `true` to enable desktop notifications.
- **temporary_clipboard:** Set to `true` to enable storing the original clipboard text. When enabled, the replaced clipboard text is pasted, and the original text is automatically restored after a timeout unless you choose otherwise from the tray menu or notification actions.
- **reversion_timeout:** The number of seconds to wait before automatically reverting the clipboard to its original content (default is 10 seconds if not specified).
- **replacements:** An array of regex rules with their corresponding replacement text.

> **Important Warning:** Replacements are processed sequentially in the order they appear in the configuration file. This means the order of your regex rules matters! Earlier replacements can affect the text that later replacements operate on. Consider this carefully when organizing your replacement rules to avoid unexpected results.

## Usage

1. **Running the Application:**  
   You can run the application during development with:

   ```bash
   go run main.go
   ```

   This will launch the application, register the hotkey, and show the system tray icon.

2. **Triggering Clipboard Processing:**  
   Copy some text, then press the configured hotkey (e.g., `Ctrl+Alt+V`). The application will:
   - Read your clipboard text.
   - Apply the configured regex replacements.
   - Update the clipboard.
   - Simulate a paste action.
   - Display a toast notification (on Windows) indicating the number of replacements performed.
   - If enabled, temporarily store the original clipboard content and revert it automatically after 10 seconds (unless you choose to keep the replaced text via the tray menu or notification action).

3. **Exiting the Application:**  
   Right-click the system tray icon and select **Quit** to exit.

## Building for Windows

To build a Windows executable without a console window, run the following command:

```bash
go build -ldflags="-H=windowsgui" -o ClipboardRegexReplace.exe main.go
```

Distribute the resulting `ClipboardRegexReplace.exe` along with the external files `config.json` and optionally `icon.png`. Optionally, a shortcut of the `ClipboardRegexReplace.exe` can be placed in the startup folder.

## Dependencies

- [github.com/atotto/clipboard](https://github.com/atotto/clipboard) – Clipboard access.
- [github.com/gen2brain/beeep](https://github.com/gen2brain/beeep) – Fallback notification library.
- [github.com/getlantern/systray](https://github.com/getlantern/systray) – System tray icon.
- [github.com/go-toast/toast](https://github.com/go-toast/toast) – Windows toast notifications.
- [golang.design/x/hotkey](https://pkg.go.dev/golang.design/x/hotkey) – Global hotkey registration.

## Changelog

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