package ui

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/getlantern/systray"

	"github.com/nenad/pinger/internal/config"
	renderer "github.com/nenad/pinger/internal/icon"
	pinger "github.com/nenad/pinger/internal/ping"
)

type TrayApp struct {
	mgr              *pinger.Manager
	cfg              *config.Config
	targetLabel      *systray.MenuItem
	menuItems        []*systray.MenuItem
	changeTargetItem *systray.MenuItem
	icmpModeItem     *systray.MenuItem
	httpModeItem     *systray.MenuItem
	quitItem         *systray.MenuItem
	lastIcon         []byte
	iconMu           sync.Mutex
}

func NewTrayApp(mgr *pinger.Manager, cfg *config.Config) *TrayApp {
	return &TrayApp{mgr: mgr, cfg: cfg}
}

func (a *TrayApp) Run() {
	systray.Run(a.onReady, a.onExit)
}

func (a *TrayApp) onReady() {
	a.mgr.Start()
	a.updateTooltip()

	// Target label (read-only)
	a.targetLabel = systray.AddMenuItem("", "Current target")
	a.targetLabel.Disable()
	a.updateTargetLabel()
	systray.AddSeparator()

	// Configuration menu
	a.changeTargetItem = systray.AddMenuItem("Change Target...", "Change ping target")
	systray.AddSeparator()

	// Probe mode submenu
	a.icmpModeItem = systray.AddMenuItem("ICMP Mode", "Use ICMP ping")
	a.httpModeItem = systray.AddMenuItem("HTTP Mode", "Use HTTP probe (port 80)")
	a.updateModeCheckmarks()

	systray.AddSeparator()

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

	// Handle change target
	go func() {
		for range a.changeTargetItem.ClickedCh {
			a.handleChangeTarget()
		}
	}()

	// Handle ICMP mode
	go func() {
		for range a.icmpModeItem.ClickedCh {
			a.handleChangeMode(config.ProbeModeICMP)
		}
	}()

	// Handle HTTP mode
	go func() {
		for range a.httpModeItem.ClickedCh {
			a.handleChangeMode(config.ProbeModeHTTP)
		}
	}()

	// Handle quit
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

func (a *TrayApp) updateTooltip() {
	mode := a.mgr.ProbeMode()
	target := a.mgr.Target()
	systray.SetTooltip(fmt.Sprintf("Pinger [%s] → %s", mode, target))
}

func (a *TrayApp) updateTargetLabel() {
	mode := a.mgr.ProbeMode()
	target := a.mgr.Target()
	a.targetLabel.SetTitle(fmt.Sprintf("[%s] → %s", mode, target))
}

func (a *TrayApp) updateModeCheckmarks() {
	mode := a.mgr.ProbeMode()
	if mode == config.ProbeModeICMP {
		a.icmpModeItem.Check()
		a.httpModeItem.Uncheck()
	} else {
		a.icmpModeItem.Uncheck()
		a.httpModeItem.Check()
	}
}

func (a *TrayApp) handleChangeTarget() {
	currentTarget := a.mgr.Target()
	newTarget := a.showInputDialog("Change Target", "Enter new target address:", currentTarget)
	if newTarget == "" || newTarget == currentTarget {
		return
	}

	a.cfg.Target = newTarget
	if err := a.cfg.Save(); err != nil {
		// Silently fail, but could show notification
		return
	}

	a.mgr.SetTarget(newTarget)
	a.mgr.Restart()
	a.updateTooltip()
	a.updateTargetLabel()
}

func (a *TrayApp) handleChangeMode(mode config.ProbeMode) {
	if a.mgr.ProbeMode() == mode {
		return // Already in this mode
	}

	a.cfg.ProbeMode = mode
	if err := a.cfg.Save(); err != nil {
		// Silently fail
		return
	}

	a.mgr.SetProbeMode(mode)
	a.mgr.Restart()
	a.updateModeCheckmarks()
	a.updateTooltip()
	a.updateTargetLabel()
}

func (a *TrayApp) showInputDialog(title, prompt, defaultValue string) string {
	// Use osascript to show a native macOS dialog
	script := fmt.Sprintf(`display dialog "%s" default answer "%s" with title "%s" buttons {"Cancel", "OK"} default button "OK"`,
		prompt, defaultValue, title)

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	// Parse output: "button returned:OK, text returned:value"
	result := string(output)
	if !strings.Contains(result, "text returned:") {
		return ""
	}

	parts := strings.Split(result, "text returned:")
	if len(parts) < 2 {
		return ""
	}

	return strings.TrimSpace(parts[1])
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
