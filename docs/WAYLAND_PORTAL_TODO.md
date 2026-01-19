# Wayland Portal Implementation Guide

## Status: NOT YET IMPLEMENTED (v1.8.0)

This document outlines the roadmap for implementing full Wayland global hotkey support via the XDG Desktop Portal GlobalShortcuts interface.

---

## Current State (v1.8.0)

### ✅ What Works on Wayland
- Clipboard read/write operations (`wl-clipboard`)
- Paste simulation (`wtype`)
- System tray (GTK)
- Secret management (KWallet/GNOME Keyring)
- File opening (`xdg-open`)

### ❌ What Doesn't Work
- **Global hotkeys**: Not available due to Wayland security model
- Application runs but hotkey registration is skipped
- Users must manually trigger clipboard processing (future: add UI button?)

### ✅ Architecture Ready
- Backend abstraction layer implemented ([internal/hotkey/backend.go](../internal/hotkey/backend.go))
- Display server detection functional ([internal/hotkey/detect.go](../internal/hotkey/detect.go))
- Portal backend stub exists ([internal/hotkey/backend_portal.go](../internal/hotkey/backend_portal.go))
- Build tags ensure platform compatibility

---

## Implementation Roadmap

### Phase 1: Add D-Bus Dependency (Easy)

**Files to modify:**
- `go.mod`: Add `github.com/godbus/dbus/v5 v5.1.0`

**Commands:**
```bash
go get github.com/godbus/dbus/v5
go mod tidy
```

**Estimated effort:** 5 minutes

---

### Phase 2: Implement Portal Backend (Medium)

**File to implement:** `internal/hotkey/backend_portal.go`

**Required methods:**

#### 2.1 Connection Setup
```go
func NewPortalBackend() (*PortalBackend, error) {
    // Connect to session D-Bus
    conn, err := dbus.SessionBus()
    if err != nil {
        return nil, err
    }

    // Get portal object
    obj := conn.Object(
        "org.freedesktop.portal.Desktop",
        "/org/freedesktop/portal/desktop",
    )

    return &PortalBackend{
        conn: conn,
        obj:  obj,
        shortcuts: make(map[string]*portalHotkey),
    }, nil
}
```

#### 2.2 Session Creation
```go
func (b *PortalBackend) createSession() (dbus.ObjectPath, error) {
    // Call CreateSession method
    // Parameters: options (map)
    // Returns: session handle (ObjectPath)

    var sessionPath dbus.ObjectPath
    err := b.obj.Call(
        "org.freedesktop.portal.GlobalShortcuts.CreateSession",
        0,
        map[string]dbus.Variant{
            "session_handle_token": dbus.MakeVariant("clipregex_session"),
        },
    ).Store(&sessionPath)

    return sessionPath, err
}
```

#### 2.3 Bind Shortcuts (Shows Permission Dialog)
```go
func (b *PortalBackend) Register(hotkeyStr string) (RegisteredHotkey, error) {
    // Parse hotkey string to D-Bus format
    // Format: <Ctrl><Alt>v
    dbusFormat := convertToDBusFormat(hotkeyStr)

    // Create shortcuts array
    shortcuts := []map[string]dbus.Variant{
        {
            "id":          dbus.MakeVariant(hotkeyStr),
            "description": dbus.MakeVariant("Clipboard transformation"),
            "preferred_trigger": dbus.MakeVariant(dbusFormat),
        },
    }

    // Call BindShortcuts - this shows dialog to user
    var response map[string]dbus.Variant
    err := b.obj.Call(
        "org.freedesktop.portal.GlobalShortcuts.BindShortcuts",
        0,
        b.session,
        shortcuts,
        "", // parent_window
        map[string]dbus.Variant{},
    ).Store(&response)

    // Create hotkey wrapper and start listening for signals
    ph := &portalHotkey{
        hotkeyStr: hotkeyStr,
        keydownCh: make(chan struct{}, 1),
    }

    b.shortcuts[hotkeyStr] = ph
    b.startSignalListener(ph)

    return ph, nil
}
```

#### 2.4 Signal Listener
```go
func (b *PortalBackend) startSignalListener(ph *portalHotkey) {
    // Subscribe to Activated signal
    // Signal: org.freedesktop.portal.GlobalShortcuts.Activated
    // Parameters: session, shortcut_id, timestamp, options

    ch := make(chan *dbus.Signal, 10)
    b.conn.Signal(ch)

    // Match rule for our session
    rule := fmt.Sprintf(
        "type='signal',interface='org.freedesktop.portal.GlobalShortcuts',member='Activated',path='%s'",
        b.session,
    )
    b.conn.BusObject().Call(
        "org.freedesktop.DBus.AddMatch",
        0,
        rule,
    )

    go func() {
        for signal := range ch {
            if signal.Name == "org.freedesktop.portal.GlobalShortcuts.Activated" {
                shortcutID := signal.Body[1].(string)
                if shortcutID == ph.hotkeyStr {
                    select {
                    case ph.keydownCh <- struct{}{}:
                    default:
                        // Channel full, skip this event
                    }
                }
            }
        }
    }()
}
```

#### 2.5 Hotkey Format Conversion
```go
func convertToDBusFormat(hotkeyStr string) string {
    // Convert "ctrl+alt+v" to "<Ctrl><Alt>v"
    parts := strings.Split(strings.ToLower(hotkeyStr), "+")

    var result strings.Builder
    for i, part := range parts {
        if i == len(parts)-1 {
            // Last part is the key
            result.WriteString(strings.ToUpper(part[:1]))
            result.WriteString(part[1:])
        } else {
            // Modifiers
            switch part {
            case "ctrl":
                result.WriteString("<Ctrl>")
            case "alt":
                result.WriteString("<Alt>")
            case "shift":
                result.WriteString("<Shift>")
            case "super", "win", "cmd":
                result.WriteString("<Super>")
            }
        }
    }

    return result.String()
}
```

**Estimated effort:** 1-2 days

---

### Phase 3: Update Backend Selection (Easy)

**File to modify:** `internal/hotkey/backend_legacy.go` (SelectBackend function)

**Change:**
```go
case DisplayServerWayland:
    // Check if Portal is available
    if HasPortalSupport() {
        backend := NewPortalBackend()
        if backend.IsAvailable() {
            log.Printf("Selected backend: %s for Wayland", backend.Name())
            return backend
        }
    }

    // Fallback: no hotkeys
    log.Println("Wayland detected without Portal support - hotkeys unavailable")
    return nil
```

**Estimated effort:** 15 minutes

---

### Phase 4: Integrate into Manager (Medium)

**File to modify:** `internal/hotkey/hotkey.go`

**Changes needed:**

1. Add backend field to Manager:
```go
type Manager struct {
    mu                sync.RWMutex
    config            *config.Config
    backend           Backend  // NEW: Backend abstraction
    registeredHotkeys map[string]RegisteredHotkey  // Changed type
    quitChannels      map[string]chan struct{}
    onTrigger         func(string, bool)
    onRevert          func()
}
```

2. Update NewManager:
```go
func NewManager(cfg *config.Config, onTrigger func(string, bool), onRevert func()) *Manager {
    backend := SelectBackend()
    if backend == nil {
        log.Println("Warning: No hotkey backend available, hotkeys will be disabled")
    } else {
        log.Printf("Using hotkey backend: %s", backend.Name())
    }

    return &Manager{
        config:            cfg,
        backend:           backend,
        registeredHotkeys: make(map[string]RegisteredHotkey),
        quitChannels:      make(map[string]chan struct{}),
        onTrigger:         onTrigger,
        onRevert:          onRevert,
    }
}
```

3. Update registerProfileHotkey to use backend:
```go
func (m *Manager) registerProfileHotkey(profile config.ProfileConfig, hotkeyStr string, isReverse bool) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    // Check if backend is available
    if m.backend == nil {
        return fmt.Errorf("no hotkey backend available")
    }

    // Skip if already registered
    if _, exists := m.registeredHotkeys[hotkeyStr]; exists {
        return nil
    }

    // Register via backend
    hk, err := m.backend.Register(hotkeyStr)
    if err != nil {
        return err
    }

    // Rest remains the same, but use hk.Keydown() instead of hotkey.Keydown()
    // ...
}
```

**Estimated effort:** 2-3 hours

---

### Phase 5: Testing (Critical)

**Test environments needed:**
1. **KDE Plasma Wayland** (primary target for Kubuntu)
2. **GNOME Wayland** (Ubuntu)
3. **Sway/wlroots** (if time permits)

**Test cases:**

| Test Case | Expected Result |
|-----------|----------------|
| App starts on Wayland | Portal backend selected, no errors |
| First hotkey registration | Permission dialog appears |
| User approves hotkey | Hotkey works, triggers transformation |
| User denies hotkey | Graceful error, app continues without hotkeys |
| User customizes hotkey | Custom key combo works |
| Multiple hotkeys | All show in one permission dialog |
| App restart | Hotkeys work without new permission dialog |
| Fallback on X11 | Legacy backend used, existing behavior |
| Fallback on Windows | Legacy backend used, existing behavior |

**Estimated effort:** 1-2 days

---

## Total Estimated Effort

- **Phase 1 (Dependency)**: 5 minutes
- **Phase 2 (Portal Backend)**: 1-2 days
- **Phase 3 (Backend Selection)**: 15 minutes
- **Phase 4 (Manager Integration)**: 2-3 hours
- **Phase 5 (Testing)**: 1-2 days

**Total: 3-5 days** for someone familiar with D-Bus and Go

---

## Reference Documentation

### Official Specifications
- [XDG Desktop Portal GlobalShortcuts](https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.GlobalShortcuts.html)
- [D-Bus Specification](https://dbus.freedesktop.org/doc/dbus-specification.html)

### Library Documentation
- [godbus/dbus v5](https://pkg.go.dev/github.com/godbus/dbus/v5)
- [godbus examples](https://github.com/godbus/dbus/tree/master/_examples)

### Portal Examples
- [Mumble Portal PR](https://github.com/mumble-voip/mumble/pull/5976) - Real-world Portal implementation
- [KDE Portal Source](https://invent.kde.org/plasma/xdg-desktop-portal-kde) - Portal backend implementation

---

## Alternative: Manual Clipboard Mode

If Portal implementation is delayed, consider adding a UI button to manually trigger clipboard processing:

```go
// In systray menu
menu.AddSeparator()
processBtn := menu.AddMenuItem("Process Clipboard Now", "Manually trigger transformation")
processBtn.Click(func() {
    // Call clipboard processing without hotkey
    app.ProcessClipboardManual()
})
```

This gives Wayland users a workaround until hotkeys are available.

---

## Questions & Considerations

### Q: What if user denies permission?
**A:** App continues to run, hotkeys are disabled. Consider showing one-time notification explaining hotkeys won't work. Manual processing button recommended.

### Q: Do permissions persist across restarts?
**A:** Yes, Portal stores approved shortcuts. Subsequent launches don't require permission dialog (unless shortcuts change).

### Q: Can we test on XWayland?
**A:** XWayland runs X11 apps on Wayland. App would detect X11 and use Legacy backend, bypassing Portal. Need native Wayland testing.

### Q: What about other Wayland compositors (Sway, Hyprland)?
**A:** Should work if compositor implements GlobalShortcuts portal. May require testing on each compositor.

---

**Last Updated:** 2026-01-19
**Document Version:** 1.0
**Author:** Claude Sonnet 4.5
**Status:** Ready for implementation when Wayland support is prioritized
