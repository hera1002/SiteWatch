package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ashanmugaraja/cronzee/app/structs"
)

// LoadConfig loads configuration from a JSON file
func LoadConfig(filename string) (*structs.Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config structs.Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if config.CheckInterval.Duration == 0 {
		config.CheckInterval.Duration = 30 * time.Second
	}
	
	if config.Server.Port == 0 {
		config.Server.Port = 8080
	}

	// Default SSL expiry warning to 30 days if not set
	if config.SSLExpiryWarningDays == 0 {
		config.SSLExpiryWarningDays = 30
	}

	// Default SSL summary time to 09:30 if not set
	if config.SSLSummaryTime == "" {
		config.SSLSummaryTime = "09:30"
	}

	for i := range config.Endpoints {
		if config.Endpoints[i].Method == "" {
			config.Endpoints[i].Method = "GET"
		}
		if config.Endpoints[i].Timeout.Duration == 0 {
			config.Endpoints[i].Timeout.Duration = 10 * time.Second
		}
		if config.Endpoints[i].ExpectedStatus == 0 {
			config.Endpoints[i].ExpectedStatus = 200
		}
		if config.Endpoints[i].FailureThreshold == 0 {
			config.Endpoints[i].FailureThreshold = 3
		}
		if config.Endpoints[i].SuccessThreshold == 0 {
			config.Endpoints[i].SuccessThreshold = 2
		}
	}

	return &config, nil
}
