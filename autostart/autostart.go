// Package autostart manages the XDG autostart entry for k8s-context-switcher.
// It writes ~/.config/autostart/k8s-context-switcher.desktop so the tray applet
// launches automatically on login on any XDG-compliant desktop.
package autostart

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const desktopEntry = `[Desktop Entry]
Type=Application
Name=K8s Context Switcher
Comment=Tray applet to switch the current Kubernetes context
Exec=%s
Icon=utilities-terminal
Categories=Development;
X-GNOME-Autostart-enabled=true
Hidden=false
NoDisplay=false
`

func entryPath() (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cfg, "autostart")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "k8s-context-switcher.desktop"), nil
}

// IsEnabled reports whether the autostart .desktop file is present.
func IsEnabled() (bool, error) {
	p, err := entryPath()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(p)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// Enable writes the autostart .desktop file pointing at the current executable.
func Enable() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	p, err := entryPath()
	if err != nil {
		return err
	}
	return os.WriteFile(p, []byte(fmt.Sprintf(desktopEntry, exe)), 0o644)
}

// Disable removes the autostart .desktop file if it exists.
func Disable() error {
	p, err := entryPath()
	if err != nil {
		return err
	}
	if err := os.Remove(p); !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
