package structs

import (
	"encoding/json"
	"fmt"
	"time"
)

// Duration is a custom type for JSON unmarshaling of time.Duration
type Duration struct {
	time.Duration
}

// UnmarshalJSON implements json.Unmarshaler for Duration
func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		d.Duration = time.Duration(value)
		return nil
	case string:
		var err error
		d.Duration, err = time.ParseDuration(value)
		if err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("invalid duration")
	}
}

// Config represents the application configuration
type Config struct {
	Server              ServerConfig `json:"server"`
	CheckInterval       Duration     `json:"check_interval"`
	SSLExpiryWarningDays int         `json:"ssl_expiry_warning_days"`
	SSLSummaryTime      string       `json:"ssl_summary_time"`
	AdminPasskey        string       `json:"admin_passkey"`
	Endpoints           []Endpoint   `json:"endpoints"`
	Alerting            Alerting     `json:"alerting"`
}

// ServerConfig represents web server configuration
type ServerConfig struct {
	Enabled bool `json:"enabled"`
	Port    int  `json:"port"`
}

// Endpoint represents a monitored endpoint
type Endpoint struct {
	Name             string            `json:"name"`
	URL              string            `json:"url"`
	Method           string            `json:"method"`
	Timeout          Duration          `json:"timeout"`
	ExpectedStatus   int               `json:"expected_status"`
	Headers          map[string]string `json:"headers"`
	FailureThreshold int               `json:"failure_threshold"`
	SuccessThreshold int               `json:"success_threshold"`
}

// Alerting represents alerting configuration
type Alerting struct {
	Enabled      bool              `json:"enabled"`
	TeamsEnabled bool              `json:"teams_enabled"`
	TeamsWebhook string            `json:"teams_webhook"`
	WebhookURL   string            `json:"webhook_url"`
	EmailEnabled bool              `json:"email_enabled"`
	EmailConfig  EmailConfig       `json:"email_config"`
	SlackEnabled bool              `json:"slack_enabled"`
	SlackWebhook string            `json:"slack_webhook"`
	CustomFields map[string]string `json:"custom_fields"`
}

// EmailConfig represents email configuration
type EmailConfig struct {
	SMTPHost string   `json:"smtp_host"`
	SMTPPort int      `json:"smtp_port"`
	From     string   `json:"from"`
	To       []string `json:"to"`
	Username string   `json:"username"`
	Password string   `json:"password"`
}

// StoredEndpoint represents an endpoint stored in the database
type StoredEndpoint struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	URL              string            `json:"url"`
	Method           string            `json:"method"`
	Timeout          time.Duration     `json:"timeout"`
	CheckInterval    time.Duration     `json:"check_interval"`
	ExpectedStatus   int               `json:"expected_status"`
	Headers          map[string]string `json:"headers"`
	FailureThreshold int               `json:"failure_threshold"`
	SuccessThreshold int               `json:"success_threshold"`
	Enabled          bool              `json:"enabled"`
	AlertsSuppressed bool              `json:"alerts_suppressed"`
	MonitorHealth    bool              `json:"monitor_health"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

// HealthCheckRecord represents a single health check result stored in history
type HealthCheckRecord struct {
	EndpointID   string        `json:"endpoint_id"`
	Timestamp    time.Time     `json:"timestamp"`
	Status       string        `json:"status"`
	ResponseTime time.Duration `json:"response_time"`
	StatusCode   int           `json:"status_code"`
	Error        string        `json:"error,omitempty"`
}

// HealthStatus represents the health status of an endpoint
type HealthStatus string

const (
	StatusHealthy   HealthStatus = "healthy"
	StatusUnhealthy HealthStatus = "unhealthy"
	StatusUnknown   HealthStatus = "unknown"
)

// EndpointState tracks the state of a monitored endpoint
type EndpointState struct {
	Endpoint             Endpoint
	Status               HealthStatus
	LastCheck            time.Time
	LastStatusChange     time.Time
	ConsecutiveFailures  int
	ConsecutiveSuccesses int
	ResponseTime         time.Duration
	LastError            string
	Enabled              bool
	AlertsSuppressed     bool
	MonitorHealth        bool
	ID                   string
	CheckInterval        time.Duration
	NextCheck            time.Time
	SSLCertExpiry        time.Time
	SSLExpiringSoon      bool
	DaysToExpiry         int
	LastSSLCheck         time.Time // Track when SSL was last validated (for daily check)
}

// ToEndpoint converts StoredEndpoint to Endpoint for monitoring
func (s *StoredEndpoint) ToEndpoint() Endpoint {
	return Endpoint{
		Name:             s.Name,
		URL:              s.URL,
		Method:           s.Method,
		Timeout:          Duration{Duration: s.Timeout},
		ExpectedStatus:   s.ExpectedStatus,
		Headers:          s.Headers,
		FailureThreshold: s.FailureThreshold,
		SuccessThreshold: s.SuccessThreshold,
	}
}
