package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// HealthStatus represents the health status of an endpoint
type HealthStatus string

const (
	StatusHealthy   HealthStatus = "healthy"
	StatusUnhealthy HealthStatus = "unhealthy"
	StatusUnknown   HealthStatus = "unknown"
)

// EndpointState tracks the state of a monitored endpoint
type EndpointState struct {
	Endpoint           Endpoint
	Status             HealthStatus
	LastCheck          time.Time
	LastStatusChange   time.Time
	ConsecutiveFailures  int
	ConsecutiveSuccesses int
	ResponseTime       time.Duration
	LastError          string
	Enabled            bool
	AlertsSuppressed   bool
	ID                 string
	CheckInterval      time.Duration
	NextCheck          time.Time
	mu                 sync.RWMutex
}

// Monitor manages health checks for multiple endpoints
type Monitor struct {
	config    *Config
	states    map[string]*EndpointState
	alerter   *Alerter
	db        *Database
	ticker    *time.Ticker
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.RWMutex
}

// NewMonitor creates a new health monitor
func NewMonitor(config *Config, db *Database) *Monitor {
	ctx, cancel := context.WithCancel(context.Background())
	
	monitor := &Monitor{
		config:  config,
		states:  make(map[string]*EndpointState),
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
		log.Printf("Error loading endpoints from database: %v", err)
		return
	}

	for _, stored := range endpoints {
		checkInterval := stored.CheckInterval
		if checkInterval == 0 {
			checkInterval = m.config.CheckInterval
		}
		m.states[stored.ID] = &EndpointState{
			ID:               stored.ID,
			Endpoint:         stored.ToEndpoint(),
			Status:           StatusUnknown,
			LastCheck:        time.Now(),
			Enabled:          stored.Enabled,
			AlertsSuppressed: stored.AlertsSuppressed,
			CheckInterval:    checkInterval,
			NextCheck:        time.Now(),
		}
	}
}

// ReloadEndpoints reloads endpoints from the database
func (m *Monitor) ReloadEndpoints() {
	m.loadEndpointsFromDB()
	log.Printf("Reloaded %d endpoints from database", len(m.states))
}

// AddEndpoint adds a new endpoint to monitoring
func (m *Monitor) AddEndpoint(stored *StoredEndpoint) error {
	if err := m.db.SaveEndpoint(stored); err != nil {
		return err
	}

	checkInterval := stored.CheckInterval
	if checkInterval == 0 {
		checkInterval = m.config.CheckInterval
	}

	m.mu.Lock()
	m.states[stored.ID] = &EndpointState{
		ID:               stored.ID,
		Endpoint:         stored.ToEndpoint(),
		Status:           StatusUnknown,
		LastCheck:        time.Now(),
		Enabled:          stored.Enabled,
		AlertsSuppressed: stored.AlertsSuppressed,
		CheckInterval:    checkInterval,
		NextCheck:        time.Now(),
	}
	m.mu.Unlock()

	log.Printf("Added endpoint: %s", stored.Name)
	return nil
}

// RemoveEndpoint removes an endpoint from monitoring
func (m *Monitor) RemoveEndpoint(id string) error {
	log.Printf("RemoveEndpoint called with id: %s", id)
	
	// Log current states before deletion
	m.mu.RLock()
	log.Printf("Current states keys: %v", func() []string {
		keys := make([]string, 0, len(m.states))
		for k := range m.states {
			keys = append(keys, k)
		}
		return keys
	}())
	_, exists := m.states[id]
	log.Printf("Endpoint %s exists in states: %v", id, exists)
	m.mu.RUnlock()
	
	if err := m.db.DeleteEndpoint(id); err != nil {
		log.Printf("Error deleting from DB: %v", err)
		return err
	}
	log.Printf("Deleted from DB: %s", id)

	m.mu.Lock()
	delete(m.states, id)
	log.Printf("Deleted from states map: %s, remaining count: %d", id, len(m.states))
	m.mu.Unlock()

	log.Printf("Removed endpoint: %s", id)
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

	log.Printf("Enabled endpoint: %s", id)
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

	log.Printf("Disabled endpoint: %s", id)
	return nil
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

	log.Printf("Suppressed alerts for endpoint: %s", id)
	return nil
}

// UpdateEndpointSettings updates endpoint settings in the monitor state
func (m *Monitor) UpdateEndpointSettings(id string, stored *StoredEndpoint) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, ok := m.states[id]; ok {
		state.mu.Lock()
		state.Endpoint.Timeout = stored.Timeout
		state.Endpoint.FailureThreshold = stored.FailureThreshold
		state.Endpoint.SuccessThreshold = stored.SuccessThreshold
		state.CheckInterval = stored.CheckInterval
		state.mu.Unlock()
		log.Printf("Updated endpoint settings: %s", id)
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

	log.Printf("Unsuppressed alerts for endpoint: %s", id)
	return nil
}

// Start begins monitoring all endpoints
func (m *Monitor) Start() {
	// Use a faster ticker (5 seconds) to check if any endpoint needs checking
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
}

// Stop stops the monitor
func (m *Monitor) Stop() {
	if m.ticker != nil {
		m.ticker.Stop()
	}
	m.cancel()
	m.wg.Wait()
}

// checkAllEndpoints checks all configured endpoints (used for initial check)
func (m *Monitor) checkAllEndpoints() {
	var wg sync.WaitGroup
	
	m.mu.RLock()
	for name, state := range m.states {
		state.mu.RLock()
		enabled := state.Enabled
		state.mu.RUnlock()
		
		if !enabled {
			continue
		}
		
		wg.Add(1)
		go func(n string, s *EndpointState) {
			defer wg.Done()
			m.checkEndpoint(s)
		}(name, state)
	}
	m.mu.RUnlock()
	
	wg.Wait()
}

// checkDueEndpoints checks endpoints that are due for checking based on their interval
func (m *Monitor) checkDueEndpoints() {
	var wg sync.WaitGroup
	now := time.Now()
	
	m.mu.RLock()
	for name, state := range m.states {
		state.mu.RLock()
		enabled := state.Enabled
		nextCheck := state.NextCheck
		state.mu.RUnlock()
		
		if !enabled || now.Before(nextCheck) {
			continue
		}
		
		wg.Add(1)
		go func(n string, s *EndpointState) {
			defer wg.Done()
			m.checkEndpoint(s)
		}(name, state)
	}
	m.mu.RUnlock()
	
	wg.Wait()
}

// checkEndpoint performs a health check on a single endpoint
func (m *Monitor) checkEndpoint(state *EndpointState) {
	start := time.Now()
	
	ctx, cancel := context.WithTimeout(m.ctx, state.Endpoint.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, state.Endpoint.Method, state.Endpoint.URL, nil)
	if err != nil {
		m.handleCheckFailure(state, fmt.Sprintf("failed to create request: %v", err), 0)
		return
	}

	// Add custom headers
	for key, value := range state.Endpoint.Headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{
		Timeout: state.Endpoint.Timeout,
	}

	resp, err := client.Do(req)
	responseTime := time.Since(start)

	if err != nil {
		m.handleCheckFailure(state, fmt.Sprintf("request failed: %v", err), responseTime)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != state.Endpoint.ExpectedStatus {
		m.handleCheckFailure(state, 
			fmt.Sprintf("unexpected status code: got %d, expected %d", resp.StatusCode, state.Endpoint.ExpectedStatus),
			responseTime)
		return
	}

	m.handleCheckSuccess(state, responseTime)
}

// handleCheckSuccess handles a successful health check
func (m *Monitor) handleCheckSuccess(state *EndpointState, responseTime time.Duration) {
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
		state.Status = StatusHealthy
	}

	log.Printf("[%s] ✓ Health check passed (status: %s, response time: %v)", 
		state.Endpoint.Name, state.Status, responseTime)

	// Send recovery alert if endpoint recovered
	if previousStatus == StatusUnhealthy && state.Status == StatusHealthy {
		state.LastStatusChange = time.Now()
		if !state.AlertsSuppressed {
			m.alerter.SendRecoveryAlert(state.Endpoint, state)
		}
	}

	// Save health check record to database
	m.saveHealthRecord(state, "")
}

// handleCheckFailure handles a failed health check
func (m *Monitor) handleCheckFailure(state *EndpointState, errorMsg string, responseTime time.Duration) {
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
		state.Status = StatusUnhealthy
	}

	log.Printf("[%s] ✗ Health check failed (status: %s, error: %s)", 
		state.Endpoint.Name, state.Status, errorMsg)

	// Send alert if endpoint became unhealthy
	if previousStatus != StatusUnhealthy && state.Status == StatusUnhealthy {
		state.LastStatusChange = time.Now()
		if !state.AlertsSuppressed {
			m.alerter.SendFailureAlert(state.Endpoint, state)
		}
	}

	// Save health check record to database
	m.saveHealthRecord(state, errorMsg)
}

// saveHealthRecord saves a health check result to the database
func (m *Monitor) saveHealthRecord(state *EndpointState, errorMsg string) {
	if m.db == nil {
		return
	}

	record := &HealthCheckRecord{
		EndpointID:   state.ID,
		Timestamp:    state.LastCheck,
		Status:       string(state.Status),
		ResponseTime: state.ResponseTime,
		Error:        errorMsg,
	}

	if err := m.db.SaveHealthCheckRecord(record); err != nil {
		log.Printf("Error saving health check record: %v", err)
	}
}

// GetStatus returns the current status of all endpoints
func (m *Monitor) GetStatus() map[string]*EndpointState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[string]*EndpointState)
	for name, state := range m.states {
		state.mu.RLock()
		status[name] = state
		state.mu.RUnlock()
	}
	return status
}
