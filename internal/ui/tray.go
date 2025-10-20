package ui

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"github.com/getlantern/systray"

	renderer "github.com/nenad/pinger/internal/icon"
	pinger "github.com/nenad/pinger/internal/ping"
)

type TrayApp struct {
	mgr       *pinger.Manager
	menuItems []*systray.MenuItem
	quitItem  *systray.MenuItem
	target    string
	lastIcon  []byte
	iconMu    sync.Mutex
}

func NewTrayApp(mgr *pinger.Manager, target string) *TrayApp {
	return &TrayApp{mgr: mgr, target: target}
}

func (a *TrayApp) Run() {
	systray.Run(a.onReady, a.onExit)
}

func (a *TrayApp) onReady() {
	a.mgr.Start()
	systray.SetTooltip(fmt.Sprintf("Pinger → %s", a.target))
	// Initialize menu items for latest 20 pings
	a.menuItems = make([]*systray.MenuItem, 20)
	for i := 0; i < 20; i++ {
		item := systray.AddMenuItem("…", "Ping sample")
		a.menuItems[i] = item
		go func(mi *systray.MenuItem) {
			for range mi.ClickedCh {
				// Selecting a menu item closes the popup on macOS; do nothing.
			}
		}(item)
	}
	systray.AddSeparator()
	a.quitItem = systray.AddMenuItem("Quit", "Quit Pinger")
	go func() {
		<-a.quitItem.ClickedCh
		systray.Quit()
	}()

	// Initial icon
	a.updateIcon(0)
	a.updateMenu()

	// Update on results
	go func() {
		for range a.mgr.Results() {
			a.updateMenu()
			// a.updateIcon(0)
		}
	}()

	// On entering in-flight, update once using the elapsed age to influence background
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			_, age := a.mgr.IsInFlight()
			a.updateIcon(age.Milliseconds())
		}
	}()
}

func (a *TrayApp) onExit() {
	a.mgr.Stop()
}

func (a *TrayApp) updateMenu() {
	latest := a.mgr.History().Latest(20)
	// latest returns most recent first; map to menu top→newest
	for i := 0; i < 20; i++ {
		label := "—"
		if i < len(latest) {
			s := latest[i]
			if s.Failed {
				label = fmt.Sprintf("%s  FAIL", s.Timestamp.Format("15:04:05"))
			} else {
				label = fmt.Sprintf("%s  %d ms", s.Timestamp.Format("15:04:05"), s.Latency.Milliseconds())
			}
		}
		a.menuItems[i].SetTitle(label)
	}
}

func (a *TrayApp) updateIcon(inflightAge int64) {
	png := renderer.Render(a.mgr.History(), inflightAge)
	a.iconMu.Lock()
	defer a.iconMu.Unlock()
	if bytes.Equal(a.lastIcon, png) {
		return
	}
	a.lastIcon = png
	systray.SetTemplateIcon(png, png)
}
