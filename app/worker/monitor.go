package worker

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/ashanmugaraja/cronzee/app/logger"
	"github.com/ashanmugaraja/cronzee/app/models"
	"github.com/ashanmugaraja/cronzee/app/structs"
)

// Monitor manages health checks for multiple endpoints
type Monitor struct {
	config  *structs.Config
	states  map[string]*MonitorState
	alerter *Alerter
	db      *models.Database
	ticker  *time.Ticker
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.RWMutex
}

// MonitorState tracks the state of a monitored endpoint with mutex
type MonitorState struct {
	*structs.EndpointState
	mu sync.RWMutex
}

// NewMonitor creates a new health monitor
func NewMonitor(config *structs.Config, db *models.Database) *Monitor {
	ctx, cancel := context.WithCancel(context.Background())
	
	monitor := &Monitor{
		config:  config,
		states:  make(map[string]*MonitorState),
		alerter: NewAlerter(&config.Alerting),
		db:      db,
		ctx:     ctx,
		cancel:  cancel,
	}

	// Initialize endpoint states from database
	monitor.loadEndpointsFromDB()

	return monitor
}

// loadEndpointsFromDB loads endpoints from the database
func (m *Monitor) loadEndpointsFromDB() {
	m.mu.Lock()
	defer m.mu.Unlock()

	endpoints, err := m.db.GetAllEndpoints()
	if err != nil {
		logger.Errorf("Error loading endpoints from database: %v", err)
		return
	}

	for _, stored := range endpoints {
		checkInterval := stored.CheckInterval
		if checkInterval == 0 && stored.MonitorHealth {
			checkInterval = m.config.CheckInterval.Duration
		}
		m.states[stored.ID] = &MonitorState{
			EndpointState: &structs.EndpointState{
				ID:               stored.ID,
				Endpoint:         stored.ToEndpoint(),
				Status:           structs.StatusUnknown,
				LastCheck:        time.Now(),
				Enabled:          stored.Enabled,
				AlertsSuppressed: stored.AlertsSuppressed,
				MonitorHealth:    stored.MonitorHealth,
				CheckInterval:    checkInterval,
				NextCheck:        time.Now(),
			},
		}
	}
}

// ReloadEndpoints reloads endpoints from the database
func (m *Monitor) ReloadEndpoints() {
	m.loadEndpointsFromDB()
	logger.Infof("Reloaded %d endpoints from database", len(m.states))
}

// AddEndpoint adds a new endpoint to monitoring
func (m *Monitor) AddEndpoint(stored *structs.StoredEndpoint) error {
	if err := m.db.SaveEndpoint(stored); err != nil {
		return err
	}

	checkInterval := stored.CheckInterval
	if checkInterval == 0 && stored.MonitorHealth {
		checkInterval = m.config.CheckInterval.Duration
	}

	m.mu.Lock()
	m.states[stored.ID] = &MonitorState{
		EndpointState: &structs.EndpointState{
			ID:               stored.ID,
			Endpoint:         stored.ToEndpoint(),
			Status:           structs.StatusUnknown,
			LastCheck:        time.Now(),
			Enabled:          stored.Enabled,
			AlertsSuppressed: stored.AlertsSuppressed,
			MonitorHealth:    stored.MonitorHealth,
			CheckInterval:    checkInterval,
			NextCheck:        time.Now(),
		},
	}
	m.mu.Unlock()

	logger.Infof("Added endpoint: %s", stored.Name)
	return nil
}

// RemoveEndpoint removes an endpoint from monitoring
func (m *Monitor) RemoveEndpoint(id string) error {
	logger.Debugf("RemoveEndpoint called with id: %s", id)
	
	if err := m.db.DeleteEndpoint(id); err != nil {
		logger.Errorf("Error deleting from DB: %v", err)
		return err
	}

	m.mu.Lock()
	delete(m.states, id)
	m.mu.Unlock()

	logger.Infof("Removed endpoint: %s", id)
	return nil
}

// EnableEndpoint enables monitoring for an endpoint
func (m *Monitor) EnableEndpoint(id string) error {
	if err := m.db.EnableEndpoint(id); err != nil {
		return err
	}

	m.mu.Lock()
	if state, ok := m.states[id]; ok {
		state.mu.Lock()
		state.Enabled = true
		state.mu.Unlock()
	}
	m.mu.Unlock()

	logger.Infof("Enabled endpoint: %s", id)
	return nil
}

// DisableEndpoint disables monitoring for an endpoint
func (m *Monitor) DisableEndpoint(id string) error {
	if err := m.db.DisableEndpoint(id); err != nil {
		return err
	}

	m.mu.Lock()
	if state, ok := m.states[id]; ok {
		state.mu.Lock()
		state.Enabled = false
		state.mu.Unlock()
	}
	m.mu.Unlock()

	logger.Infof("Disabled endpoint: %s", id)
	return nil
}

// EnableHealthMonitoring enables health monitoring for an endpoint
func (m *Monitor) EnableHealthMonitoring(id string, stored *structs.StoredEndpoint) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, ok := m.states[id]; ok {
		state.mu.Lock()
		state.MonitorHealth = true
		state.CheckInterval = stored.CheckInterval
		state.Endpoint.Timeout.Duration = stored.Timeout
		state.Endpoint.ExpectedStatus = stored.ExpectedStatus
		state.Endpoint.FailureThreshold = stored.FailureThreshold
		state.Endpoint.SuccessThreshold = stored.SuccessThreshold
		state.NextCheck = time.Now()
		state.mu.Unlock()
		logger.Infof("Enabled health monitoring for endpoint: %s", id)
	}
}

// SuppressAlerts suppresses alerts for an endpoint
func (m *Monitor) SuppressAlerts(id string) error {
	if err := m.db.SuppressAlerts(id); err != nil {
		return err
	}

	m.mu.Lock()
	if state, ok := m.states[id]; ok {
		state.mu.Lock()
		state.AlertsSuppressed = true
		state.mu.Unlock()
	}
	m.mu.Unlock()

	logger.Infof("Suppressed alerts for endpoint: %s", id)
	return nil
}

// UpdateEndpointSettings updates endpoint settings in the monitor state
func (m *Monitor) UpdateEndpointSettings(id string, stored *structs.StoredEndpoint) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, ok := m.states[id]; ok {
		state.mu.Lock()
		state.Endpoint.Timeout = structs.Duration{Duration: stored.Timeout}
		state.Endpoint.FailureThreshold = stored.FailureThreshold
		state.Endpoint.SuccessThreshold = stored.SuccessThreshold
		state.CheckInterval = stored.CheckInterval
		state.mu.Unlock()
		logger.Infof("Updated endpoint settings: %s", id)
	}
}

// UnsuppressAlerts enables alerts for an endpoint
func (m *Monitor) UnsuppressAlerts(id string) error {
	if err := m.db.UnsuppressAlerts(id); err != nil {
		return err
	}

	m.mu.Lock()
	if state, ok := m.states[id]; ok {
		state.mu.Lock()
		state.AlertsSuppressed = false
		state.mu.Unlock()
	}
	m.mu.Unlock()

	logger.Infof("Unsuppressed alerts for endpoint: %s", id)
	return nil
}

// Start begins monitoring all endpoints
func (m *Monitor) Start() {
	m.ticker = time.NewTicker(5 * time.Second)
	
	// Perform initial check
	m.checkAllEndpoints()

	// Start periodic checks
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		for {
			select {
			case <-m.ctx.Done():
				return
			case <-m.ticker.C:
				m.checkDueEndpoints()
			}
		}
	}()

	// Start daily SSL expiry summary scheduler
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.startSSLExpirySummaryScheduler()
	}()
}

// Stop stops the monitor
func (m *Monitor) Stop() {
	if m.ticker != nil {
		m.ticker.Stop()
	}
	m.cancel()
	m.wg.Wait()
}

// checkAllEndpoints checks all configured endpoints
func (m *Monitor) checkAllEndpoints() {
	var wg sync.WaitGroup
	
	m.mu.RLock()
	for _, state := range m.states {
		state.mu.RLock()
		enabled := state.Enabled
		state.mu.RUnlock()
		
		if !enabled {
			continue
		}
		
		wg.Add(1)
		go func(s *MonitorState) {
			defer wg.Done()
			m.checkEndpoint(s)
		}(state)
	}
	m.mu.RUnlock()
	
	wg.Wait()
}

// checkDueEndpoints checks endpoints that are due for checking
func (m *Monitor) checkDueEndpoints() {
	var wg sync.WaitGroup
	now := time.Now()
	
	m.mu.RLock()
	for _, state := range m.states {
		state.mu.RLock()
		enabled := state.Enabled
		nextCheck := state.NextCheck
		state.mu.RUnlock()
		
		if !enabled || now.Before(nextCheck) {
			continue
		}
		
		wg.Add(1)
		go func(s *MonitorState) {
			defer wg.Done()
			m.checkEndpoint(s)
		}(state)
	}
	m.mu.RUnlock()
	
	wg.Wait()
}

// checkEndpoint performs a health check on a single endpoint
func (m *Monitor) checkEndpoint(state *MonitorState) {
	state.mu.RLock()
	monitorHealth := state.MonitorHealth
	url := state.Endpoint.URL
	state.mu.RUnlock()

	// If health monitoring is disabled, only check SSL certificate
	if !monitorHealth {
		m.checkSSLOnly(state, url)
		return
	}

	start := time.Now()
	
	state.mu.RLock()
	timeout := state.Endpoint.Timeout.Duration
	method := state.Endpoint.Method
	headers := state.Endpoint.Headers
	expectedStatus := state.Endpoint.ExpectedStatus
	state.mu.RUnlock()
	
	ctx, cancel := context.WithTimeout(m.ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		m.handleCheckFailure(state, fmt.Sprintf("failed to create request: %v", err), 0)
		return
	}

	// Add custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{
		Timeout: timeout,
	}

	resp, err := client.Do(req)
	responseTime := time.Since(start)

	if err != nil {
		m.handleCheckFailure(state, fmt.Sprintf("request failed: %v", err), responseTime)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != expectedStatus {
		m.handleCheckFailure(state, 
			fmt.Sprintf("unexpected status code: got %d, expected %d", resp.StatusCode, expectedStatus),
			responseTime)
		return
	}

	m.handleCheckSuccess(state, responseTime)
}

// checkSSLOnly checks only the SSL certificate for an endpoint (no health check)
func (m *Monitor) checkSSLOnly(state *MonitorState, url string) {
	state.mu.Lock()
	defer state.mu.Unlock()

	now := time.Now()
	shouldCheckSSL := state.LastSSLCheck.IsZero() || now.Sub(state.LastSSLCheck) >= 24*time.Hour

	if shouldCheckSSL {
		sslInfo := CheckSSLCertificate(url, m.config.SSLExpiryWarningDays)
		if sslInfo.IsHTTPS {
			state.SSLCertExpiry = sslInfo.Expiry
			state.DaysToExpiry = sslInfo.DaysToExpiry
			state.SSLExpiringSoon = sslInfo.ExpiringSoon
			state.LastSSLCheck = now

			if sslInfo.ExpiringSoon {
				logger.Infof("[%s] ⚠️  SSL certificate expiring in %d days", state.Endpoint.Name, sslInfo.DaysToExpiry)
			}

			logger.Infof("[%s] SSL certificate validated (expires: %s, days remaining: %d)",
				state.Endpoint.Name, sslInfo.Expiry.Format("2006-01-02"), sslInfo.DaysToExpiry)
		}
	}

	// Set next check to 24 hours for SSL-only endpoints
	state.LastCheck = now
	state.NextCheck = now.Add(24 * time.Hour)
}

// handleCheckSuccess handles a successful health check
func (m *Monitor) handleCheckSuccess(state *MonitorState, responseTime time.Duration) {
	state.mu.Lock()
	defer state.mu.Unlock()

	state.LastCheck = time.Now()
	state.NextCheck = time.Now().Add(state.CheckInterval)
	state.ResponseTime = responseTime
	state.ConsecutiveFailures = 0
	state.ConsecutiveSuccesses++
	state.LastError = ""

	previousStatus := state.Status

	// Update status if threshold is met
	if state.ConsecutiveSuccesses >= state.Endpoint.SuccessThreshold {
		state.Status = structs.StatusHealthy
	}

	// Check SSL certificate expiry for HTTPS endpoints (once per day)
	// Run immediately for new endpoints (LastSSLCheck is zero) or if 24 hours have passed
	now := time.Now()
	shouldCheckSSL := state.LastSSLCheck.IsZero() || now.Sub(state.LastSSLCheck) >= 24*time.Hour
	
	if shouldCheckSSL {
		sslInfo := CheckSSLCertificate(state.Endpoint.URL, m.config.SSLExpiryWarningDays)
		if sslInfo.IsHTTPS {
			state.SSLCertExpiry = sslInfo.Expiry
			state.DaysToExpiry = sslInfo.DaysToExpiry
			state.SSLExpiringSoon = sslInfo.ExpiringSoon
			state.LastSSLCheck = now
			
			if sslInfo.ExpiringSoon {
				logger.Infof("[%s] ⚠️  SSL certificate expiring in %d days", state.Endpoint.Name, sslInfo.DaysToExpiry)
			}
			
			logger.Infof("[%s] SSL certificate validated (expires: %s, days remaining: %d)", 
				state.Endpoint.Name, sslInfo.Expiry.Format("2006-01-02"), sslInfo.DaysToExpiry)
		}
	}

	logger.Infof("[%s] ✓ Health check passed (status: %s, response time: %v)", 
		state.Endpoint.Name, state.Status, responseTime)

	// Send recovery alert if endpoint recovered
	if previousStatus == structs.StatusUnhealthy && state.Status == structs.StatusHealthy {
		state.LastStatusChange = time.Now()
		if !state.AlertsSuppressed {
			m.alerter.SendRecoveryAlert(state.Endpoint, state.EndpointState)
		}
	}

	// Save health check record to database
	m.saveHealthRecord(state, "")
}

// handleCheckFailure handles a failed health check
func (m *Monitor) handleCheckFailure(state *MonitorState, errorMsg string, responseTime time.Duration) {
	state.mu.Lock()
	defer state.mu.Unlock()

	state.LastCheck = time.Now()
	state.NextCheck = time.Now().Add(state.CheckInterval)
	state.ResponseTime = responseTime
	state.ConsecutiveSuccesses = 0
	state.ConsecutiveFailures++
	state.LastError = errorMsg

	previousStatus := state.Status

	// Update status if threshold is met
	if state.ConsecutiveFailures >= state.Endpoint.FailureThreshold {
		state.Status = structs.StatusUnhealthy
	}

	logger.Infof("[%s] ✗ Health check failed (status: %s, error: %s)", 
		state.Endpoint.Name, state.Status, errorMsg)

	// Send alert if endpoint became unhealthy
	if previousStatus != structs.StatusUnhealthy && state.Status == structs.StatusUnhealthy {
		state.LastStatusChange = time.Now()
		if !state.AlertsSuppressed {
			m.alerter.SendFailureAlert(state.Endpoint, state.EndpointState)
		}
	}

	// Save health check record to database
	m.saveHealthRecord(state, errorMsg)
}

// saveHealthRecord saves a health check result to the database
func (m *Monitor) saveHealthRecord(state *MonitorState, errorMsg string) {
	if m.db == nil {
		return
	}

	record := &structs.HealthCheckRecord{
		EndpointID:   state.ID,
		Timestamp:    state.LastCheck,
		Status:       string(state.Status),
		ResponseTime: state.ResponseTime,
		Error:        errorMsg,
	}

	if err := m.db.SaveHealthCheckRecord(record); err != nil {
		logger.Errorf("Error saving health check record: %v", err)
	}
}

// GetStatus returns the current status of all endpoints
func (m *Monitor) GetStatus() map[string]*structs.EndpointState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[string]*structs.EndpointState)
	for name, state := range m.states {
		state.mu.RLock()
		status[name] = state.EndpointState
		state.mu.RUnlock()
	}
	return status
}

// startSSLExpirySummaryScheduler schedules daily SSL expiry summary at configured time
func (m *Monitor) startSSLExpirySummaryScheduler() {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		loc = time.FixedZone("IST", 5*60*60+30*60)
	}

	// Parse configured time (format: HH:MM)
	var hour, minute int
	_, err = fmt.Sscanf(m.config.SSLSummaryTime, "%d:%d", &hour, &minute)
	if err != nil {
		logger.Errorf("Invalid SSL summary time format '%s', using default 09:30", m.config.SSLSummaryTime)
		hour, minute = 9, 30
	}

	for {
		now := time.Now().In(loc)
		
		// Calculate next scheduled time
		next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, loc)
		if now.After(next) {
			// If it's already past the scheduled time today, schedule for tomorrow
			next = next.Add(24 * time.Hour)
		}

		duration := next.Sub(now)
		logger.Infof("Next SSL expiry summary scheduled at: %s (in %v)", next.Format("02 Jan 2006 03:04 PM"), duration.Round(time.Minute))

		select {
		case <-m.ctx.Done():
			return
		case <-time.After(duration):
			// Send SSL expiry summary
			m.sendSSLExpirySummary()
		}
	}
}

// sendSSLExpirySummary collects and sends SSL expiry summary
func (m *Monitor) sendSSLExpirySummary() {
	expiringCerts := m.getExpiringCertificates()
	
	if len(expiringCerts) > 0 {
		logger.Infof("Sending SSL expiry summary for %d certificates", len(expiringCerts))
		m.alerter.SendSSLExpirySummary(expiringCerts)
	} else {
		logger.Info("No expiring SSL certificates to report in daily summary")
	}
}

// getExpiringCertificates returns a list of expiring SSL certificates sorted by days remaining (ascending)
func (m *Monitor) getExpiringCertificates() []SSLExpiryInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var expiringCerts []SSLExpiryInfo

	for _, state := range m.states {
		state.mu.RLock()
		if state.SSLExpiringSoon && !state.SSLCertExpiry.IsZero() {
			expiringCerts = append(expiringCerts, SSLExpiryInfo{
				EndpointName: state.Endpoint.Name,
				URL:          state.Endpoint.URL,
				ExpiryDate:   state.SSLCertExpiry,
				DaysToExpiry: state.DaysToExpiry,
			})
		}
		state.mu.RUnlock()
	}

	// Sort by days remaining (ascending order - most urgent first)
	for i := 0; i < len(expiringCerts)-1; i++ {
		for j := i + 1; j < len(expiringCerts); j++ {
			if expiringCerts[i].DaysToExpiry > expiringCerts[j].DaysToExpiry {
				expiringCerts[i], expiringCerts[j] = expiringCerts[j], expiringCerts[i]
			}
		}
	}

	return expiringCerts
}
