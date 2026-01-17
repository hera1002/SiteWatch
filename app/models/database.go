package models

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ashanmugaraja/cronzee/app/logger"
	"github.com/ashanmugaraja/cronzee/app/structs"
	"github.com/ashanmugaraja/cronzee/app/utils"
	bolt "go.etcd.io/bbolt"
)

const (
	// Bucket names
	EndpointsBucket = "endpoints"
	HistoryBucket   = "history"
	SettingsBucket  = "settings"

	// Data retention period
	DataRetentionDays = 3
)

// Database wraps BoltDB operations
type Database struct {
	db *bolt.DB
	mu sync.RWMutex
}

// NewDatabase creates and initializes a new BoltDB database
func NewDatabase(path string) (*Database, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create buckets
	err = db.Update(func(tx *bolt.Tx) error {
		buckets := []string{EndpointsBucket, HistoryBucket, SettingsBucket}
		for _, bucket := range buckets {
			_, err := tx.CreateBucketIfNotExists([]byte(bucket))
			if err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", bucket, err)
			}
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, err
	}

	database := &Database{db: db}

	// Start cleanup goroutine
	go database.startCleanupRoutine()

	return database, nil
}

// Close closes the database
func (d *Database) Close() error {
	return d.db.Close()
}

// SaveEndpoint saves or updates an endpoint
func (d *Database) SaveEndpoint(endpoint *structs.StoredEndpoint) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(EndpointsBucket))

		// Set timestamps
		now := time.Now()
		if endpoint.CreatedAt.IsZero() {
			endpoint.CreatedAt = now
		}
		endpoint.UpdatedAt = now

		// Set defaults
		if endpoint.Method == "" {
			endpoint.Method = "GET"
		}
		if endpoint.Timeout == 0 {
			endpoint.Timeout = 10 * time.Second
		}
		if endpoint.ExpectedStatus == 0 {
			endpoint.ExpectedStatus = 200
		}
		if endpoint.FailureThreshold == 0 {
			endpoint.FailureThreshold = 3
		}
		if endpoint.SuccessThreshold == 0 {
			endpoint.SuccessThreshold = 2
		}
		if endpoint.CheckInterval == 0 {
			endpoint.CheckInterval = 30 * time.Second
		}

		data, err := json.Marshal(endpoint)
		if err != nil {
			return fmt.Errorf("failed to marshal endpoint: %w", err)
		}

		return b.Put([]byte(endpoint.ID), data)
	})
}

// GetEndpoint retrieves an endpoint by ID
func (d *Database) GetEndpoint(id string) (*structs.StoredEndpoint, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var endpoint structs.StoredEndpoint
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(EndpointsBucket))
		data := b.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("endpoint not found: %s", id)
		}
		return json.Unmarshal(data, &endpoint)
	})
	if err != nil {
		return nil, err
	}
	return &endpoint, nil
}

// GetAllEndpoints retrieves all endpoints
func (d *Database) GetAllEndpoints() ([]*structs.StoredEndpoint, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var endpoints []*structs.StoredEndpoint
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(EndpointsBucket))
		return b.ForEach(func(k, v []byte) error {
			var endpoint structs.StoredEndpoint
			if err := json.Unmarshal(v, &endpoint); err != nil {
				return err
			}
			endpoints = append(endpoints, &endpoint)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return endpoints, nil
}

// GetEnabledEndpoints retrieves only enabled endpoints
func (d *Database) GetEnabledEndpoints() ([]*structs.StoredEndpoint, error) {
	all, err := d.GetAllEndpoints()
	if err != nil {
		return nil, err
	}

	var enabled []*structs.StoredEndpoint
	for _, ep := range all {
		if ep.Enabled {
			enabled = append(enabled, ep)
		}
	}
	return enabled, nil
}

// DeleteEndpoint removes an endpoint
func (d *Database) DeleteEndpoint(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(EndpointsBucket))
		return b.Delete([]byte(id))
	})
}

// EnableEndpoint enables an endpoint
func (d *Database) EnableEndpoint(id string) error {
	endpoint, err := d.GetEndpoint(id)
	if err != nil {
		return err
	}
	endpoint.Enabled = true
	return d.SaveEndpoint(endpoint)
}

// DisableEndpoint disables an endpoint
func (d *Database) DisableEndpoint(id string) error {
	endpoint, err := d.GetEndpoint(id)
	if err != nil {
		return err
	}
	endpoint.Enabled = false
	return d.SaveEndpoint(endpoint)
}

// SuppressAlerts suppresses alerts for an endpoint
func (d *Database) SuppressAlerts(id string) error {
	endpoint, err := d.GetEndpoint(id)
	if err != nil {
		return err
	}
	endpoint.AlertsSuppressed = true
	return d.SaveEndpoint(endpoint)
}

// UnsuppressAlerts enables alerts for an endpoint
func (d *Database) UnsuppressAlerts(id string) error {
	endpoint, err := d.GetEndpoint(id)
	if err != nil {
		return err
	}
	endpoint.AlertsSuppressed = false
	return d.SaveEndpoint(endpoint)
}

// SaveHealthCheckRecord saves a health check result to history
func (d *Database) SaveHealthCheckRecord(record *structs.HealthCheckRecord) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(HistoryBucket))

		// Create a unique key using endpoint ID and timestamp
		key := fmt.Sprintf("%s:%d", record.EndpointID, record.Timestamp.UnixNano())

		data, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("failed to marshal health check record: %w", err)
		}

		return b.Put([]byte(key), data)
	})
}

// GetHealthHistory retrieves health check history for an endpoint
func (d *Database) GetHealthHistory(endpointID string, limit int) ([]*structs.HealthCheckRecord, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var records []*structs.HealthCheckRecord
	prefix := []byte(endpointID + ":")

	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(HistoryBucket))
		c := b.Cursor()

		// Collect all matching records
		for k, v := c.Seek(prefix); k != nil && len(k) >= len(prefix) && string(k[:len(prefix)]) == string(prefix); k, v = c.Next() {
			var record structs.HealthCheckRecord
			if err := json.Unmarshal(v, &record); err != nil {
				continue
			}
			records = append(records, &record)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Sort by timestamp descending and limit
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	if limit > 0 && len(records) > limit {
		records = records[:limit]
	}

	return records, nil
}

// CleanupOldData removes data older than retention period
func (d *Database) CleanupOldData() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -DataRetentionDays)
	deletedCount := 0

	err := d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(HistoryBucket))
		c := b.Cursor()

		var keysToDelete [][]byte

		for k, v := c.First(); k != nil; k, v = c.Next() {
			var record structs.HealthCheckRecord
			if err := json.Unmarshal(v, &record); err != nil {
				continue
			}
			if record.Timestamp.Before(cutoff) {
				keysToDelete = append(keysToDelete, k)
			}
		}

		for _, key := range keysToDelete {
			if err := b.Delete(key); err != nil {
				return err
			}
			deletedCount++
		}

		return nil
	})

	if err == nil && deletedCount > 0 {
		logger.Infof("Cleaned up %d old health check records (older than %d days)", deletedCount, DataRetentionDays)
	}

	return err
}

// startCleanupRoutine runs periodic cleanup of old data
func (d *Database) startCleanupRoutine() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	// Run initial cleanup
	if err := d.CleanupOldData(); err != nil {
		logger.Errorf("Error during initial cleanup: %v", err)
	}

	for range ticker.C {
		if err := d.CleanupOldData(); err != nil {
			logger.Errorf("Error during cleanup: %v", err)
		}
	}
}

// MigrateFromConfig imports endpoints from config file to database
func (d *Database) MigrateFromConfig(endpoints []structs.Endpoint) error {
	for _, ep := range endpoints {
		stored := &structs.StoredEndpoint{
			ID:               utils.GenerateIDWithURL(ep.Name, ep.URL),
			Name:             ep.Name,
			URL:              ep.URL,
			Method:           ep.Method,
			Timeout:          ep.Timeout.Duration,
			ExpectedStatus:   ep.ExpectedStatus,
			Headers:          ep.Headers,
			FailureThreshold: ep.FailureThreshold,
			SuccessThreshold: ep.SuccessThreshold,
			Enabled:          true,
			AlertsSuppressed: false,
		}

		// Check if endpoint already exists
		existing, err := d.GetEndpoint(stored.ID)
		if err == nil && existing != nil {
			// Keep existing settings
			continue
		}

		if err := d.SaveEndpoint(stored); err != nil {
			return fmt.Errorf("failed to migrate endpoint %s: %w", ep.Name, err)
		}
		logger.Infof("Migrated endpoint from config: %s", ep.Name)
	}
	return nil
}
