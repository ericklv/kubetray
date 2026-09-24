// Command gke-context-switcher is a tray icon that lets you pick the
// current kubeconfig context, built without any GTK dependency: the
// tray icon and menu are implemented directly against the freedesktop
// StatusNotifierItem / com.canonical.dbusmenu D-Bus specs.
//
// Requires a StatusNotifierHost to actually show the icon — on niri
// that's typically waybar's `tray` module (or snixembed for other bars).
package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"

	"gke-context-switcher/autostart"
	"gke-context-switcher/icon"
	"gke-context-switcher/kubecontext"
	"gke-context-switcher/tray"
)

const (
	menuObjectPath = dbus.ObjectPath("/MenuBar")
	appID          = "gke-context-switcher"
	watchInterval  = 2 * time.Second
	// Sessions expire without touching kubeconfig, so recheck periodically.
	sessionInterval = time.Minute
)

// prodPattern matches context names that look like production, which
// turns the tray icon red as a reminder before running anything.
var prodPattern = regexp.MustCompile(`(^|[-_.])(prd|prod|production)([-_.]|$)`)

func main() {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		log.Fatalf("connecting to session bus: %v", err)
	}
	defer conn.Close()

	menu, err := tray.NewMenu(conn, menuObjectPath)
	if err != nil {
		log.Fatalf("exporting menu: %v", err)
	}

	regularIcon := icon.Render(icon.Blue)
	prodIcon := icon.Render(icon.Red)

	item, err := tray.NewItem(conn, appID, "Kubernetes context", regularIcon, menu)
	if err != nil {
		log.Fatalf("exporting tray item: %v", err)
	}

	// State is shared by D-Bus callbacks, the watcher and the session
	// ticker. render only reads it, so it never waits on external processes.
	var (
		mu               sync.Mutex
		cfg              kubecontext.Config
		loadErr          error
		warning          string
		entryToContext   map[int32]string // dbusmenu ids are int32, contexts are keyed by name
		autostartEntryID int32
		quitEntryID      int32
		lastCurrent      = "\x00" // never a valid context name, forces the first status update
	)

	// render rebuilds the menu from state; callers must hold mu.
	render := func() {
		entryToContext = make(map[int32]string)
		var entries []tray.MenuEntry
		nextID := int32(1) // 0 is reserved for the layout root
		add := func(e tray.MenuEntry) int32 {
			e.ID = nextID
			nextID++
			entries = append(entries, e)
			return e.ID
		}

		switch {
		case loadErr != nil:
			add(tray.MenuEntry{Header: true, Label: "Could not read kubeconfig (see logs)"})
		case len(cfg.Contexts) == 0:
			add(tray.MenuEntry{Header: true, Label: "No contexts in kubeconfig"})
		default:
			for _, group := range groupByProject(cfg.Contexts) {
				if group.title != "" {
					add(tray.MenuEntry{Header: true, Label: group.title})
				}
				for _, c := range group.contexts {
					id := add(tray.MenuEntry{Label: menuLabel(c), Checked: c.Name == cfg.Current})
					entryToContext[id] = c.Name
				}
			}
		}

		if warning != "" {
			add(tray.MenuEntry{Separator: true})
			add(tray.MenuEntry{Label: warning, ToggleType: "none"})
		}

		add(tray.MenuEntry{Separator: true})
		autostartEnabled, _ := autostart.IsEnabled()
		autostartEntryID = add(tray.MenuEntry{
			Label:      "Launch at login",
			ToggleType: "checkmark",
			Checked:    autostartEnabled,
		})
		quitEntryID = add(tray.MenuEntry{Label: "Quit", ToggleType: "none"})

		menu.SetEntries(entries)

		if current := cfg.Current; current != lastCurrent {
			pixmaps := regularIcon
			if prodPattern.MatchString(current) {
				pixmaps = prodIcon
			}
			item.SetStatus(statusTitle(current), current, pixmaps)
			lastCurrent = current
		}
	}

	checkSession := func() {
		mu.Lock()
		current, _ := cfg.Find(cfg.Current)
		mu.Unlock()

		w := kubecontext.SessionWarning(current)

		mu.Lock()
		defer mu.Unlock()
		// Drop the result if the context changed mid-check; that change
		// triggers its own check.
		if cfg.Current == current.Name {
			warning = w
			render()
		}
	}

	reload := func() {
		c, err := kubecontext.Load()
		if err != nil {
			log.Printf("reading kubeconfig: %v", err)
		}
		mu.Lock()
		cfg, loadErr = c, err
		render()
		mu.Unlock()
		checkSession()
	}

	menu.OnSelect = func(entryID int32) {
		mu.Lock()
		defer mu.Unlock()

		name, isContext := entryToContext[entryID]
		switch {
		case isContext:
			// Move the radio mark right away; kubectl takes ~1s.
			cfg.Current = name
			render()
			go func() {
				if err := kubecontext.Use(name); err != nil {
					log.Printf("switching to context %s: %v", name, err)
				}
				reload()
			}()
		case entryID == autostartEntryID:
			if enabled, _ := autostart.IsEnabled(); enabled {
				if err := autostart.Disable(); err != nil {
					log.Printf("disabling autostart: %v", err)
				}
			} else {
				if err := autostart.Enable(); err != nil {
					log.Printf("enabling autostart: %v", err)
				}
			}
			render()
		case entryID == quitEntryID:
			// Dropping the bus connection makes the watcher remove the icon.
			os.Exit(0)
		}
	}

	reload()
	go kubecontext.Watch(watchInterval, reload)
	go func() {
		for range time.Tick(sessionInterval) {
			checkSession()
		}
	}()

	fmt.Fprintln(os.Stderr, "gke-context-switcher running; waiting for tray host to display the icon")
	select {} // block forever; all work happens in D-Bus callbacks and the watcher
}

type contextGroup struct {
	title    string // GCP project id, "Other", or "" when there's only one ungrouped list
	contexts []kubecontext.Context
}

// groupByProject sorts GKE contexts under their GCP project (projects
// alphabetically, clusters alphabetically within each), followed by
// any non-GKE contexts under "Other". If nothing is a GKE context the
// list is returned as a single untitled group.
func groupByProject(contexts []kubecontext.Context) []contextGroup {
	byProject := make(map[string][]kubecontext.Context)
	var projects []string
	var other []kubecontext.Context
	for _, c := range contexts {
		if !c.IsGKE() {
			other = append(other, c)
			continue
		}
		if _, seen := byProject[c.Project]; !seen {
			projects = append(projects, c.Project)
		}
		byProject[c.Project] = append(byProject[c.Project], c)
	}
	sort.Strings(projects)
	sort.Slice(other, func(i, j int) bool { return other[i].Name < other[j].Name })

	var groups []contextGroup
	for _, p := range projects {
		cs := byProject[p]
		sort.Slice(cs, func(i, j int) bool {
			if cs[i].Cluster != cs[j].Cluster {
				return cs[i].Cluster < cs[j].Cluster
			}
			return cs[i].Location < cs[j].Location
		})
		groups = append(groups, contextGroup{title: p, contexts: cs})
	}
	if len(other) > 0 {
		title := "Other"
		if len(groups) == 0 {
			title = ""
		}
		groups = append(groups, contextGroup{title: title, contexts: other})
	}
	return groups
}

// menuLabel shows GKE contexts as "cluster · location" (the project is
// already the section header) and anything else by its raw name.
func menuLabel(c kubecontext.Context) string {
	if c.IsGKE() {
		return c.Cluster + "  ·  " + c.Location
	}
	return c.Name
}

// statusTitle is the tray title/tooltip headline for the current context.
func statusTitle(current string) string {
	if current == "" {
		return "Kubernetes: no context"
	}
	if c := kubecontext.Parse(current); c.IsGKE() {
		return "Kubernetes: " + c.Cluster + " (" + c.Project + ")"
	}
	return "Kubernetes: " + current
}
