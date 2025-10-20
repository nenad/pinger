package main

import (
	"flag"
	"time"

	pinger "github.com/nenad/pinger/internal/ping"
	"github.com/nenad/pinger/internal/ui"
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
