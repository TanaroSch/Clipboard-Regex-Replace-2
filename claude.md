# Claude.md - Clipboard Regex Replace Project Guide

## Project Overview

**Clipboard Regex Replace** is a lightweight, cross-platform Go application that automates text transformations on clipboard content. It runs in the system tray, monitors for global hotkeys, and applies predefined regex rules to clipboard text with secure secret management.

**Version:** v1.7.3
**Language:** Go 1.23.6
**License:** MIT
**Platforms:** Windows, macOS, Linux

---

## Core Functionality

The application monitors keyboard input for user-defined global hotkeys. When triggered:
1. Reads current clipboard text
2. Resolves `{{secret_name}}` placeholders from OS keychain
3. Applies regex replacements from enabled profiles matching the hotkey
4. Updates clipboard with transformed text
5. Simulates paste action (Ctrl+V)
6. Shows change diff in browser
7. Optionally auto-reverts clipboard after paste

**Key Use Cases:**
- Sanitize sensitive data (API keys, PII) before pasting to LLMs or public forums
- Clean code formatting and remove debug statements
- Anonymize configurations and logs
- Automate repetitive text transformations
- Chain complex edits under single hotkey

---

## Project Structure

```
c:\Programs\Clipboard-Regex-Replace-2/
├── cmd/
│   └── clipregex/
│       └── main.go                  # Application entry point
├── internal/                        # Core application modules
│   ├── app/
│   │   └── app.go                   # Application orchestration & callbacks
│   ├── clipboard/
│   │   ├── clipboard.go             # Clipboard operations & regex processing
│   │   ├── paste_windows.go         # Windows paste simulation
│   │   └── paste_unix.go            # Unix paste simulation
│   ├── config/
│   │   └── config.go                # Configuration & secret management
│   ├── diffutil/
│   │   └── diffutil.go              # Text diff generation
│   ├── hotkey/
│   │   └── hotkey.go                # Global hotkey registration
│   ├── resources/
│   │   ├── resources.go             # Embedded resources
│   │   └── main_icon.go             # Icon data
│   └── ui/
│       ├── systray.go               # System tray menu & icon
│       ├── notifications.go         # Toast/desktop notifications
│       ├── diffviewer.go            # HTML diff report generator
│       └── shellexecute_windows.go  # Windows file opening
├── docs/                            # Documentation
│   ├── FEATURES.md                  # Detailed feature documentation
│   ├── CONFIGURATION.md             # Config structure & examples
│   ├── CONTRIBUTING.md              # Development guide
│   └── CHANGELOG.md                 # Version history
├── demo/                            # Screenshots & GIFs
├── config.json.example              # Example configuration
├── config.json                      # Active configuration (gitignored)
├── icon.png                         # Application icon
├── ClipboardRegexReplace.exe        # Compiled Windows executable (gitignored)
├── CliboardRegexReplace.bat         # Autostart batch script
├── go.mod / go.sum                  # Go dependencies
├── README.md                        # Main documentation
├── .claudeignore                    # Claude context exclusions
└── LICENSE                          # MIT license
```

---

## Key Dependencies

| Package | Purpose | Version |
|---------|---------|---------|
| `github.com/atotto/clipboard` | Read/write clipboard | v1.4.3 |
| `golang.design/x/hotkey` | Global hotkey registration | v0.4.1 |
| `github.com/getlantern/systray` | System tray icon & menu | v1.2.2 |
| `github.com/99designs/keyring` | Secure OS credential store | v1.2.2 |
| `github.com/ncruces/zenity` | Native dialogs | v0.10.16 |
| `github.com/go-toast/toast` | Windows toast notifications | v0.0.0 |
| `github.com/gen2brain/beeep` | Cross-platform notifications | v0.0.0 |
| `github.com/sergi/go-diff` | Generate text diffs | v1.3.1 |

---

## Core Components

### 1. Entry Point: `cmd/clipregex/main.go`
- Initializes logging, loads config and secrets
- Creates Application instance
- Runs main event loop with panic recovery
- Handles graceful shutdown

### 2. Application Orchestrator: `internal/app/app.go`
- Central `Application` struct coordinating all managers
- Manages clipboard, hotkey, and systray subsystems
- Implements callbacks for:
  - Hotkey triggers (forward and reverse)
  - Config reload without restart
  - Secret management (add/update/remove)
  - Profile enable/disable
  - Clipboard revert
  - Diff viewing
  - Simple rule addition

### 3. Clipboard Manager: `internal/clipboard/clipboard.go`
- `ProcessClipboard()`: Main transformation engine
- Applies regex replacements from matching profiles
- Resolves `{{secret_name}}` placeholders from keychain
- Supports forward and reverse replacements
- Maintains clipboard history for revert
- Generates diff reports
- Handles platform-specific paste operations
- Case-preserving replacement logic

### 4. Configuration Manager: `internal/config/config.go`
- Defines structs: `Config`, `ProfileConfig`, `Replacement`
- `Load()`: Reads config.json and resolves secrets
- `Save()`: Persists config changes
- Manages secret references (logical names → keyring storage)
- Creates default config if missing
- Backward compatibility handling

### 5. Hotkey Manager: `internal/hotkey/hotkey.go`
- Registers/unregisters global hotkeys using `golang.design/x/hotkey`
- Maps hotkey strings to callback functions
- Supports both forward and reverse hotkeys per profile
- Lifecycle management for hotkey registration

### 6. UI Components: `internal/ui/`
- **systray.go**: System tray icon, dynamic menu with profile toggles
- **notifications.go**: Admin and replacement notifications with verbosity control
- **diffviewer.go**: Generates HTML diff report and opens in browser
- **shellexecute_windows.go**: Windows-specific file opening

---

## Configuration Structure

The application uses `config.json` in the executable's directory:

```json
{
  "admin_notification_level": "Error|Warn|Info|None",
  "notify_on_replacement": true,
  "temporary_clipboard": true,
  "automatic_reversion": false,
  "revert_hotkey": "ctrl+shift+alt+r",
  "secrets": {
    "my_api_key": "managed",
    "my_email": "managed"
  },
  "profiles": [
    {
      "name": "General Cleanup",
      "enabled": true,
      "hotkey": "ctrl+alt+v",
      "reverse_hotkey": "shift+alt+v",
      "replacements": [
        {
          "regex": "\\s+",
          "replace_with": " ",
          "preserve_case": false,
          "reverse_with": ""
        }
      ]
    }
  ]
}
```

**Key Settings:**
- **admin_notification_level**: Controls verbosity (Error/Warn/Info/None)
- **notify_on_replacement**: Show notifications on successful transformations
- **temporary_clipboard**: Enable clipboard backup for revert
- **automatic_reversion**: Auto-revert clipboard after paste
- **secrets**: Logical names mapped to OS keychain entries
- **profiles**: Rule sets with individual hotkeys and enable/disable state

**Replacement Options:**
- `regex`: Regular expression pattern
- `replace_with`: Replacement text (can include `{{secret_name}}`)
- `preserve_case`: Maintain original text casing
- `reverse_with`: Pattern for reverse transformation

---

## Building & Development

### Build Commands

```bash
# Standard executable
go build -o ClipboardRegexReplace cmd/clipregex/main.go

# Windows GUI executable (no console window)
go build -ldflags="-H=windowsgui" -o ClipboardRegexReplace.exe cmd/clipregex/main.go

# Run from source
go run cmd/clipregex/main.go

# Install dependencies
go mod tidy
```

### Development Requirements
- Go 1.16+ (project uses 1.23.6)
- OS with credential store support:
  - Windows: Credential Manager
  - macOS: Keychain
  - Linux: Secret Service (GNOME Keyring, KWallet)

### Testing
Currently no automated test suite. Manual testing workflow:
1. Build executable
2. Create test config.json with sample rules
3. Test hotkey triggers with various clipboard content
4. Verify secret resolution from keychain
5. Test systray menu interactions
6. Verify cross-platform compatibility

---

## Development Workflow

### Making Changes

1. **Code Changes:**
   - Modify files in `internal/` directories
   - Follow Go conventions and existing code style
   - Maintain separation of concerns

2. **Config Changes:**
   - Update `config.json.example` if schema changes
   - Ensure backward compatibility in `config.go`
   - Document new options in `docs/CONFIGURATION.md`

3. **Feature Additions:**
   - Add callbacks in `app.go` if needed
   - Update systray menu in `ui/systray.go`
   - Add documentation to `docs/FEATURES.md`

4. **Testing:**
   - Build and run locally
   - Test on target platforms (Windows/macOS/Linux)
   - Verify hotkey registration
   - Test secret management flow

5. **Documentation:**
   - Update README.md for user-facing changes
   - Update CHANGELOG.md with version notes
   - Update relevant docs/ files

### Common Operations

**Adding a New Replacement Feature:**
1. Extend `Replacement` struct in `config/config.go`
2. Update replacement logic in `clipboard/clipboard.go`
3. Update `config.json.example`
4. Document in `docs/FEATURES.md`

**Adding a New UI Feature:**
1. Add menu item in `ui/systray.go`
2. Create callback in `app/app.go`
3. Wire callback in `main.go` initialization
4. Update screenshots in `demo/`

**Adding a New Global Setting:**
1. Add field to `Config` struct in `config/config.go`
2. Update default config creation
3. Use setting in relevant component
4. Document in `docs/CONFIGURATION.md`

---

## Important Patterns & Conventions

### Secret Management
- Secrets stored in OS keychain, never in config.json
- Config uses placeholder syntax: `{{secret_name}}`
- Secret references marked as "managed" in config
- Requires app restart after adding/removing secrets

### Hotkey Registration
- Hotkeys registered on startup for enabled profiles
- Config reload does NOT re-register hotkeys (requires restart)
- Both forward and reverse hotkeys supported per profile

### Callback Architecture
- `app.go` coordinates all subsystems via callbacks
- Callbacks passed to managers during initialization
- Enables loose coupling between components

### Error Handling
- Errors displayed via notifications at admin_notification_level
- Non-fatal errors logged but don't crash app
- Panic recovery in main loop

### Platform-Specific Code
- Use build tags for platform-specific implementations
- Windows: `paste_windows.go`, `shellexecute_windows.go`
- Unix: `paste_unix.go`

---

## Git Workflow

**Current State:**
- Branch: `main`
- Modified: `go.mod`, `go.sum`
- Recent commits focused on diff viewer and notification controls

**Ignored Files (.claudeignore):**
- Binaries: `bin/`, `dist/`, `*.exe`
- Logs: `clipboard_regex_replace.log`
- Config: `config.json` (user-specific)
- Build artifacts: `*.o`, `*.obj`, `vendor/`

**Commit Guidelines:**
- Use conventional commit format
- Include version bump in commit message for releases
- Co-authored with Claude: `Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>`

---

## Security Considerations

### Sensitive Data
- API keys, passwords stored in OS keychain only
- Config file safe to version control (contains "managed" markers)
- Secret placeholders prevent accidental exposure in logs

### Input Validation
- Regex patterns validated on load
- Hotkey strings validated before registration
- Secret names sanitized

### Permissions
- Requires clipboard access
- Requires global hotkey registration
- Requires OS credential store access
- No network access required

---

## Common Issues & Solutions

### Hotkeys Not Working
- Check if hotkey already registered by another app
- Verify hotkey syntax in config.json
- Try restarting application (not just reload config)

### Secrets Not Resolving
- Ensure secret added via "Manage Secrets" menu
- Verify app restarted after adding secret
- Check OS credential store permissions

### Config Reload vs. Restart
- **Reload Config**: Changes to rules, notifications, profile enable/disable
- **Restart Required**: New secrets, new hotkeys, new profiles

### Build Issues
- Run `go mod tidy` to sync dependencies
- Check Go version (requires 1.16+)
- Verify platform-specific dependencies installed

---

## Feature Roadmap & TODOs

### Implemented (v1.7.3)
- Multi-profile support with individual hotkeys
- Secure secret management via OS keychain
- Regex replacements with case preservation
- Reversible transformations
- Browser-based diff viewer
- Granular notification controls
- Simple rule editor via systray
- Temporary clipboard with revert
- Cross-platform support

### Potential Future Enhancements
- Automated test suite
- Rule templates/presets
- Import/export profiles
- Hotkey conflict detection
- Rule testing/preview mode
- Statistics tracking
- Cloud config sync
- Plugin system for custom transformations

---

## Contact & Contributing

**Repository:** https://github.com/TanaroSch/Clipboard-Regex-Replace-2
**Issues:** https://github.com/TanaroSch/Clipboard-Regex-Replace-2/issues
**License:** MIT

For detailed contributing guidelines, see [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md).

---

## Quick Reference

### File Locations
- **Config:** `config.json` (same directory as executable)
- **Example:** `config.json.example`
- **Logs:** `clipboard_regex_replace.log` (if logging enabled)
- **Secrets:** OS credential store (Windows Credential Manager, macOS Keychain, Linux Secret Service)

### Key Hotkeys (Default)
- `Ctrl+Alt+V`: General Cleanup profile
- `Ctrl+Alt+C`: Code Formatting profile
- `Ctrl+Shift+Alt+R`: Revert clipboard to original

### Systray Menu Actions
- **Toggle Profiles**: Enable/disable individual profiles
- **View Last Change Details**: Open diff in browser
- **Revert to Original**: Restore clipboard before transformation
- **Add Simple Rule**: Quick 1:1 text replacement
- **Manage Secrets**: Add/update/remove secure secrets
- **Open Config File**: Edit config.json
- **Reload Configuration**: Apply config changes without restart
- **Restart Application**: Full restart (for secrets/hotkeys)
- **Quit**: Exit application

---

## Code Style Guidelines

- Use Go standard formatting (`gofmt`)
- Follow Go naming conventions (PascalCase exports, camelCase private)
- Keep functions focused and single-purpose
- Document exported functions and types
- Use meaningful variable names
- Prefer explicit error handling over panic
- Keep structs in same file as primary methods
- Group imports: stdlib, external, internal

---

**Last Updated:** 2026-01-19
**Document Version:** 1.0
**Claude Agent ID:** a85a04a (Explore agent used for project analysis)
