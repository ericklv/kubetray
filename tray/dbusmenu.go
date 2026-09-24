package tray

import (
	"slices"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
)

const (
	menuInterface = "com.canonical.dbusmenu"
)

// MenuEntry is one clickable line in the tray menu.
type MenuEntry struct {
	ID         int32  // stable numeric id, referenced by Event()
	Label      string // e.g. the context's cluster name
	IconName   string // theme icon name, or "" for none
	Checked    bool   // true if this entry is active/selected
	ToggleType string // "radio" (default), "checkmark" or "none"; ignored for separators and headers
	Separator  bool   // if true, renders a visual divider; other fields are ignored
	Header     bool   // if true, renders Label as a greyed-out, unclickable section title
}

// Menu implements com.canonical.dbusmenu backing a single flat list of
// MenuEntry items. There are no submenus: contexts are grouped under
// Header entries instead, so switching is always a single click.
type Menu struct {
	path dbus.ObjectPath
	conn *dbus.Conn

	mu       sync.Mutex
	entries  []MenuEntry
	revision uint32

	// OnSelect is called with the MenuEntry.ID the user clicked.
	OnSelect func(id int32)
}

// NewMenu exports a Menu at the given object path.
func NewMenu(conn *dbus.Conn, path dbus.ObjectPath) (*Menu, error) {
	m := &Menu{path: path, conn: conn}

	if err := conn.Export(m, path, menuInterface); err != nil {
		return nil, err
	}

	node := &introspect.Node{
		Name: string(path),
		Interfaces: []introspect.Interface{
			introspect.IntrospectData,
			{
				Name: menuInterface,
				Methods: []introspect.Method{
					{Name: "GetLayout", Args: []introspect.Arg{
						{Name: "parentId", Type: "i", Direction: "in"},
						{Name: "recursionDepth", Type: "i", Direction: "in"},
						{Name: "propertyNames", Type: "as", Direction: "in"},
						{Name: "revision", Type: "u", Direction: "out"},
						{Name: "layout", Type: "(ia{sv}av)", Direction: "out"},
					}},
					{Name: "GetGroupProperties", Args: []introspect.Arg{
						{Name: "ids", Type: "ai", Direction: "in"},
						{Name: "propertyNames", Type: "as", Direction: "in"},
						{Name: "properties", Type: "a(ia{sv})", Direction: "out"},
					}},
					{Name: "Event", Args: []introspect.Arg{
						{Name: "id", Type: "i", Direction: "in"},
						{Name: "eventId", Type: "s", Direction: "in"},
						{Name: "data", Type: "v", Direction: "in"},
						{Name: "timestamp", Type: "u", Direction: "in"},
					}},
					{Name: "AboutToShow", Args: []introspect.Arg{
						{Name: "id", Type: "i", Direction: "in"},
						{Name: "needUpdate", Type: "b", Direction: "out"},
					}},
				},
				Signals: []introspect.Signal{
					{Name: "LayoutUpdated", Args: []introspect.Arg{
						{Name: "revision", Type: "u"},
						{Name: "parentId", Type: "i"},
					}},
				},
			},
		},
	}
	if err := conn.Export(introspect.NewIntrospectable(node), path, "org.freedesktop.DBus.Introspectable"); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Menu) ObjectPath() dbus.ObjectPath { return m.path }

// SetEntries replaces the full entry list (e.g. after the kubeconfig
// changed) and notifies any listening host
// via LayoutUpdated so it redraws.
func (m *Menu) SetEntries(entries []MenuEntry) {
	m.mu.Lock()
	if slices.Equal(m.entries, entries) {
		m.mu.Unlock()
		return
	}
	m.entries = entries
	m.revision++
	rev := m.revision
	m.mu.Unlock()

	m.conn.Emit(m.path, menuInterface+".LayoutUpdated", rev, int32(0))
}

// entryItem builds the (ia{sv}av) dbus struct for one menu entry. The
// dbusmenu spec's "layout" type is this recursively-variant-wrapped
// struct; godbus marshals a Go struct positionally into a dbus struct,
// so field order here must match the signature exactly: id, then
// a{sv} properties, then av children (empty, since we have no submenus).
type entryItem struct {
	ID         int32
	Properties map[string]dbus.Variant
	Children   []dbus.Variant
}

// entryProps builds the dbusmenu property map for a single MenuEntry.
func entryProps(e MenuEntry) map[string]dbus.Variant {
	if e.Separator {
		return map[string]dbus.Variant{
			"type":    dbus.MakeVariant("separator"),
			"visible": dbus.MakeVariant(true),
		}
	}
	if e.Header {
		return map[string]dbus.Variant{
			"label":   dbus.MakeVariant(e.Label),
			"enabled": dbus.MakeVariant(false),
			"visible": dbus.MakeVariant(true),
		}
	}
	toggleType := e.ToggleType
	switch toggleType {
	case "":
		toggleType = "radio"
	case "none":
		toggleType = ""
	}
	state := int32(0)
	if e.Checked {
		state = 1
	}
	p := map[string]dbus.Variant{
		"label":        dbus.MakeVariant(e.Label),
		"enabled":      dbus.MakeVariant(true),
		"visible":      dbus.MakeVariant(true),
		"toggle-type":  dbus.MakeVariant(toggleType),
		"toggle-state": dbus.MakeVariant(state),
	}
	if e.IconName != "" {
		p["icon-name"] = dbus.MakeVariant(e.IconName)
	}
	return p
}

func (m *Menu) buildLayout() entryItem {
	m.mu.Lock()
	defer m.mu.Unlock()

	children := make([]dbus.Variant, 0, len(m.entries))
	for _, e := range m.entries {
		children = append(children, dbus.MakeVariant(entryItem{
			ID: e.ID, Properties: entryProps(e), Children: []dbus.Variant{},
		}))
	}
	return entryItem{
		ID:         0,
		Properties: map[string]dbus.Variant{"children-display": dbus.MakeVariant("submenu")},
		Children:   children,
	}
}

// GetLayout is called by the host to fetch the full menu tree.
func (m *Menu) GetLayout(parentID int32, recursionDepth int32, propertyNames []string) (uint32, entryItem, *dbus.Error) {
	m.mu.Lock()
	rev := m.revision
	m.mu.Unlock()
	return rev, m.buildLayout(), nil
}

// groupProps mirrors the a(ia{sv}) return type of GetGroupProperties.
type groupProps struct {
	ID         int32
	Properties map[string]dbus.Variant
}

// GetGroupProperties lets the host re-fetch properties for specific
// ids without re-walking the whole layout.
func (m *Menu) GetGroupProperties(ids []int32, propertyNames []string) ([]groupProps, *dbus.Error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	wanted := make(map[int32]MenuEntry, len(m.entries))
	for _, e := range m.entries {
		wanted[e.ID] = e
	}

	var out []groupProps
	for _, id := range ids {
		if e, ok := wanted[id]; ok {
			out = append(out, groupProps{ID: id, Properties: entryProps(e)})
		}
	}
	return out, nil
}

// Event is called by the host on user interaction; we only care about
// "clicked" on one of our leaf entries.
func (m *Menu) Event(id int32, eventID string, data dbus.Variant, timestamp uint32) *dbus.Error {
	if eventID == "clicked" && m.OnSelect != nil {
		m.OnSelect(id)
	}
	return nil
}

// AboutToShow is called right before the host displays the menu; we
// have no lazy-loaded submenus, so nothing needs refreshing here.
func (m *Menu) AboutToShow(id int32) (bool, *dbus.Error) {
	return false, nil
}
