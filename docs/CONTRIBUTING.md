# Contributing Information

This document provides information useful for those looking to build the project from source or understand the codebase structure and dependencies.

For general usage instructions, please see the main [README.md](README.md).

## Requirements

*   [Go 1.16+](https://golang.org/dl/)
*   A Windows, macOS, or Linux machine with a supported native credential store for the Secure Secret Management feature.
    *   Windows: Credential Manager
    *   macOS: Keychain Access
    *   Linux: GNOME Keyring, KWallet, or other service implementing the Secret Service API.
*   For building: A suitable build environment for your target OS.

## Getting Started (Development)

1.  **Clone the Repository:**

    ```bash
    git clone https://github.com/TanaroSch/Clipboard-Regex-Replace-2.git
    cd Clipboard-Regex-Replace-2
    ```

2.  **Download Dependencies:**

    The repository uses Go modules. The required dependencies will be fetched automatically when you build or run the project. You can explicitly fetch/verify them using:

    ```bash
    go mod tidy
    ```

## Building from Source

To build a standard executable for your current operating system:

```bash
go build -o ClipboardRegexReplace cmd/clipregex/main.go
```

To build a Windows executable without a console window:

```bash
go build -ldflags="-H=windowsgui" -o ClipboardRegexReplace.exe cmd/clipregex/main.go
```

The resulting executable (e.g., `ClipboardRegexReplace` or `ClipboardRegexReplace.exe`) should be placed in the same directory as your `config.json` file.

## Project Structure

The project follows a modern Go project structure:

```
clipboard-regex-replace/
├── cmd/
│   └── clipregex/          # Main application entry point
├── internal/               # Internal application code (not meant for external use)
│   ├── app/                # Core application logic orchestration
│   ├── clipboard/          # Clipboard reading, writing, and transformation logic
│   ├── config/             # Configuration loading, saving, and secret management logic
│   ├── diffutil/           # Text difference generation utilities
│   ├── hotkey/             # Global hotkey registration and management
│   ├── resources/          # Embedded resources (like the application icon)
│   └── ui/                 # User interface elements (systray, notifications, dialogs)
├── dist/                   # (Optional) Directory for distribution builds
├── assets/                 # (Optional) Directory for external assets like images
├── go.mod                  # Go module definition
├── go.sum                  # Go module checksums
├── config.json.example     # Example configuration file
├── icon.png                # External icon for better notification quality (optional)
├── README.md               # Main project readme
├── CHANGELOG.md            # History of changes
└── LICENSE                 # Project license (MIT)
```
*(Note: The `dist/` and `assets/` directories might not be present in your current structure but are common conventions included here from the original README's example structure.)*

## Dependencies

*   [github.com/99designs/keyring](https://github.com/99designs/keyring) - Secure secret storage using OS credential manager.
*   [github.com/atotto/clipboard](https://github.com/atotto/clipboard) – Clipboard access.
*   [github.com/gen2brain/beeep](https://github.com/gen2brain/beeep) – Fallback notification library.
*   [github.com/getlantern/systray](https://github.com/getlantern/systray) – System tray icon.
*   [github.com/go-toast/toast](https://github.com/go-toast/toast) – Windows toast notifications.
*   [github.com/ncruces/zenity](https://github.com/ncruces/zenity) - Cross-platform native dialogs for secret management.
*   [github.com/sergi/go-diff/diffmatchpatch](https://github.com/sergi/go-diff) – Text differencing library.
*   [golang.design/x/hotkey](https://pkg.go.dev/golang.design/x/hotkey) – Global hotkey registration.

## License

Contributions are made under the project's [MIT License](LICENSE).