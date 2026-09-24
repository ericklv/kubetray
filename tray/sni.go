// Package tray implements just enough of the freedesktop
// StatusNotifierItem + com.canonical.dbusmenu specs to show a tray
// icon with a click menu, without linking against GTK or any toolkit.
// It talks to whatever StatusNotifierHost is running (e.g. waybar's
// tray module) purely over D-Bus.
package tray

import (
	"fmt"
	"os"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
)

const (
	sniObjectPath     = dbus.ObjectPath("/StatusNotifierItem")
	sniInterface      = "org.kde.StatusNotifierItem"
	watcherInterface  = "org.kde.StatusNotifierWatcher"
	watcherObjectPath = dbus.ObjectPath("/StatusNotifierWatcher")
	watcherBusName    = "org.kde.StatusNotifierWatcher"
)

// Pixmap mirrors one (iiay) entry of the SNI IconPixmap property:
// width, height, then pixels as ARGB32 in network byte order.
type Pixmap struct {
	Width  int32
	Height int32
	Data   []byte
}

// toolTip mirrors the SNI ToolTip property, (sa(iiay)ss): icon name,
// icon pixmaps, title, then description. Field order must match.
type toolTip struct {
	IconName    string
	IconPixmaps []Pixmap
	Title       string
	Description string
}

// Item is a minimal StatusNotifierItem. ActivateFunc is called when
// the user left-clicks the tray icon itself (as opposed to picking a
// menu entry); for this app that's unused but wired up for completeness.
type Item struct {
	conn        *dbus.Conn
	props       *prop.Properties
	menu        *Menu
	OnLeftClick func()
}

// NewItem creates the SNI object, exports it, and registers it with
// the StatusNotifierWatcher. Unlike a theme IconName, pixmaps are
// drawn by the app itself; menu is the dbusmenu this item points at.
func NewItem(conn *dbus.Conn, id, title string, pixmaps []Pixmap, menu *Menu) (*Item, error) {
	it := &Item{conn: conn, menu: menu}

	propsSpec := prop.Map{
		sniInterface: {
			"Category":   {Value: "ApplicationStatus", Writable: false, Emit: prop.EmitTrue},
			"Id":         {Value: id, Writable: false, Emit: prop.EmitTrue},
			"Title":      {Value: title, Writable: false, Emit: prop.EmitTrue},
			"Status":     {Value: "Active", Writable: false, Emit: prop.EmitTrue},
			"IconName":   {Value: "", Writable: false, Emit: prop.EmitTrue},
			"IconPixmap": {Value: pixmaps, Writable: false, Emit: prop.EmitTrue},
			"ToolTip":    {Value: toolTip{Title: title, IconPixmaps: []Pixmap{}}, Writable: false, Emit: prop.EmitTrue},
			"ItemIsMenu": {Value: true, Writable: false, Emit: prop.EmitTrue},
			"Menu":       {Value: menu.ObjectPath(), Writable: false, Emit: prop.EmitTrue},
			"WindowId":   {Value: int32(0), Writable: false, Emit: prop.EmitTrue},
		},
	}

	p, err := prop.Export(conn, sniObjectPath, propsSpec)
	if err != nil {
		return nil, fmt.Errorf("exporting SNI properties: %w", err)
	}
	it.props = p

	if err := conn.Export(it, sniObjectPath, sniInterface); err != nil {
		return nil, fmt.Errorf("exporting SNI methods: %w", err)
	}

	node := &introspect.Node{
		Name: string(sniObjectPath),
		Interfaces: []introspect.Interface{
			introspect.IntrospectData,
			prop.IntrospectData,
			{
				Name: sniInterface,
				Methods: []introspect.Method{
					{Name: "Activate", Args: []introspect.Arg{
						{Name: "x", Type: "i", Direction: "in"},
						{Name: "y", Type: "i", Direction: "in"},
					}},
					{Name: "ContextMenu", Args: []introspect.Arg{
						{Name: "x", Type: "i", Direction: "in"},
						{Name: "y", Type: "i", Direction: "in"},
					}},
				},
				Signals: []introspect.Signal{
					{Name: "NewTitle"},
					{Name: "NewIcon"},
					{Name: "NewToolTip"},
				},
			},
		},
	}
	if err := conn.Export(introspect.NewIntrospectable(node), sniObjectPath, "org.freedesktop.DBus.Introspectable"); err != nil {
		return nil, fmt.Errorf("exporting SNI introspection: %w", err)
	}

	if err := it.registerWithWatcher(); err != nil {
		return nil, err
	}
	return it, nil
}

// SetStatus updates the icon, title and tooltip together. Hosts don't
// watch PropertiesChanged on SNI items; they re-fetch a property only
// after its dedicated New* signal, so each change emits one.
func (it *Item) SetStatus(title, description string, pixmaps []Pixmap) {
	it.props.SetMust(sniInterface, "Title", title)
	it.props.SetMust(sniInterface, "IconPixmap", pixmaps)
	it.props.SetMust(sniInterface, "ToolTip", toolTip{
		IconPixmaps: []Pixmap{},
		Title:       title,
		Description: description,
	})
	for _, sig := range []string{"NewTitle", "NewIcon", "NewToolTip"} {
		it.conn.Emit(sniObjectPath, sniInterface+"."+sig)
	}
}

// Activate is called by the host on a plain left-click of the icon.
func (it *Item) Activate(x, y int32) *dbus.Error {
	if it.OnLeftClick != nil {
		it.OnLeftClick()
	}
	return nil
}

// ContextMenu is called by some hosts on right-click; we treat it the
// same as a normal click since our left-click menu already covers it —
// most hosts just open the Menu object path regardless.
func (it *Item) ContextMenu(x, y int32) *dbus.Error {
	return nil
}

// registerWithWatcher requests a well-known bus name (as SNI hosts
// generally expect, rather than relying on the connection's unique
// name) and registers it with org.kde.StatusNotifierWatcher.
func (it *Item) registerWithWatcher() error {
	busName := fmt.Sprintf("org.kde.StatusNotifierItem-%d-1", os.Getpid())
	reply, err := it.conn.RequestName(busName, dbus.NameFlagDoNotQueue)
	if err != nil {
		return fmt.Errorf("requesting bus name: %w", err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return fmt.Errorf("bus name %s already taken", busName)
	}

	go func() {
		watcher := it.conn.Object(watcherBusName, watcherObjectPath)
		for {
			call := watcher.Call(watcherInterface+".RegisterStatusNotifierItem", 0, busName)
			if call.Err == nil {
				return
			}
			time.Sleep(1 * time.Second)
		}
	}()
	return nil
}
