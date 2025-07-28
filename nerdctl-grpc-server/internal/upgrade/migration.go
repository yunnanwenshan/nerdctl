package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/containerd/nerdctl-grpc-server/internal/config"
	"github.com/containerd/nerdctl-grpc-server/internal/interfaces"
)

// MigrationStrategy defines different migration approaches
type MigrationStrategy interface {
	// Execute performs the migration
	Execute(ctx context.Context, migration *Migration) error
	
	// Validate checks if migration is possible
	Validate(migration *Migration) error
	
	// Rollback reverses the migration
	Rollback(ctx context.Context, migration *Migration) error
}

// Migration represents a version migration operation
type Migration struct {
	ID              string            `json:"id"`
	FromVersion     string            `json:"from_version"`
	ToVersion       string            `json:"to_version"`
	Strategy        string            `json:"strategy"`
	StartTime       time.Time         `json:"start_time"`
	EndTime         *time.Time        `json:"end_time,omitempty"`
	Status          MigrationStatus   `json:"status"`
	Config          *config.Config    `json:"config"`
	Metadata        map[string]string `json:"metadata"`
	RollbackData    *RollbackData     `json:"rollback_data,omitempty"`
	Error           *string           `json:"error,omitempty"`
}

// MigrationStatus represents the current state of a migration
type MigrationStatus string

const (
	MigrationStatusPending   MigrationStatus = "pending"
	MigrationStatusRunning   MigrationStatus = "running"
	MigrationStatusCompleted MigrationStatus = "completed"
	MigrationStatusFailed    MigrationStatus = "failed"
	MigrationStatusRolledBack MigrationStatus = "rolled_back"
)

// RollbackData contains information needed for rollback
type RollbackData struct {
	PreviousConfig   *config.Config    `json:"previous_config"`
	PreviousBinary   string            `json:"previous_binary"`
	AdapterState     map[string]interface{} `json:"adapter_state"`
	BackupTimestamp  time.Time         `json:"backup_timestamp"`
}

// ImmediateMigrationStrategy performs immediate hot-swap migration
type ImmediateMigrationStrategy struct {
	manager *UpgradeManager
}

// GracefulMigrationStrategy performs graceful migration with connection draining
type GracefulMigrationStrategy struct {
	manager         *UpgradeManager
	drainTimeout    time.Duration
	healthCheckFunc func() bool
}

// ScheduledMigrationStrategy performs migration at a scheduled time
type ScheduledMigrationStrategy struct {
	manager      *UpgradeManager
	scheduledTime time.Time
}

// MigrationRegistry manages available migration strategies
type MigrationRegistry struct {
	strategies map[string]MigrationStrategy
}

// NewMigrationRegistry creates a new registry with default strategies
func NewMigrationRegistry(manager *UpgradeManager) *MigrationRegistry {
	registry := &MigrationRegistry{
		strategies: make(map[string]MigrationStrategy),
	}

	// Register default strategies
	registry.RegisterStrategy("immediate", &ImmediateMigrationStrategy{manager: manager})
	registry.RegisterStrategy("graceful", &GracefulMigrationStrategy{
		manager:      manager,
		drainTimeout: 30 * time.Second,
	})
	registry.RegisterStrategy("scheduled", &ScheduledMigrationStrategy{manager: manager})

	return registry
}

// RegisterStrategy registers a new migration strategy
func (mr *MigrationRegistry) RegisterStrategy(name string, strategy MigrationStrategy) {
	mr.strategies[name] = strategy
}

// GetStrategy returns a migration strategy by name
func (mr *MigrationRegistry) GetStrategy(name string) (MigrationStrategy, error) {
	strategy, exists := mr.strategies[name]
	if !exists {
		return nil, fmt.Errorf("unknown migration strategy: %s", name)
	}
	return strategy, nil
}

// ListStrategies returns all available strategy names
func (mr *MigrationRegistry) ListStrategies() []string {
	var strategies []string
	for name := range mr.strategies {
		strategies = append(strategies, name)
	}
	return strategies
}

// Immediate Migration Strategy Implementation

func (ims *ImmediateMigrationStrategy) Execute(ctx context.Context, migration *Migration) error {
	migration.Status = MigrationStatusRunning
	
	// Create rollback data
	rollbackData, err := ims.createRollbackData(migration)
	if err != nil {
		return fmt.Errorf("failed to create rollback data: %w", err)
	}
	migration.RollbackData = rollbackData

	// Update configuration for new version
	newConfig := *migration.Config
	newConfig.PreferredVersion = migration.ToVersion
	
	// Create new adapter
	newAdapter, err := ims.manager.factory.CreateAdapter(ctx)
	if err != nil {
		migration.Status = MigrationStatusFailed
		errorMsg := err.Error()
		migration.Error = &errorMsg
		return fmt.Errorf("failed to create new adapter: %w", err)
	}

	// Perform atomic swap
	ims.manager.mutex.Lock()
	oldAdapter := ims.manager.currentAdapter
	ims.manager.currentAdapter = newAdapter
	ims.manager.config = &newConfig
	ims.manager.mutex.Unlock()

	// Verify new adapter works
	if err := ims.verifyAdapter(ctx, newAdapter); err != nil {
		// Rollback on verification failure
		ims.manager.mutex.Lock()
		ims.manager.currentAdapter = oldAdapter
		ims.manager.mutex.Unlock()
		
		migration.Status = MigrationStatusFailed
		errorMsg := fmt.Sprintf("adapter verification failed: %v", err)
		migration.Error = &errorMsg
		return err
	}

	// Clean up old adapter
	ims.manager.cleanupAdapter(oldAdapter)

	// Update migration status
	migration.Status = MigrationStatusCompleted
	now := time.Now()
	migration.EndTime = &now

	return nil
}

func (ims *ImmediateMigrationStrategy) Validate(migration *Migration) error {
	if migration.FromVersion == migration.ToVersion {
		return fmt.Errorf("source and target versions are the same")
	}
	
	// Validate version compatibility
	if !ims.isCompatibleMigration(migration.FromVersion, migration.ToVersion) {
		return fmt.Errorf("incompatible migration from %s to %s", migration.FromVersion, migration.ToVersion)
	}
	
	return nil
}

func (ims *ImmediateMigrationStrategy) Rollback(ctx context.Context, migration *Migration) error {
	if migration.RollbackData == nil {
		return fmt.Errorf("no rollback data available")
	}

	// Restore previous configuration
	ims.manager.mutex.Lock()
	ims.manager.config = migration.RollbackData.PreviousConfig
	ims.manager.mutex.Unlock()

	// Create adapter with previous configuration
	previousAdapter, err := ims.manager.factory.CreateAdapter(ctx)
	if err != nil {
		return fmt.Errorf("failed to create rollback adapter: %w", err)
	}

	// Perform rollback swap
	ims.manager.mutex.Lock()
	currentAdapter := ims.manager.currentAdapter
	ims.manager.currentAdapter = previousAdapter
	ims.manager.mutex.Unlock()

	// Clean up current adapter
	ims.manager.cleanupAdapter(currentAdapter)

	migration.Status = MigrationStatusRolledBack
	return nil
}

func (ims *ImmediateMigrationStrategy) createRollbackData(migration *Migration) (*RollbackData, error) {
	return &RollbackData{
		PreviousConfig:   migration.Config,
		PreviousBinary:   migration.Config.NerdctlPath,
		BackupTimestamp:  time.Now(),
		AdapterState:     make(map[string]interface{}),
	}, nil
}

func (ims *ImmediateMigrationStrategy) verifyAdapter(ctx context.Context, adapter interfaces.Adapter) error {
	// Basic verification - test core functionality
	containerManager := adapter.ContainerManager()
	if containerManager == nil {
		return fmt.Errorf("container manager is nil")
	}

	// Test basic operation
	_, err := containerManager.ListContainers(ctx, false)
	return err
}

func (ims *ImmediateMigrationStrategy) isCompatibleMigration(from, to string) bool {
	// Implement compatibility checking logic
	// For now, allow all migrations
	return true
}

// Graceful Migration Strategy Implementation

func (gms *GracefulMigrationStrategy) Execute(ctx context.Context, migration *Migration) error {
	migration.Status = MigrationStatusRunning

	// Create rollback data
	rollbackData, err := gms.createRollbackData(migration)
	if err != nil {
		return fmt.Errorf("failed to create rollback data: %w", err)
	}
	migration.RollbackData = rollbackData

	// Wait for graceful shutdown opportunity
	if err := gms.waitForGracefulMoment(ctx); err != nil {
		migration.Status = MigrationStatusFailed
		errorMsg := err.Error()
		migration.Error = &errorMsg
		return err
	}

	// Proceed with migration similar to immediate strategy
	return gms.executeGracefulSwap(ctx, migration)
}

func (gms *GracefulMigrationStrategy) Validate(migration *Migration) error {
	// Same validation as immediate strategy
	if migration.FromVersion == migration.ToVersion {
		return fmt.Errorf("source and target versions are the same")
	}
	return nil
}

func (gms *GracefulMigrationStrategy) Rollback(ctx context.Context, migration *Migration) error {
	// Similar to immediate rollback but with graceful handling
	return gms.executeGracefulRollback(ctx, migration)
}

func (gms *GracefulMigrationStrategy) waitForGracefulMoment(ctx context.Context) error {
	drainCtx, cancel := context.WithTimeout(ctx, gms.drainTimeout)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if gms.healthCheckFunc != nil && !gms.healthCheckFunc() {
				// System is quiet, proceed with migration
				return nil
			}
		case <-drainCtx.Done():
			// Timeout reached, proceed anyway
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (gms *GracefulMigrationStrategy) executeGracefulSwap(ctx context.Context, migration *Migration) error {
	// Implementation similar to immediate strategy but with additional graceful handling
	newConfig := *migration.Config
	newConfig.PreferredVersion = migration.ToVersion
	
	newAdapter, err := gms.manager.factory.CreateAdapter(ctx)
	if err != nil {
		migration.Status = MigrationStatusFailed
		errorMsg := err.Error()
		migration.Error = &errorMsg
		return err
	}

	// Gradual traffic shift (if applicable)
	if err := gms.performGradualShift(ctx, newAdapter); err != nil {
		migration.Status = MigrationStatusFailed
		errorMsg := err.Error()
		migration.Error = &errorMsg
		return err
	}

	// Complete the swap
	gms.manager.mutex.Lock()
	oldAdapter := gms.manager.currentAdapter
	gms.manager.currentAdapter = newAdapter
	gms.manager.config = &newConfig
	gms.manager.mutex.Unlock()

	gms.manager.cleanupAdapter(oldAdapter)

	migration.Status = MigrationStatusCompleted
	now := time.Now()
	migration.EndTime = &now
	return nil
}

func (gms *GracefulMigrationStrategy) executeGracefulRollback(ctx context.Context, migration *Migration) error {
	// Graceful rollback with connection draining
	if migration.RollbackData == nil {
		return fmt.Errorf("no rollback data available")
	}

	// Similar to forward migration but in reverse
	gms.manager.mutex.Lock()
	gms.manager.config = migration.RollbackData.PreviousConfig
	gms.manager.mutex.Unlock()

	previousAdapter, err := gms.manager.factory.CreateAdapter(ctx)
	if err != nil {
		return fmt.Errorf("failed to create rollback adapter: %w", err)
	}

	gms.manager.mutex.Lock()
	currentAdapter := gms.manager.currentAdapter
	gms.manager.currentAdapter = previousAdapter
	gms.manager.mutex.Unlock()

	gms.manager.cleanupAdapter(currentAdapter)

	migration.Status = MigrationStatusRolledBack
	return nil
}

func (gms *GracefulMigrationStrategy) createRollbackData(migration *Migration) (*RollbackData, error) {
	return &RollbackData{
		PreviousConfig:   migration.Config,
		PreviousBinary:   migration.Config.NerdctlPath,
		BackupTimestamp:  time.Now(),
		AdapterState:     make(map[string]interface{}),
	}, nil
}

func (gms *GracefulMigrationStrategy) performGradualShift(ctx context.Context, newAdapter interfaces.Adapter) error {
	// Placeholder for gradual traffic shifting logic
	return nil
}

// Scheduled Migration Strategy Implementation

func (sms *ScheduledMigrationStrategy) Execute(ctx context.Context, migration *Migration) error {
	migration.Status = MigrationStatusPending

	// Wait until scheduled time
	select {
	case <-time.After(time.Until(sms.scheduledTime)):
		// Proceed with migration
		return sms.executeScheduledMigration(ctx, migration)
	case <-ctx.Done():
		migration.Status = MigrationStatusFailed
		errorMsg := "migration cancelled"
		migration.Error = &errorMsg
		return ctx.Err()
	}
}

func (sms *ScheduledMigrationStrategy) Validate(migration *Migration) error {
	if sms.scheduledTime.Before(time.Now()) {
		return fmt.Errorf("scheduled time is in the past")
	}
	return nil
}

func (sms *ScheduledMigrationStrategy) Rollback(ctx context.Context, migration *Migration) error {
	// Similar to immediate rollback
	if migration.RollbackData == nil {
		return fmt.Errorf("no rollback data available")
	}

	sms.manager.mutex.Lock()
	sms.manager.config = migration.RollbackData.PreviousConfig
	sms.manager.mutex.Unlock()

	previousAdapter, err := sms.manager.factory.CreateAdapter(ctx)
	if err != nil {
		return fmt.Errorf("failed to create rollback adapter: %w", err)
	}

	sms.manager.mutex.Lock()
	currentAdapter := sms.manager.currentAdapter
	sms.manager.currentAdapter = previousAdapter
	sms.manager.mutex.Unlock()

	sms.manager.cleanupAdapter(currentAdapter)

	migration.Status = MigrationStatusRolledBack
	return nil
}

func (sms *ScheduledMigrationStrategy) executeScheduledMigration(ctx context.Context, migration *Migration) error {
	migration.Status = MigrationStatusRunning

	// Execute similar to immediate strategy
	newConfig := *migration.Config
	newConfig.PreferredVersion = migration.ToVersion
	
	newAdapter, err := sms.manager.factory.CreateAdapter(ctx)
	if err != nil {
		migration.Status = MigrationStatusFailed
		errorMsg := err.Error()
		migration.Error = &errorMsg
		return err
	}

	sms.manager.mutex.Lock()
	oldAdapter := sms.manager.currentAdapter
	sms.manager.currentAdapter = newAdapter
	sms.manager.config = &newConfig
	sms.manager.mutex.Unlock()

	sms.manager.cleanupAdapter(oldAdapter)

	migration.Status = MigrationStatusCompleted
	now := time.Now()
	migration.EndTime = &now
	return nil
}

// Migration persistence for recovery

// MigrationPersistence handles saving and loading migration state
type MigrationPersistence struct {
	stateDir string
}

// NewMigrationPersistence creates a new persistence manager
func NewMigrationPersistence(stateDir string) *MigrationPersistence {
	return &MigrationPersistence{
		stateDir: stateDir,
	}
}

// SaveMigration saves migration state to disk
func (mp *MigrationPersistence) SaveMigration(migration *Migration) error {
	if err := os.MkdirAll(mp.stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	filePath := filepath.Join(mp.stateDir, fmt.Sprintf("migration_%s.json", migration.ID))
	
	data, err := json.MarshalIndent(migration, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal migration: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write migration file: %w", err)
	}

	return nil
}

// LoadMigration loads migration state from disk
func (mp *MigrationPersistence) LoadMigration(migrationID string) (*Migration, error) {
	filePath := filepath.Join(mp.stateDir, fmt.Sprintf("migration_%s.json", migrationID))
	
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read migration file: %w", err)
	}

	var migration Migration
	if err := json.Unmarshal(data, &migration); err != nil {
		return nil, fmt.Errorf("failed to unmarshal migration: %w", err)
	}

	return &migration, nil
}

// ListMigrations returns all stored migrations
func (mp *MigrationPersistence) ListMigrations() ([]*Migration, error) {
	files, err := filepath.Glob(filepath.Join(mp.stateDir, "migration_*.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to list migration files: %w", err)
	}

	var migrations []*Migration
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue // Skip corrupted files
		}

		var migration Migration
		if err := json.Unmarshal(data, &migration); err != nil {
			continue // Skip corrupted files
		}

		migrations = append(migrations, &migration)
	}

	return migrations, nil
}

// DeleteMigration removes migration state file
func (mp *MigrationPersistence) DeleteMigration(migrationID string) error {
	filePath := filepath.Join(mp.stateDir, fmt.Sprintf("migration_%s.json", migrationID))
	return os.Remove(filePath)
}