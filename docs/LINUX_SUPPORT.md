# Linux Support for Clipboard Regex Replace

## Overview

Clipboard Regex Replace fully supports Linux distributions with X11 (including Kubuntu). The application uses the same core codebase (~90%) with platform-specific implementations for clipboard, hotkeys, and paste simulation.

**Status**: X11 fully supported | Wayland partial

**Wayland Note**: Global hotkeys are not yet available on Wayland due to compositor security restrictions. Backend architecture is in place for future XDG Desktop Portal implementation. Clipboard operations, system tray, and secret management all work on Wayland.

---

## System Requirements

### Supported Environments
- **X11 (X Window System)**: ✅ Fully supported
- **Wayland**: ⚠️ Partial support
  - ✅ Clipboard operations work
  - ✅ System tray works
  - ✅ Secret management works
  - ❌ Global hotkeys not yet available (backend architecture ready for future XDG Portal implementation)

### Tested Distributions
- Kubuntu 22.04+ (KDE Plasma)
- Ubuntu 22.04+
- Debian-based distributions

---

## Dependencies

### Required Packages

Install the following packages before building or running:

```bash
# Development libraries (required for building)
sudo apt install -y libx11-dev

# Clipboard utilities (at least one required)
sudo apt install -y xclip    # Recommended
# OR
sudo apt install -y xsel     # Alternative

# Paste simulation (required for auto-paste feature)
sudo apt install -y xdotool

# File opening utilities (usually pre-installed)
sudo apt install -y xdg-utils

# Optional: Secret storage support
sudo apt install -y gnome-keyring  # For GNOME
# OR
sudo apt install -y kwalletmanager # For KDE (usually pre-installed on Kubuntu)
```

### Quick Install (One Command)

For Kubuntu/Ubuntu with KDE:
```bash
sudo apt install -y libx11-dev xclip xdotool xdg-utils
```

For Ubuntu with GNOME:
```bash
sudo apt install -y libx11-dev xclip xdotool xdg-utils gnome-keyring
```

### Wayland Support (Experimental)

For Wayland environments, install these instead:
```bash
sudo apt install -y wl-clipboard wtype
```

Note: Global hotkey support on Wayland is limited due to security restrictions in the compositor.

---

## Building on Linux

### Prerequisites
- Go 1.16 or higher (project uses 1.23.6)
- GCC compiler (for CGo)
- libx11-dev installed

### Build Commands

```bash
# Navigate to project directory
cd clipboard-regex-replace-2

# Ensure dependencies are installed
sudo apt install -y libx11-dev xclip xdotool xdg-utils

# Download Go dependencies
go mod tidy

# Build the executable
go build -o clipboardregexreplace cmd/clipregex/main.go

# Make it executable
chmod +x clipboardregexreplace

# Run the application
./clipboardregexreplace
```

### Build Flags

```bash
# Standard build
go build -o clipboardregexreplace cmd/clipregex/main.go

# Build with debug symbols
go build -gcflags="all=-N -l" -o clipboardregexreplace cmd/clipregex/main.go

# Build with optimizations
go build -ldflags="-s -w" -o clipboardregexreplace cmd/clipregex/main.go
```

---

## Installation

### Option 1: Manual Installation

```bash
# Build the application
go build -o clipboardregexreplace cmd/clipregex/main.go

# Copy to user bin directory
mkdir -p ~/.local/bin
cp clipboardregexreplace ~/.local/bin/
cp icon.png ~/.local/share/icons/clipboardregexreplace.png
cp config.json.example ~/.config/clipboardregexreplace/config.json

# Ensure ~/.local/bin is in PATH
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

### Option 2: System-Wide Installation

```bash
# Build the application
go build -o clipboardregexreplace cmd/clipregex/main.go

# Install system-wide (requires sudo)
sudo cp clipboardregexreplace /usr/local/bin/
sudo cp icon.png /usr/share/icons/clipboardregexreplace.png
```

---

## Running on Linux

### Starting the Application

```bash
# Run from terminal (logs will be visible)
./clipboardregexreplace

# Run in background
./clipboardregexreplace &

# Run detached from terminal
nohup ./clipboardregexreplace > /dev/null 2>&1 &
```

### Autostart on Login

#### KDE Plasma (Kubuntu)

1. **Create autostart desktop entry:**
```bash
mkdir -p ~/.config/autostart
nano ~/.config/autostart/clipboardregexreplace.desktop
```

2. **Add the following content:**
```ini
[Desktop Entry]
Type=Application
Name=Clipboard Regex Replace
Comment=Automated clipboard text transformations
Exec=/home/YOUR_USERNAME/.local/bin/clipboardregexreplace
Icon=/home/YOUR_USERNAME/.local/share/icons/clipboardregexreplace.png
Terminal=false
StartupNotify=false
X-KDE-autostart-after=panel
```

3. **Replace YOUR_USERNAME with your actual username**

4. **Make it executable:**
```bash
chmod +x ~/.config/autostart/clipboardregexreplace.desktop
```

#### GNOME/Ubuntu Desktop

Create the same desktop entry in `~/.config/autostart/` as above.

---

## Configuration

### Config File Location

Linux follows XDG Base Directory specification:

```bash
# Default search locations (in order):
1. ./config.json (current directory)
2. ~/.config/clipboardregexreplace/config.json
3. /etc/clipboardregexreplace/config.json
```

### Creating Initial Config

```bash
# Copy example config
mkdir -p ~/.config/clipboardregexreplace
cp config.json.example ~/.config/clipboardregexreplace/config.json

# Edit configuration
nano ~/.config/clipboardregexreplace/config.json
```

### Secret Storage

Secrets are stored in the system keyring:

- **KDE (Kubuntu)**: KWallet
- **GNOME (Ubuntu)**: GNOME Keyring

Ensure the keyring service is running:

```bash
# Check KWallet status (Kubuntu)
kwalletmanager5

# Check GNOME Keyring status (Ubuntu)
gnome-keyring-daemon --version
```

---

## Platform-Specific Notes

### X11 vs Wayland Detection

The application automatically adapts based on available tools:

```bash
# Check your display server
echo $XDG_SESSION_TYPE

# Output "x11" = X11 session (full support)
# Output "wayland" = Wayland session (limited hotkey support)
```

### Hotkey Behavior

**X11:**
- Global hotkeys work system-wide
- AutoRepeat: Key release events may fire continuously while key is held
- Some keys may map to multiple modifier keys (e.g., Ctrl+Alt+S → Ctrl+Mod2+Mod4+S)

**Wayland:**
- Global hotkeys are restricted by compositor security
- May require compositor-specific configuration
- Consider using application-specific hotkeys instead

### Clipboard Behavior

The clipboard library will automatically detect and use:
1. `xclip` (preferred)
2. `xsel` (fallback)
3. `wl-clipboard` (Wayland)

### Paste Simulation

The paste simulation tries methods in order:
1. `xdotool` (X11)
2. `wtype` (Wayland)

If both fail, transformations still work but auto-paste is disabled.

---

## Troubleshooting

### "No clipboard utilities available"

**Problem**: Clipboard read/write fails

**Solution**:
```bash
sudo apt install -y xclip xsel
```

### "Hotkey registration failed"

**Problem**: Global hotkeys don't register

**Solutions**:

1. **Check X11 is running:**
```bash
echo $DISPLAY
# Should output something like ":0" or ":1"
```

2. **Install X11 development libraries:**
```bash
sudo apt install -y libx11-dev
```

3. **Check for conflicting hotkeys:**
```bash
# Use KDE System Settings → Shortcuts
# Or check with:
xdotool search --name ""
```

4. **Rebuild with CGo enabled:**
```bash
CGO_ENABLED=1 go build -o clipboardregexreplace cmd/clipregex/main.go
```

### "Could not simulate paste"

**Problem**: Clipboard transformation works but doesn't paste

**Solution**:
```bash
# For X11
sudo apt install -y xdotool

# For Wayland
sudo apt install -y wtype
```

### System tray icon not showing

**Problem**: Application runs but no tray icon

**Solutions**:

1. **KDE Plasma (Kubuntu):**
- Right-click on system tray
- Select "Configure System Tray"
- Under "Entries", ensure "Clipboard Regex Replace" is set to "Show"

2. **Install tray support:**
```bash
# For older systems
sudo apt install -y libappindicator3-1
```

3. **Check if systray is enabled:**
```bash
# For KDE
qdbus org.kde.plasmashell /PlasmaShell
```

### Secret management not working

**Problem**: Cannot save/retrieve secrets

**Solution**:

```bash
# For KDE (Kubuntu)
sudo apt install -y kwalletmanager
kwalletmanager5

# For GNOME
sudo apt install -y gnome-keyring
gnome-keyring-daemon --start
```

### Permission denied errors

**Problem**: Cannot access certain resources

**Solution**:
```bash
# Make executable
chmod +x clipboardregexreplace

# Check config file permissions
chmod 644 ~/.config/clipboardregexreplace/config.json
```

### Building fails with CGo errors

**Problem**: Build fails with C compilation errors

**Solutions**:

1. **Install build essentials:**
```bash
sudo apt install -y build-essential
```

2. **Install X11 development files:**
```bash
sudo apt install -y libx11-dev
```

3. **Set CGo explicitly:**
```bash
CGO_ENABLED=1 go build -o clipboardregexreplace cmd/clipregex/main.go
```

---

## Performance Considerations

### Memory Usage

Typical memory footprint on Linux:
- Idle: 15-30 MB
- Active (processing): 30-60 MB

### CPU Usage

- Idle: <1% CPU
- During transformation: 5-15% CPU (brief spike)

### Startup Time

- X11: ~100-300ms
- Wayland: ~200-400ms

---

## Known Limitations

1. **Wayland Hotkeys**: Global hotkeys have limited support on Wayland due to compositor security policies

2. **Modifier Key Mapping**: Some key combinations may register differently than expected (e.g., additional Mod keys)

3. **Clipboard Formats**: Only plain text clipboard content is supported (no images, rich text, etc.)

4. **Headless Servers**: Requires X11 server (use Xvfb for headless environments)

---

## Testing the Installation

### Quick Test

```bash
# 1. Start the application
./clipboardregexreplace

# 2. Check system tray for icon

# 3. Copy some text to clipboard
echo "test  text  with   spaces" | xclip -selection clipboard

# 4. Trigger your configured hotkey (e.g., Ctrl+Alt+V)

# 5. Paste the result
# It should be transformed according to your rules
```

### Verify Dependencies

```bash
# Check all required tools are installed
command -v xclip && echo "✓ xclip installed" || echo "✗ xclip missing"
command -v xdotool && echo "✓ xdotool installed" || echo "✗ xdotool missing"
command -v xdg-open && echo "✓ xdg-open installed" || echo "✗ xdg-open missing"

# Check X11 libraries
pkg-config --exists x11 && echo "✓ libx11 installed" || echo "✗ libx11 missing"
```

---

## Uninstallation

```bash
# Remove application files
rm ~/.local/bin/clipboardregexreplace
rm ~/.local/share/icons/clipboardregexreplace.png

# Remove configuration
rm -rf ~/.config/clipboardregexreplace/

# Remove autostart entry
rm ~/.config/autostart/clipboardregexreplace.desktop

# Remove secrets from keyring
# Use KWallet Manager or GNOME Seahorse to remove stored secrets
```

---

## Additional Resources

### Library Documentation

- **Clipboard**: [github.com/atotto/clipboard](https://github.com/atotto/clipboard)
- **Hotkey**: [golang.design/x/hotkey](https://github.com/golang-design/hotkey) | [hotkey_linux.go](https://github.com/golang-design/hotkey/blob/main/hotkey_linux.go)
- **System Tray**: [github.com/getlantern/systray](https://github.com/getlantern/systray)
- **Keyring**: [github.com/99designs/keyring](https://github.com/99designs/keyring)

### Kubuntu/KDE Resources

- [KDE Plasma Documentation](https://docs.kde.org/)
- [KWallet User Guide](https://docs.kde.org/stable5/en/kdeutils/kwallet5/)
- [System Tray Configuration](https://userbase.kde.org/Plasma/SystemTray)

### X11 Tools

- [xdotool Documentation](https://www.semicomplete.com/projects/xdotool/)
- [xclip Manual](https://github.com/astrand/xclip)

---

## Contributing Linux-Specific Improvements

See [CONTRIBUTING.md](CONTRIBUTING.md) for general guidelines.

Linux-specific contributions welcome:
- Wayland hotkey improvements
- Additional clipboard format support
- AppImage or Flatpak packaging
- Distribution-specific packages (.deb, .rpm)
- Integration with desktop environments

---

**Last Updated**: 2026-01-19
**Document Version**: 1.0
