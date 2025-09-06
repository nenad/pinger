package main

import (
	"flag"
	"time"

	ui "github.com/nenad/pinger/internal/ui"
	pinger "github.com/nenad/pinger/internal/ping"
)

func main() {
	target := flag.String("target", "1.1.1.1", "Target address to ping")
	interval := flag.Duration("interval", time.Second, "Ping interval")
	timeout := flag.Duration("timeout", 2*time.Second, "Ping timeout")
	flag.Parse()

	mgr := pinger.NewManager(*target, *interval, *timeout, 60)
	app := ui.NewTrayApp(mgr, *target)
	app.Run()
}
