package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	configFile := flag.String("config", "config.yaml", "Path to configuration file")
	dbPath := flag.String("db", "cronzee.db", "Path to database file")
	flag.Parse()

	// Load configuration
	config, err := LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database
	db, err := NewDatabase(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Note: Endpoints are loaded only from database, not from config.yaml
	// Use the web UI to add/remove endpoints

	log.Printf("Starting Site Watch...")

	// Initialize monitor with database
	monitor := NewMonitor(config, db)

	// Count endpoints from database
	endpoints, _ := db.GetAllEndpoints()
	log.Printf("Monitoring %d endpoints with check interval: %s", len(endpoints), config.CheckInterval)

	// Start web server if enabled
	if config.Server.Enabled {
		server := NewServer(monitor, db, config.Server.Port)
		server.Start()
	}

	// Start monitoring
	monitor.Start()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down Site Watch...")
	monitor.Stop()
	time.Sleep(1 * time.Second)
}
