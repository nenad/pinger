package main

import (
	"log"
	"time"

	"github.com/nenad/pinger/internal/config"
	pinger "github.com/nenad/pinger/internal/ping"
	"github.com/nenad/pinger/internal/ui"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Save initial config if it doesn't exist
	if err := cfg.Save(); err != nil {
		log.Printf("Warning: Failed to save config: %v", err)
	}

	// Create ping manager with config values
	interval := time.Second
	timeout := 2 * time.Second
	mgr := pinger.NewManager(cfg.Target, interval, timeout, cfg.ProbeMode, 60)

	// Create and run UI
	app := ui.NewTrayApp(mgr, cfg)
	app.Run()
}
