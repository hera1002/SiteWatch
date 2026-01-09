package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/ashanmugaraja/cronzee/app/logger"
	"github.com/ashanmugaraja/cronzee/app/models"
	"github.com/ashanmugaraja/cronzee/app/structs"
	"github.com/ashanmugaraja/cronzee/app/utils"
	"github.com/ashanmugaraja/cronzee/app/worker"
)

// HealthHandler handles health check related endpoints
type HealthHandler struct {
	monitor *worker.Monitor
	db      *models.Database
	config  *structs.Config
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(monitor *worker.Monitor, db *models.Database, config *structs.Config) *HealthHandler {
	return &HealthHandler{
		monitor: monitor,
		db:      db,
		config:  config,
	}
}

// GetStatus returns the current status of all endpoints
func (h *HealthHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	states := h.monitor.GetStatus()

	response := map[string]interface{}{
		"endpoints": make(map[string]interface{}),
		"timestamp": time.Now(),
	}

	endpoints := make(map[string]interface{})
	for name, state := range states {
		endpointData := map[string]interface{}{
			"id":                    state.ID,
			"name":                  state.Endpoint.Name,
			"url":                   state.Endpoint.URL,
			"method":                state.Endpoint.Method,
			"status":                string(state.Status),
			"last_check":            state.LastCheck.Format(time.RFC3339),
			"last_success":          state.LastSuccess.Format(time.RFC3339),
			"last_error":            state.LastError,
			"response_time_ms":      float64(state.ResponseTime.Microseconds()) / 1000.0,
			"consecutive_failures":  state.ConsecutiveFailures,
			"consecutive_successes": state.ConsecutiveSuccesses,
			"ssl_expiring_soon":     state.SSLExpiringSoon,
			"days_to_expiry":        state.DaysToExpiry,
		}

		// Add SSL expiry date if available
		if !state.SSLCertExpiry.IsZero() {
			endpointData["ssl_cert_expiry"] = state.SSLCertExpiry.Format(time.RFC3339)
		}

		endpoints[name] = endpointData
	}
	response["endpoints"] = endpoints

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetEndpoints returns all endpoints from the database
func (h *HealthHandler) GetEndpoints(w http.ResponseWriter, r *http.Request) {
	endpoints, err := h.db.GetAllEndpoints()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"endpoints": endpoints,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// GetExpiringCerts returns list of endpoints with expiring SSL certificates
func (h *HealthHandler) GetExpiringCerts(w http.ResponseWriter, r *http.Request) {
	states := h.monitor.GetStatus()

	expiringCerts := []map[string]interface{}{}

	for _, state := range states {
		if state.SSLExpiringSoon {
			certInfo := map[string]interface{}{
				"id":             state.ID,
				"name":           state.Endpoint.Name,
				"url":            state.Endpoint.URL,
				"days_to_expiry": state.DaysToExpiry,
			}

			if !state.SSLCertExpiry.IsZero() {
				certInfo["expiry_date"] = state.SSLCertExpiry.Format(time.RFC3339)
			}

			expiringCerts = append(expiringCerts, certInfo)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"expiring_certs": expiringCerts,
		"count":          len(expiringCerts),
		"timestamp":      time.Now().Format(time.RFC3339),
	})
}

// GetHistory returns health check history for an endpoint
func (h *HealthHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Endpoint ID is required", http.StatusBadRequest)
		return
	}

	limit := 1000
	records, err := h.db.GetHealthHistory(id, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Calculate average response time
	var totalResponseTime int64
	var count int
	for _, r := range records {
		if r.ResponseTime > 0 {
			totalResponseTime += int64(r.ResponseTime)
			count++
		}
	}
	var avgResponseTimeMs float64
	if count > 0 {
		avgResponseTimeMs = float64(totalResponseTime/int64(count)) / 1000000.0
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"endpoint_id":          id,
		"records":              records,
		"avg_response_time_ms": avgResponseTimeMs,
		"record_count":         count,
		"timestamp":            time.Now().Format(time.RFC3339),
	})
}

// AddEndpoint adds a new endpoint
func (h *HealthHandler) AddEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name             string            `json:"name"`
		URL              string            `json:"url"`
		MonitorHealth    bool              `json:"monitor_health"`
		Method           string            `json:"method"`
		Timeout          string            `json:"timeout"`
		CheckInterval    string            `json:"check_interval"`
		ExpectedStatus   int               `json:"expected_status"`
		Headers          map[string]string `json:"headers"`
		FailureThreshold int               `json:"failure_threshold"`
		SuccessThreshold int               `json:"success_threshold"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.URL == "" {
		http.Error(w, "Name and URL are required", http.StatusBadRequest)
		return
	}

	// Validate and normalize URL format (from oldfiles/server.go logic)
	// Ensure URL has proper scheme format with ://
	if !strings.Contains(req.URL, "://") {
		http.Error(w, "Invalid URL format: must include protocol (e.g., https://)", http.StatusBadRequest)
		return
	}

	// Check if endpoint with same name or URL already exists
	allEndpoints, err := h.db.GetAllEndpoints()
	if err != nil {
		http.Error(w, "Failed to check existing endpoints: "+err.Error(), http.StatusInternalServerError)
		return
	}

	for _, ep := range allEndpoints {
		if ep.Name == req.Name {
			http.Error(w, "Endpoint with this name already exists", http.StatusConflict)
			return
		}
		if ep.URL == req.URL {
			http.Error(w, "Endpoint with this URL already exists", http.StatusConflict)
			return
		}
	}

	timeout := 10 * time.Second
	if req.Timeout != "" && req.MonitorHealth {
		var err error
		timeout, err = time.ParseDuration(req.Timeout)
		if err != nil {
			http.Error(w, "Invalid timeout format: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	// If health monitoring is disabled, set check interval to 0
	var checkInterval time.Duration
	if req.MonitorHealth {
		checkInterval = 30 * time.Second
		if req.CheckInterval != "" {
			var err error
			checkInterval, err = time.ParseDuration(req.CheckInterval)
			if err != nil {
				http.Error(w, "Invalid check_interval format: "+err.Error(), http.StatusBadRequest)
				return
			}
		}
	}

	endpoint := &structs.StoredEndpoint{
		ID:               utils.GenerateIDWithURL(req.Name, req.URL),
		Name:             req.Name,
		URL:              req.URL,
		Method:           req.Method,
		Timeout:          timeout,
		CheckInterval:    checkInterval,
		ExpectedStatus:   req.ExpectedStatus,
		Headers:          req.Headers,
		FailureThreshold: req.FailureThreshold,
		SuccessThreshold: req.SuccessThreshold,
		Enabled:          true,
		AlertsSuppressed: false,
		MonitorHealth:    req.MonitorHealth,
	}

	if err := h.monitor.AddEndpoint(endpoint); err != nil {
		logger.Errorf("Failed to add endpoint: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"endpoint": endpoint,
	})
}

// DeleteEndpoint removes an endpoint from monitoring
func (h *HealthHandler) DeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	logger.Debugf("Delete endpoint request: method=%s", r.Method)

	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		logger.Debugf("Delete endpoint: method not allowed")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	logger.Debugf("Delete endpoint: query id=%s", id)

	if id == "" {
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			id = req.ID
			logger.Debugf("Delete endpoint: body id=%s", id)
		} else {
			logger.Debugf("Delete endpoint: body decode error=%v", err)
		}
	}

	if id == "" {
		logger.Debugf("Delete endpoint: ID is empty")
		http.Error(w, "Endpoint ID is required", http.StatusBadRequest)
		return
	}

	logger.Debugf("Delete endpoint: attempting to remove id=%s", id)
	if err := h.monitor.RemoveEndpoint(id); err != nil {
		logger.Errorf("Delete endpoint: error=%v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	logger.Infof("Delete endpoint: success id=%s", id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Endpoint deleted",
	})
}

// EnableEndpoint enables an endpoint
func (h *HealthHandler) EnableEndpoint(w http.ResponseWriter, r *http.Request) {
	h.handleEndpointAction(w, r, h.monitor.EnableEndpoint, "enabled")
}

// DisableEndpoint disables an endpoint
func (h *HealthHandler) DisableEndpoint(w http.ResponseWriter, r *http.Request) {
	h.handleEndpointAction(w, r, h.monitor.DisableEndpoint, "disabled")
}

// SuppressAlerts suppresses alerts for an endpoint
func (h *HealthHandler) SuppressAlerts(w http.ResponseWriter, r *http.Request) {
	h.handleEndpointAction(w, r, h.monitor.SuppressAlerts, "alerts suppressed")
}

// UnsuppressAlerts enables alerts for an endpoint
func (h *HealthHandler) UnsuppressAlerts(w http.ResponseWriter, r *http.Request) {
	h.handleEndpointAction(w, r, h.monitor.UnsuppressAlerts, "alerts enabled")
}

// handleEndpointAction is a helper for endpoint actions
func (h *HealthHandler) handleEndpointAction(w http.ResponseWriter, r *http.Request, action func(string) error, actionName string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			id = req.ID
		}
	}

	if id == "" {
		http.Error(w, "Endpoint ID is required", http.StatusBadRequest)
		return
	}

	if err := action(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Endpoint " + actionName,
	})
}

// ToggleEndpoint enables or disables an endpoint (deprecated, kept for compatibility)
func (h *HealthHandler) ToggleEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var err error
	if req.Enabled {
		err = h.monitor.EnableEndpoint(req.ID)
	} else {
		err = h.monitor.DisableEndpoint(req.ID)
	}

	if err != nil {
		logger.Errorf("Failed to toggle endpoint: %v", err)
		http.Error(w, "Failed to toggle endpoint: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Endpoint toggled successfully",
	})
}

// ToggleAlerts toggles alert suppression for an endpoint (deprecated, kept for compatibility)
func (h *HealthHandler) ToggleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID         string `json:"id"`
		Suppressed bool   `json:"suppressed"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var err error
	if req.Suppressed {
		err = h.monitor.SuppressAlerts(req.ID)
	} else {
		err = h.monitor.UnsuppressAlerts(req.ID)
	}

	if err != nil {
		logger.Errorf("Failed to toggle alerts: %v", err)
		http.Error(w, "Failed to toggle alerts: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Alerts toggled successfully",
	})
}

// UpdateEndpoint updates endpoint settings
func (h *HealthHandler) UpdateEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID               string `json:"id"`
		CheckInterval    string `json:"check_interval"`
		Timeout          string `json:"timeout"`
		FailureThreshold int    `json:"failure_threshold"`
		SuccessThreshold int    `json:"success_threshold"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	endpoint, err := h.db.GetEndpoint(req.ID)
	if err != nil {
		http.Error(w, "Endpoint not found", http.StatusNotFound)
		return
	}

	// Update fields if provided
	if req.CheckInterval != "" {
		interval, err := time.ParseDuration(req.CheckInterval)
		if err != nil {
			http.Error(w, "Invalid check_interval format: "+err.Error(), http.StatusBadRequest)
			return
		}
		endpoint.CheckInterval = interval
	}
	if req.Timeout != "" {
		timeout, err := time.ParseDuration(req.Timeout)
		if err != nil {
			http.Error(w, "Invalid timeout format: "+err.Error(), http.StatusBadRequest)
			return
		}
		endpoint.Timeout = timeout
	}
	if req.FailureThreshold > 0 {
		endpoint.FailureThreshold = req.FailureThreshold
	}
	if req.SuccessThreshold > 0 {
		endpoint.SuccessThreshold = req.SuccessThreshold
	}

	if err := h.db.SaveEndpoint(endpoint); err != nil {
		logger.Errorf("Failed to update endpoint: %v", err)
		http.Error(w, "Failed to update endpoint", http.StatusInternalServerError)
		return
	}

	h.monitor.UpdateEndpointSettings(req.ID, endpoint)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Endpoint updated successfully",
	})
}

// GetConfig returns public configuration settings
func (h *HealthHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ssl_expiry_warning_days": h.config.SSLExpiryWarningDays,
		"has_passkey":             h.config.AdminPasskey != "",
	})
}

// VerifyPasskey verifies the admin passkey
func (h *HealthHandler) VerifyPasskey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Passkey string `json:"passkey"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	valid := h.config.AdminPasskey != "" && req.Passkey == h.config.AdminPasskey

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"valid": valid,
	})
}

// EnableHealthMonitoring enables health monitoring for an endpoint (requires passkey)
func (h *HealthHandler) EnableHealthMonitoring(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID               string `json:"id"`
		Passkey          string `json:"passkey"`
		CheckInterval    string `json:"check_interval"`
		Timeout          string `json:"timeout"`
		ExpectedStatus   int    `json:"expected_status"`
		FailureThreshold int    `json:"failure_threshold"`
		SuccessThreshold int    `json:"success_threshold"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Verify passkey
	if h.config.AdminPasskey != "" && req.Passkey != h.config.AdminPasskey {
		http.Error(w, "Invalid passkey", http.StatusUnauthorized)
		return
	}

	endpoint, err := h.db.GetEndpoint(req.ID)
	if err != nil {
		http.Error(w, "Endpoint not found", http.StatusNotFound)
		return
	}

	// Update health monitoring settings
	endpoint.MonitorHealth = true

	if req.CheckInterval != "" {
		interval, err := time.ParseDuration(req.CheckInterval)
		if err != nil {
			http.Error(w, "Invalid check_interval format", http.StatusBadRequest)
			return
		}
		endpoint.CheckInterval = interval
	} else {
		endpoint.CheckInterval = 30 * time.Second
	}

	if req.Timeout != "" {
		timeout, err := time.ParseDuration(req.Timeout)
		if err != nil {
			http.Error(w, "Invalid timeout format", http.StatusBadRequest)
			return
		}
		endpoint.Timeout = timeout
	}

	if req.ExpectedStatus > 0 {
		endpoint.ExpectedStatus = req.ExpectedStatus
	}
	if req.FailureThreshold > 0 {
		endpoint.FailureThreshold = req.FailureThreshold
	}
	if req.SuccessThreshold > 0 {
		endpoint.SuccessThreshold = req.SuccessThreshold
	}

	if err := h.db.SaveEndpoint(endpoint); err != nil {
		http.Error(w, "Failed to update endpoint", http.StatusInternalServerError)
		return
	}

	h.monitor.EnableHealthMonitoring(req.ID, endpoint)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Health monitoring enabled",
	})
}
