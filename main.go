package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ashanmugaraja/cronzee/app/config"
	"github.com/ashanmugaraja/cronzee/app/logger"
	"github.com/ashanmugaraja/cronzee/app/models"
	"github.com/ashanmugaraja/cronzee/app/router"
	"github.com/ashanmugaraja/cronzee/app/worker"
)

func main() {
	// Initialize logger
	logger.Init()

	// Parse command-line flags
	configFile := flag.String("config", "config.json", "Path to configuration file")
	dbPath := flag.String("db", "sitewatch.db", "Path to database file")
	flag.Parse()

	logger.Infof("Starting Site Watch...")

	// Load configuration
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		logger.Errorf("Failed to load configuration: %v", err)
		os.Exit(1)
	}

	// Initialize database
	db, err := models.NewDatabase(*dbPath)
	if err != nil {
		logger.Errorf("Failed to initialize database: %v", err)
		os.Exit(1)
	}
	defer db.Close()

	// Initialize monitor
	monitor := worker.NewMonitor(cfg, db)

	// Count endpoints from database
	endpoints, _ := db.GetAllEndpoints()
	logger.Infof("Monitoring %d endpoints with check interval: %s", len(endpoints), cfg.CheckInterval.Duration)

	// Start monitoring
	monitor.Start()

	// Start web server if enabled
	if cfg.Server.Enabled {
		r := router.NewRouter(monitor, db, cfg)
		addr := fmt.Sprintf(":%d", cfg.Server.Port)
		
		server := &http.Server{
			Addr:    addr,
			Handler: r,
		}

		go func() {
			logger.Infof("Web server starting on http://localhost%s", addr)
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Errorf("Server error: %v", err)
			}
		}()
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Infof("Shutting down Site Watch...")
	monitor.Stop()
	time.Sleep(1 * time.Second)
	logger.Infof("Shutdown complete")
}
