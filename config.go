package main

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Server        ServerConfig  `yaml:"server"`
	CheckInterval time.Duration `yaml:"check_interval"`
	Endpoints     []Endpoint    `yaml:"endpoints"`
	Alerting      Alerting      `yaml:"alerting"`
}

// ServerConfig represents web server configuration
type ServerConfig struct {
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"`
}

// Endpoint represents a monitored endpoint
type Endpoint struct {
	Name             string            `yaml:"name"`
	URL              string            `yaml:"url"`
	Method           string            `yaml:"method"`
	Timeout          time.Duration     `yaml:"timeout"`
	ExpectedStatus   int               `yaml:"expected_status"`
	Headers          map[string]string `yaml:"headers"`
	FailureThreshold int               `yaml:"failure_threshold"`
	SuccessThreshold int               `yaml:"success_threshold"`
}

// Alerting represents alerting configuration
type Alerting struct {
	Enabled      bool              `yaml:"enabled"`
	TeamsEnabled bool              `yaml:"teams_enabled"`
	TeamsWebhook string            `yaml:"teams_webhook"`
	WebhookURL   string            `yaml:"webhook_url"`
	EmailEnabled bool              `yaml:"email_enabled"`
	EmailConfig  EmailConfig       `yaml:"email_config"`
	SlackEnabled bool              `yaml:"slack_enabled"`
	SlackWebhook string            `yaml:"slack_webhook"`
	CustomFields map[string]string `yaml:"custom_fields"`
}

// EmailConfig represents email configuration
type EmailConfig struct {
	SMTPHost string   `yaml:"smtp_host"`
	SMTPPort int      `yaml:"smtp_port"`
	From     string   `yaml:"from"`
	To       []string `yaml:"to"`
	Username string   `yaml:"username"`
	Password string   `yaml:"password"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if config.CheckInterval == 0 {
		config.CheckInterval = 30 * time.Second
	}
	
	if config.Server.Port == 0 {
		config.Server.Port = 8080
	}

	for i := range config.Endpoints {
		if config.Endpoints[i].Method == "" {
			config.Endpoints[i].Method = "GET"
		}
		if config.Endpoints[i].Timeout == 0 {
			config.Endpoints[i].Timeout = 10 * time.Second
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
