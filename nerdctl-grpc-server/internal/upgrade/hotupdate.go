package upgrade

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/containerd/nerdctl-grpc-server/internal/config"
	"github.com/containerd/nerdctl-grpc-server/internal/interfaces"
)

// HotUpdateService manages hot updates for the gRPC server without downtime
type HotUpdateService struct {
	manager            *UpgradeManager
	migrationRegistry  *MigrationRegistry
	persistence        *MigrationPersistence
	config             *config.Config
	
	// Current state
	currentAdapter     atomic.Value // stores interfaces.Adapter
	adapterGeneration  int64        // incremented on each update
	
	// Update coordination
	updateMutex        sync.RWMutex
	pendingUpdate      *PendingUpdate
	updateQueue        chan *UpdateRequest
	
	// Monitoring
	updateHistory      []*UpdateRecord
	maxHistorySize     int
	healthChecker      HealthChecker
	
	// Lifecycle
	ctx                context.Context
	cancel             context.CancelFunc
	shutdownWg         sync.WaitGroup
}

// PendingUpdate represents an update in progress
type PendingUpdate struct {
	ID            string
	FromVersion   string
	ToVersion     string
	Strategy      string
	StartTime     time.Time
	Progress      float64
	Status        string
	LastHeartbeat time.Time
}

// UpdateRequest represents a request to perform an update
type UpdateRequest struct {
	TargetVersion string
	Strategy      string
	Force         bool
	DryRun        bool
	ResponseChan  chan *UpdateResponse
}

// UpdateResponse contains the result of an update request
type UpdateResponse struct {
	Success   bool
	UpdateID  string
	Error     error
	Metadata  map[string]interface{}
}

// UpdateRecord tracks completed updates
type UpdateRecord struct {
	ID          string    `json:"id"`
	FromVersion string    `json:"from_version"`
	ToVersion   string    `json:"to_version"`
	Strategy    string    `json:"strategy"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	Duration    time.Duration `json:"duration"`
	Success     bool      `json:"success"`
	Error       string    `json:"error,omitempty"`
}

// HealthChecker defines interface for checking system health
type HealthChecker interface {
	CheckHealth(ctx context.Context) error
	IsHealthy() bool
	GetHealthStatus() map[string]interface{}
}

// DefaultHealthChecker provides basic health checking
type DefaultHealthChecker struct {
	adapter interfaces.Adapter
}

// NewHotUpdateService creates a new hot update service
func NewHotUpdateService(manager *UpgradeManager, config *config.Config) *HotUpdateService {
	ctx, cancel := context.WithCancel(context.Background())
	
	service := &HotUpdateService{
		manager:           manager,
		migrationRegistry: NewMigrationRegistry(manager),
		persistence:       NewMigrationPersistence(config.StateDir),
		config:            config,
		updateQueue:       make(chan *UpdateRequest, 10),
		maxHistorySize:    100,
		healthChecker:     &DefaultHealthChecker{},
		ctx:               ctx,
		cancel:            cancel,
	}
	
	// Initialize with current adapter
	if manager.currentAdapter != nil {
		service.currentAdapter.Store(manager.currentAdapter)
	}
	
	return service
}

// Start begins the hot update service
func (hus *HotUpdateService) Start() error {
	// Start update worker
	hus.shutdownWg.Add(1)
	go hus.updateWorker()
	
	// Start health monitoring
	hus.shutdownWg.Add(1)
	go hus.healthMonitor()
	
	// Start periodic update checks
	if hus.config.AutoUpgrade {
		hus.shutdownWg.Add(1)
		go hus.periodicUpdateChecker()
	}
	
	return nil
}

// Stop gracefully shuts down the hot update service
func (hus *HotUpdateService) Stop() error {
	hus.cancel()
	
	// Close update queue
	close(hus.updateQueue)
	
	// Wait for all workers to complete
	done := make(chan struct{})
	go func() {
		hus.shutdownWg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		return nil
	case <-time.After(30 * time.Second):
		return fmt.Errorf("shutdown timeout")
	}
}

// GetCurrentAdapter returns the current adapter (thread-safe)
func (hus *HotUpdateService) GetCurrentAdapter() interfaces.Adapter {
	if adapter := hus.currentAdapter.Load(); adapter != nil {
		return adapter.(interfaces.Adapter)
	}
	return nil
}

// GetAdapterGeneration returns the current adapter generation
func (hus *HotUpdateService) GetAdapterGeneration() int64 {
	return atomic.LoadInt64(&hus.adapterGeneration)
}

// RequestUpdate submits an update request
func (hus *HotUpdateService) RequestUpdate(req *UpdateRequest) *UpdateResponse {
	// Validate request
	if err := hus.validateUpdateRequest(req); err != nil {
		return &UpdateResponse{
			Success: false,
			Error:   err,
		}
	}
	
	// Check if update is already in progress
	hus.updateMutex.RLock()
	if hus.pendingUpdate != nil {
		hus.updateMutex.RUnlock()
		return &UpdateResponse{
			Success: false,
			Error:   fmt.Errorf("update already in progress: %s", hus.pendingUpdate.ID),
		}
	}
	hus.updateMutex.RUnlock()
	
	// Submit to queue
	req.ResponseChan = make(chan *UpdateResponse, 1)
	
	select {
	case hus.updateQueue <- req:
		// Wait for response
		return <-req.ResponseChan
	case <-time.After(30 * time.Second):
		return &UpdateResponse{
			Success: false,
			Error:   fmt.Errorf("update request timeout"),
		}
	}
}

// GetUpdateStatus returns the status of current or recent updates
func (hus *HotUpdateService) GetUpdateStatus() *UpdateStatus {
	hus.updateMutex.RLock()
	defer hus.updateMutex.RUnlock()
	
	status := &UpdateStatus{
		CurrentGeneration: hus.GetAdapterGeneration(),
		IsHealthy:         hus.healthChecker.IsHealthy(),
		HealthStatus:      hus.healthChecker.GetHealthStatus(),
	}
	
	if hus.pendingUpdate != nil {
		status.PendingUpdate = hus.pendingUpdate
	}
	
	if len(hus.updateHistory) > 0 {
		status.LastUpdate = hus.updateHistory[len(hus.updateHistory)-1]
	}
	
	return status
}

// UpdateStatus represents the current update status
type UpdateStatus struct {
	CurrentGeneration int64                  `json:"current_generation"`
	IsHealthy         bool                   `json:"is_healthy"`
	HealthStatus      map[string]interface{} `json:"health_status"`
	PendingUpdate     *PendingUpdate         `json:"pending_update,omitempty"`
	LastUpdate        *UpdateRecord          `json:"last_update,omitempty"`
}

// Internal worker functions

func (hus *HotUpdateService) updateWorker() {
	defer hus.shutdownWg.Done()
	
	for {
		select {
		case req := <-hus.updateQueue:
			if req == nil {
				return // Channel closed
			}
			
			response := hus.processUpdateRequest(req)
			req.ResponseChan <- response
			
		case <-hus.ctx.Done():
			return
		}
	}
}

func (hus *HotUpdateService) processUpdateRequest(req *UpdateRequest) *UpdateResponse {
	updateID := generateUpdateID()
	startTime := time.Now()
	
	// Create pending update
	pending := &PendingUpdate{
		ID:            updateID,
		FromVersion:   hus.getCurrentVersion(),
		ToVersion:     req.TargetVersion,
		Strategy:      req.Strategy,
		StartTime:     startTime,
		Progress:      0.0,
		Status:        "starting",
		LastHeartbeat: startTime,
	}
	
	hus.updateMutex.Lock()
	hus.pendingUpdate = pending
	hus.updateMutex.Unlock()
	
	defer func() {
		hus.updateMutex.Lock()
		hus.pendingUpdate = nil
		hus.updateMutex.Unlock()
	}()
	
	// Dry run check
	if req.DryRun {
		return hus.performDryRun(req, pending)
	}
	
	// Execute the update
	return hus.executeUpdate(req, pending)
}

func (hus *HotUpdateService) executeUpdate(req *UpdateRequest, pending *PendingUpdate) *UpdateResponse {
	// Create migration
	migration := &Migration{
		ID:          pending.ID,
		FromVersion: pending.FromVersion,
		ToVersion:   pending.ToVersion,
		Strategy:    pending.Strategy,
		StartTime:   pending.StartTime,
		Config:      hus.config,
		Metadata:    make(map[string]string),
	}
	
	// Save migration state
	if err := hus.persistence.SaveMigration(migration); err != nil {
		return &UpdateResponse{
			Success: false,
			Error:   fmt.Errorf("failed to save migration state: %w", err),
		}
	}
	
	// Get migration strategy
	strategy, err := hus.migrationRegistry.GetStrategy(req.Strategy)
	if err != nil {
		return &UpdateResponse{
			Success: false,
			Error:   fmt.Errorf("invalid strategy: %w", err),
		}
	}
	
	// Validate migration
	if err := strategy.Validate(migration); err != nil {
		return &UpdateResponse{
			Success: false,
			Error:   fmt.Errorf("migration validation failed: %w", err),
		}
	}
	
	// Update progress
	hus.updateProgress(pending, 25.0, "validated")
	
	// Pre-update health check
	if !hus.healthChecker.IsHealthy() && !req.Force {
		return &UpdateResponse{
			Success: false,
			Error:   fmt.Errorf("system is not healthy, use force=true to override"),
		}
	}
	
	hus.updateProgress(pending, 40.0, "health_checked")
	
	// Execute migration
	if err := strategy.Execute(hus.ctx, migration); err != nil {
		// Save failed migration
		hus.persistence.SaveMigration(migration)
		
		// Record failure
		hus.recordUpdate(&UpdateRecord{
			ID:          pending.ID,
			FromVersion: pending.FromVersion,
			ToVersion:   pending.ToVersion,
			Strategy:    pending.Strategy,
			StartTime:   pending.StartTime,
			EndTime:     time.Now(),
			Duration:    time.Since(pending.StartTime),
			Success:     false,
			Error:       err.Error(),
		})
		
		return &UpdateResponse{
			Success: false,
			Error:   fmt.Errorf("migration execution failed: %w", err),
		}
	}
	
	hus.updateProgress(pending, 80.0, "executed")
	
	// Update current adapter
	newAdapter := hus.manager.GetCurrentAdapter()
	if newAdapter != nil {
		hus.currentAdapter.Store(newAdapter)
		atomic.AddInt64(&hus.adapterGeneration, 1)
	}
	
	hus.updateProgress(pending, 90.0, "updated")
	
	// Post-update health check
	if err := hus.healthChecker.CheckHealth(hus.ctx); err != nil {
		// Attempt rollback
		rollbackErr := strategy.Rollback(hus.ctx, migration)
		
		errorMsg := fmt.Sprintf("post-update health check failed: %v", err)
		if rollbackErr != nil {
			errorMsg += fmt.Sprintf(", rollback also failed: %v", rollbackErr)
		}
		
		return &UpdateResponse{
			Success: false,
			Error:   fmt.Errorf(errorMsg),
		}
	}
	
	hus.updateProgress(pending, 100.0, "completed")
	
	// Record successful update
	record := &UpdateRecord{
		ID:          pending.ID,
		FromVersion: pending.FromVersion,
		ToVersion:   pending.ToVersion,
		Strategy:    pending.Strategy,
		StartTime:   pending.StartTime,
		EndTime:     time.Now(),
		Duration:    time.Since(pending.StartTime),
		Success:     true,
	}
	hus.recordUpdate(record)
	
	// Clean up migration state
	hus.persistence.DeleteMigration(migration.ID)
	
	return &UpdateResponse{
		Success:  true,
		UpdateID: pending.ID,
		Metadata: map[string]interface{}{
			"generation": hus.GetAdapterGeneration(),
			"duration":   record.Duration.String(),
		},
	}
}

func (hus *HotUpdateService) performDryRun(req *UpdateRequest, pending *PendingUpdate) *UpdateResponse {
	// Simulate update without actually executing
	strategy, err := hus.migrationRegistry.GetStrategy(req.Strategy)
	if err != nil {
		return &UpdateResponse{
			Success: false,
			Error:   fmt.Errorf("invalid strategy: %w", err),
		}
	}
	
	migration := &Migration{
		ID:          pending.ID,
		FromVersion: pending.FromVersion,
		ToVersion:   pending.ToVersion,
		Strategy:    pending.Strategy,
		StartTime:   pending.StartTime,
		Config:      hus.config,
	}
	
	if err := strategy.Validate(migration); err != nil {
		return &UpdateResponse{
			Success: false,
			Error:   fmt.Errorf("dry run validation failed: %w", err),
		}
	}
	
	return &UpdateResponse{
		Success:  true,
		UpdateID: pending.ID,
		Metadata: map[string]interface{}{
			"dry_run":    true,
			"validation": "passed",
		},
	}
}

func (hus *HotUpdateService) healthMonitor() {
	defer hus.shutdownWg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Update health checker with current adapter
			if adapter := hus.GetCurrentAdapter(); adapter != nil {
				if defaultChecker, ok := hus.healthChecker.(*DefaultHealthChecker); ok {
					defaultChecker.adapter = adapter
				}
			}
			
			// Perform health check
			_ = hus.healthChecker.CheckHealth(hus.ctx)
			
		case <-hus.ctx.Done():
			return
		}
	}
}

func (hus *HotUpdateService) periodicUpdateChecker() {
	defer hus.shutdownWg.Done()
	
	ticker := time.NewTicker(hus.config.UpgradeCheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if hus.pendingUpdate == nil {
				hus.checkForAutoUpdate()
			}
			
		case <-hus.ctx.Done():
			return
		}
	}
}

// Helper functions

func (hus *HotUpdateService) validateUpdateRequest(req *UpdateRequest) error {
	if req.TargetVersion == "" {
		return fmt.Errorf("target version is required")
	}
	
	if req.Strategy == "" {
		req.Strategy = "immediate" // Default strategy
	}
	
	// Check if strategy is available
	if _, err := hus.migrationRegistry.GetStrategy(req.Strategy); err != nil {
		return err
	}
	
	return nil
}

func (hus *HotUpdateService) getCurrentVersion() string {
	// Extract version from current adapter
	// This is a placeholder implementation
	return "current"
}

func (hus *HotUpdateService) updateProgress(pending *PendingUpdate, progress float64, status string) {
	hus.updateMutex.Lock()
	if pending != nil {
		pending.Progress = progress
		pending.Status = status
		pending.LastHeartbeat = time.Now()
	}
	hus.updateMutex.Unlock()
}

func (hus *HotUpdateService) recordUpdate(record *UpdateRecord) {
	hus.updateMutex.Lock()
	defer hus.updateMutex.Unlock()
	
	hus.updateHistory = append(hus.updateHistory, record)
	
	// Limit history size
	if len(hus.updateHistory) > hus.maxHistorySize {
		hus.updateHistory = hus.updateHistory[1:]
	}
}

func (hus *HotUpdateService) checkForAutoUpdate() {
	// Check for available updates
	if err := hus.manager.TriggerUpgrade(hus.ctx); err != nil {
		// Log error but don't fail
		return
	}
}

func generateUpdateID() string {
	return fmt.Sprintf("update_%d", time.Now().UnixNano())
}

// DefaultHealthChecker implementation

func (dhc *DefaultHealthChecker) CheckHealth(ctx context.Context) error {
	if dhc.adapter == nil {
		return fmt.Errorf("no adapter available")
	}
	
	// Test basic functionality
	containerManager := dhc.adapter.ContainerManager()
	if containerManager == nil {
		return fmt.Errorf("container manager is nil")
	}
	
	// Test list operation
	_, err := containerManager.ListContainers(ctx, false)
	return err
}

func (dhc *DefaultHealthChecker) IsHealthy() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	return dhc.CheckHealth(ctx) == nil
}

func (dhc *DefaultHealthChecker) GetHealthStatus() map[string]interface{} {
	status := map[string]interface{}{
		"timestamp": time.Now(),
		"healthy":   dhc.IsHealthy(),
	}
	
	if dhc.adapter != nil {
		status["adapter_available"] = true
	} else {
		status["adapter_available"] = false
	}
	
	return status
}