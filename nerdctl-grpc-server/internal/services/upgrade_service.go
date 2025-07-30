package services

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/containerd/nerdctl-grpc-server/internal/upgrade"
	pb "github.com/containerd/nerdctl-grpc-server/proto/gen"
)

// UpgradeService implements gRPC upgrade management endpoints
type UpgradeService struct {
	pb.UnimplementedUpgradeServiceServer
	hotUpdateService *upgrade.HotUpdateService
	upgradeManager   *upgrade.UpgradeManager
}

// NewUpgradeService creates a new upgrade service
func NewUpgradeService(hotUpdateService *upgrade.HotUpdateService, upgradeManager *upgrade.UpgradeManager) *UpgradeService {
	return &UpgradeService{
		hotUpdateService: hotUpdateService,
		upgradeManager:   upgradeManager,
	}
}

// GetVersionInfo returns current version information
func (us *UpgradeService) GetVersionInfo(ctx context.Context, req *pb.GetVersionInfoRequest) (*pb.GetVersionInfoResponse, error) {
	versionState := us.upgradeManager.GetVersionState()
	
	response := &pb.GetVersionInfoResponse{
		CurrentVersion:    versionState.CurrentVersion,
		TargetVersion:     versionState.TargetVersion,
		LastCheck:         timestamppb.New(versionState.LastCheck),
		UpgradeInProgress: versionState.UpgradeInProgress,
		RollbackAvailable: versionState.RollbackAvailable,
		AdapterGeneration: us.hotUpdateService.GetAdapterGeneration(),
	}
	
	if versionState.LastUpgrade != nil && !versionState.LastUpgrade.IsZero() {
		response.LastUpgrade = timestamppb.New(*versionState.LastUpgrade)
	}
	
	if versionState.PreviousVersion != "" {
		response.PreviousVersion = versionState.PreviousVersion
	}
	
	return response, nil
}

// CheckForUpdates checks for available version updates
func (us *UpgradeService) CheckForUpdates(ctx context.Context, req *pb.CheckForUpdatesRequest) (*pb.CheckForUpdatesResponse, error) {
	// Force a version check
	err := us.upgradeManager.TriggerUpgrade(ctx)
	if err != nil && err.Error() != "no new version available" {
		return nil, status.Errorf(codes.Internal, "Failed to check for updates: %v", err)
	}
	
	// Get current state after check
	versionState := us.upgradeManager.GetVersionState()
	
	response := &pb.CheckForUpdatesResponse{
		UpdateAvailable:   versionState.TargetVersion != "",
		CurrentVersion:    versionState.CurrentVersion,
		AvailableVersion:  versionState.TargetVersion,
		LastCheck:         timestamppb.New(versionState.LastCheck),
	}
	
	return response, nil
}

// TriggerUpgrade initiates an upgrade process
func (us *UpgradeService) TriggerUpgrade(ctx context.Context, req *pb.TriggerUpgradeRequest) (*pb.TriggerUpgradeResponse, error) {
	// Validate request
	if req.TargetVersion == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Target version is required")
	}
	
	strategy := req.Strategy
	if strategy == "" {
		strategy = "immediate"
	}
	
	// Convert strategy from proto enum to string
	strategyStr := us.convertStrategyToString(req.StrategyEnum)
	if strategyStr != "" {
		strategy = strategyStr
	}
	
	// Create update request
	updateReq := &upgrade.UpdateRequest{
		TargetVersion: req.TargetVersion,
		Strategy:      strategy,
		Force:         req.Force,
		DryRun:        req.DryRun,
	}
	
	// Submit update request
	updateResp := us.hotUpdateService.RequestUpdate(updateReq)
	
	response := &pb.TriggerUpgradeResponse{
		Success:  updateResp.Success,
		UpdateId: updateResp.UpdateID,
	}
	
	if updateResp.Error != nil {
		response.Error = updateResp.Error.Error()
	}
	
	if updateResp.Metadata != nil {
		response.Metadata = make(map[string]string)
		for k, v := range updateResp.Metadata {
			if str, ok := v.(string); ok {
				response.Metadata[k] = str
			} else {
				response.Metadata[k] = fmt.Sprintf("%v", v)
			}
		}
	}
	
	if !updateResp.Success {
		return response, status.Errorf(codes.Internal, "Upgrade failed: %s", response.Error)
	}
	
	return response, nil
}

// GetUpgradeStatus returns the current upgrade status
func (us *UpgradeService) GetUpgradeStatus(ctx context.Context, req *pb.GetUpgradeStatusRequest) (*pb.GetUpgradeStatusResponse, error) {
	updateStatus := us.hotUpdateService.GetUpdateStatus()
	
	response := &pb.GetUpgradeStatusResponse{
		CurrentGeneration: updateStatus.CurrentGeneration,
		IsHealthy:         updateStatus.IsHealthy,
	}
	
	// Convert health status
	if updateStatus.HealthStatus != nil {
		response.HealthStatus = make(map[string]string)
		for k, v := range updateStatus.HealthStatus {
			response.HealthStatus[k] = fmt.Sprintf("%v", v)
		}
	}
	
	// Add pending update info
	if updateStatus.PendingUpdate != nil {
		response.PendingUpdate = &pb.PendingUpgrade{
			Id:            updateStatus.PendingUpdate.ID,
			FromVersion:   updateStatus.PendingUpdate.FromVersion,
			ToVersion:     updateStatus.PendingUpdate.ToVersion,
			Strategy:      updateStatus.PendingUpdate.Strategy,
			StartTime:     timestamppb.New(updateStatus.PendingUpdate.StartTime),
			Progress:      float32(updateStatus.PendingUpdate.Progress),
			Status:        updateStatus.PendingUpdate.Status,
			LastHeartbeat: timestamppb.New(updateStatus.PendingUpdate.LastHeartbeat),
		}
	}
	
	// Add last update info
	if updateStatus.LastUpdate != nil {
		response.LastUpdate = &pb.UpdateRecord{
			Id:          updateStatus.LastUpdate.ID,
			FromVersion: updateStatus.LastUpdate.FromVersion,
			ToVersion:   updateStatus.LastUpdate.ToVersion,
			Strategy:    updateStatus.LastUpdate.Strategy,
			StartTime:   timestamppb.New(updateStatus.LastUpdate.StartTime),
			EndTime:     timestamppb.New(updateStatus.LastUpdate.EndTime),
			Duration:    int64(updateStatus.LastUpdate.Duration.Seconds()),
			Success:     updateStatus.LastUpdate.Success,
			Error:       updateStatus.LastUpdate.Error,
		}
	}
	
	return response, nil
}

// RollbackUpgrade rolls back to the previous version
func (us *UpgradeService) RollbackUpgrade(ctx context.Context, req *pb.RollbackUpgradeRequest) (*pb.RollbackUpgradeResponse, error) {
	err := us.upgradeManager.RollbackToPrevious(ctx)
	
	response := &pb.RollbackUpgradeResponse{
		Success: err == nil,
	}
	
	if err != nil {
		response.Error = err.Error()
		return response, status.Errorf(codes.Internal, "Rollback failed: %v", err)
	}
	
	// Get updated version info after rollback
	versionState := us.upgradeManager.GetVersionState()
	response.NewVersion = versionState.CurrentVersion
	response.Generation = us.hotUpdateService.GetAdapterGeneration()
	
	return response, nil
}

// GetUpgradeHistory returns the history of upgrades
func (us *UpgradeService) GetUpgradeHistory(ctx context.Context, req *pb.GetUpgradeHistoryRequest) (*pb.GetUpgradeHistoryResponse, error) {
	// Get update status which includes history
	updateStatus := us.hotUpdateService.GetUpdateStatus()
	
	response := &pb.GetUpgradeHistoryResponse{
		Updates: []*pb.UpdateRecord{},
	}
	
	// Add last update if available
	if updateStatus.LastUpdate != nil {
		response.Updates = append(response.Updates, &pb.UpdateRecord{
			Id:          updateStatus.LastUpdate.ID,
			FromVersion: updateStatus.LastUpdate.FromVersion,
			ToVersion:   updateStatus.LastUpdate.ToVersion,
			Strategy:    updateStatus.LastUpdate.Strategy,
			StartTime:   timestamppb.New(updateStatus.LastUpdate.StartTime),
			EndTime:     timestamppb.New(updateStatus.LastUpdate.EndTime),
			Duration:    int64(updateStatus.LastUpdate.Duration.Seconds()),
			Success:     updateStatus.LastUpdate.Success,
			Error:       updateStatus.LastUpdate.Error,
		})
	}
	
	return response, nil
}

// SetUpgradePolicy configures automatic upgrade behavior
func (us *UpgradeService) SetUpgradePolicy(ctx context.Context, req *pb.SetUpgradePolicyRequest) (*pb.SetUpgradePolicyResponse, error) {
	// This would update the upgrade manager's policy
	// For now, return success
	response := &pb.SetUpgradePolicyResponse{
		Success: true,
	}
	
	return response, nil
}

// GetUpgradePolicy returns the current upgrade policy
func (us *UpgradeService) GetUpgradePolicy(ctx context.Context, req *pb.GetUpgradePolicyRequest) (*pb.GetUpgradePolicyResponse, error) {
	// Get current policy from upgrade manager
	// For now, return default policy
	response := &pb.GetUpgradePolicyResponse{
		AutoUpgrade:       false,
		CheckInterval:     int64(300), // 5 minutes
		Strategy:          "immediate",
		GracefulTimeout:   int64(30),  // 30 seconds
		RollbackOnFailure: true,
	}
	
	return response, nil
}

// WatchUpgradeStatus provides a stream of upgrade status updates
func (us *UpgradeService) WatchUpgradeStatus(req *pb.WatchUpgradeStatusRequest, stream pb.UpgradeService_WatchUpgradeStatusServer) error {
	ctx := stream.Context()
	
	// Initial status
	initialStatus := us.hotUpdateService.GetUpdateStatus()
	statusResponse := &pb.UpgradeStatusEvent{
		Timestamp:         timestamppb.Now(),
		EventType:         "status_update",
		CurrentGeneration: initialStatus.CurrentGeneration,
		IsHealthy:         initialStatus.IsHealthy,
	}
	
	if initialStatus.PendingUpdate != nil {
		statusResponse.PendingUpdate = &pb.PendingUpgrade{
			Id:            initialStatus.PendingUpdate.ID,
			FromVersion:   initialStatus.PendingUpdate.FromVersion,
			ToVersion:     initialStatus.PendingUpdate.ToVersion,
			Strategy:      initialStatus.PendingUpdate.Strategy,
			StartTime:     timestamppb.New(initialStatus.PendingUpdate.StartTime),
			Progress:      float32(initialStatus.PendingUpdate.Progress),
			Status:        initialStatus.PendingUpdate.Status,
			LastHeartbeat: timestamppb.New(initialStatus.PendingUpdate.LastHeartbeat),
		}
	}
	
	if err := stream.Send(statusResponse); err != nil {
		return err
	}
	
	// TODO: Implement real-time status streaming
	// This would typically use a channel or event system to push updates
	
	// For now, just keep the connection open and send periodic updates
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			currentStatus := us.hotUpdateService.GetUpdateStatus()
			event := &pb.UpgradeStatusEvent{
				Timestamp:         timestamppb.Now(),
				EventType:         "periodic_update",
				CurrentGeneration: currentStatus.CurrentGeneration,
				IsHealthy:         currentStatus.IsHealthy,
			}
			
			if currentStatus.PendingUpdate != nil {
				event.PendingUpdate = &pb.PendingUpgrade{
					Id:            currentStatus.PendingUpdate.ID,
					FromVersion:   currentStatus.PendingUpdate.FromVersion,
					ToVersion:     currentStatus.PendingUpdate.ToVersion,
					Strategy:      currentStatus.PendingUpdate.Strategy,
					StartTime:     timestamppb.New(currentStatus.PendingUpdate.StartTime),
					Progress:      float32(currentStatus.PendingUpdate.Progress),
					Status:        currentStatus.PendingUpdate.Status,
					LastHeartbeat: timestamppb.New(currentStatus.PendingUpdate.LastHeartbeat),
				}
			}
			
			if err := stream.Send(event); err != nil {
				return err
			}
			
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Helper functions

func (us *UpgradeService) convertStrategyToString(strategy pb.UpgradeStrategy) string {
	switch strategy {
	case pb.UpgradeStrategy_IMMEDIATE:
		return "immediate"
	case pb.UpgradeStrategy_GRACEFUL:
		return "graceful"
	case pb.UpgradeStrategy_SCHEDULED:
		return "scheduled"
	default:
		return ""
	}
}

// Import time for ticker
import "time"