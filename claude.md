# Claude.md - Clipboard Regex Replace Project Guide

## Project Overview

**Clipboard Regex Replace** is a lightweight, cross-platform Go application that automates text transformations on clipboard content. It runs in the system tray, monitors for global hotkeys, and applies predefined regex rules to clipboard text with secure secret management.

**Version:** v1.7.3
**Language:** Go 1.23.6
**License:** MIT
**Platforms:** Windows, macOS, Linux X11 (Kubuntu/Ubuntu fully supported)

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
│   ├── LINUX_SUPPORT.md             # Linux/Kubuntu setup guide
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

| Package | Purpose | Version | Platform Notes |
|---------|---------|---------|----------------|
| `github.com/atotto/clipboard` | Read/write clipboard | v1.4.3 | Linux: Requires xclip/xsel/wl-clipboard |
| `golang.design/x/hotkey` | Global hotkey registration | v0.4.1 | Linux: Requires libx11-dev (CGo) |
| `github.com/getlantern/systray` | System tray icon & menu | v1.2.2 | Cross-platform (GTK on Linux) |
| `github.com/99designs/keyring` | Secure OS credential store | v1.2.2 | Linux: Secret Service/KWallet |
| `github.com/ncruces/zenity` | Native dialogs | v0.10.16 | Cross-platform |
| `github.com/go-toast/toast` | Windows toast notifications | v0.0.0 | Windows only |
| `github.com/gen2brain/beeep` | Cross-platform notifications | v0.0.0 | Linux: libnotify |
| `github.com/sergi/go-diff` | Generate text diffs | v1.3.1 | Cross-platform |

### Linux-Specific External Dependencies

On Linux (Kubuntu/Ubuntu), install these system packages:
```bash
sudo apt install -y libx11-dev xclip xdotool xdg-utils
```

See [docs/LINUX_SUPPORT.md](docs/LINUX_SUPPORT.md) for complete Linux setup instructions.

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
- **Thread-safe** using `sync.RWMutex` for concurrent access protection
- `ProcessClipboard()`: Main transformation engine
- Applies regex replacements from matching profiles
- **Regex timeout protection** (5s default) prevents ReDoS attacks
- Resolves `{{secret_name}}` placeholders from keychain
- Supports forward and reverse replacements
- Maintains clipboard history for revert
- Generates diff reports
- Handles platform-specific paste operations with **configurable delays**
- Case-preserving replacement logic
- **Panic recovery** in all background goroutines

### 4. Configuration Manager: `internal/config/config.go`
- Defines structs: `Config`, `ProfileConfig`, `Replacement`
- `Load()`: Reads config.json and resolves secrets
- **Comprehensive validation**: Checks regex patterns, profile names, hotkeys at load time
- `Save()`: Persists config changes
- Manages secret references (logical names → keyring storage)
- Creates default config if missing
- Backward compatibility handling
- **Performance settings** with defaults:
  - `PasteDelayMs` (400ms) - configurable paste delay
  - `RevertDelayMs` (300ms) - configurable revert delay
  - `RegexTimeoutMs` (5000ms) - regex operation timeout
  - `DiffContextLines` (3) - diff viewer context lines

### 5. Hotkey Manager: `internal/hotkey/hotkey.go`
- **Thread-safe** using `sync.RWMutex` for concurrent access protection
- Registers/unregisters global hotkeys using `golang.design/x/hotkey`
- Maps hotkey strings to callback functions
- Supports both forward and reverse hotkeys per profile
- **Goroutine lifecycle management** with quit channels for clean shutdown
- **No goroutine leaks** - all listeners properly stopped on config reload
- **Panic recovery** in all hotkey listeners
- Expanded KeyMap includes arrow keys (up, down, left, right) and aliases (esc, return)

### 6. UI Components: `internal/ui/`
- **systray.go**:
  - **Thread-safe** using `sync.RWMutex` for config access
  - System tray icon, dynamic menu with profile toggles
  - **Panic recovery** in all UI handlers
- **notifications.go**: Admin and replacement notifications with verbosity control
- **diffviewer.go**:
  - Generates HTML diff report with **configurable context lines**
  - Opens in browser
  - **Panic recovery** in cleanup goroutines
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
  "paste_delay_ms": 400,
  "revert_delay_ms": 300,
  "regex_timeout_ms": 5000,
  "diff_context_lines": 3,
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
- **paste_delay_ms**: Delay before paste simulation (default: 400ms, optional)
- **revert_delay_ms**: Delay before auto-revert (default: 300ms, optional)
- **regex_timeout_ms**: Timeout for regex operations (default: 5000ms, optional)
- **diff_context_lines**: Lines of context in diff viewer (default: 3, optional)
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

**Windows:**
```bash
# Standard executable
go build -o ClipboardRegexReplace.exe cmd/clipregex/main.go

# Windows GUI executable (no console window)
go build -ldflags="-H=windowsgui" -o ClipboardRegexReplace.exe cmd/clipregex/main.go
```

**Linux (Kubuntu/Ubuntu):**
```bash
# Install build dependencies first
sudo apt install -y libx11-dev xclip xdotool xdg-utils

# Build executable (CGo enabled by default)
go build -o clipboardregexreplace cmd/clipregex/main.go

# Make executable
chmod +x clipboardregexreplace
```

**Cross-Platform:**
```bash
# Run from source
go run cmd/clipregex/main.go

# Install/update Go dependencies
go mod tidy
```

### Development Requirements
- Go 1.16+ (project uses 1.23.6)
- GCC compiler (for CGo on Linux)
- OS with credential store support:
  - Windows: Credential Manager
  - macOS: Keychain
  - Linux: Secret Service (GNOME Keyring, KWallet)
- **Linux-specific**: libx11-dev, xclip/xsel, xdotool (see [LINUX_SUPPORT.md](docs/LINUX_SUPPORT.md))

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

## Recent Improvements (2026-01-19)

### Phase 1: Critical Race Conditions Fixed
**Problem:** Concurrent access to shared state could cause crashes and data corruption.

**Solutions:**
- Added `sync.RWMutex` to Clipboard, Hotkey, and Systray managers
- Protected all shared state with proper lock/unlock patterns
- Made defensive copies before long operations
- Fixed goroutine leaks in hotkey listeners with quit channels
- Added panic recovery throughout

### Phase 2: High Priority Improvements
**Modernization and Safety:**
- Replaced deprecated `ioutil` with Go 1.16+ `os` package
- Added panic recovery to all diffviewer goroutines
- Expanded KeyMap with arrow keys and aliases
- Implemented comprehensive config validation:
  - Validates regex patterns at load time
  - Checks for duplicate profile names
  - Validates hotkey strings
  - Warns about duplicate hotkeys

### Phase 3: Medium Priority Enhancements
**Performance and Protection:**
- **Configurable timeouts**: All delays now configurable via config.json
- **Regex timeout enforcement**: Prevents ReDoS attacks with 5-second default timeout
- **Context-aware timeout**: Uses `context.WithTimeout` for safe regex operations
- **Configurable diff viewer**: Context lines now customizable
- **No magic numbers**: All hardcoded values replaced with config options

**Security Benefits:**
- Protection against malicious regex patterns
- Timeout on long-running operations
- Panic recovery prevents crashes
- Thread-safe concurrent access

### Phase 4: Linux X11 Support (2026-01-19)
**Feature Addition:** Full Kubuntu/Ubuntu X11 Support

**Implementation:**
- Verified existing libraries already support Linux X11
- Documented Linux-specific dependencies and build process
- Created comprehensive Linux setup guide ([docs/LINUX_SUPPORT.md](docs/LINUX_SUPPORT.md))
- Updated README with Linux quick start instructions
- No code changes needed - ~90% of codebase already cross-platform

**Linux Requirements:**
- System packages: libx11-dev, xclip/xsel, xdotool, xdg-utils
- CGo enabled for X11 hotkey support
- X11 display server (Wayland experimental)

**Platform Support Status:**
- ✅ Windows: Fully supported
- ✅ macOS: Fully supported
- ✅ Linux X11: Fully supported (Kubuntu/Ubuntu tested)
- ⚠️ Linux Wayland: Experimental (clipboard works, hotkeys limited)

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
- Use build tags for platform-specific implementations:
  - `//go:build windows` for Windows-only code
  - `//go:build !windows` for Unix/Linux/macOS code
  - `//go:build linux` for Linux-specific code (if needed)
- **Windows**: `paste_windows.go`, `shellexecute_windows.go`
- **Unix (Linux/macOS)**: `paste_unix.go`
- Platform detection at runtime via `runtime.GOOS` for cross-platform code

**Linux Notes:**
- Clipboard operations require xclip/xsel/wl-clipboard installed
- Hotkey registration requires X11 (libx11) via CGo
- Paste simulation uses xdotool (X11) or wtype (Wayland)
- System tray uses GTK (usually pre-installed on desktop distros)
- File opening uses xdg-open standard
- See [docs/LINUX_SUPPORT.md](docs/LINUX_SUPPORT.md) for details

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
**Document Version:** 1.1
**Major Changes:**
- Documented Phase 1-3 improvements (race conditions, modernization, performance)
- Updated configuration structure with new performance settings
- Highlighted thread-safety improvements across all managers
- Added security enhancements documentation
