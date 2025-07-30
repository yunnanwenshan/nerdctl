package service

import (
	"context"
	"time"
	"os"

	pb "dsagent/api/health/v1"
)

type HealthService struct {
	pb.UnimplementedHealthServer
}

func NewHealthService() *HealthService {
	return &HealthService{}
}

func (s *HealthService) Check(ctx context.Context, req *pb.HealthRequest) (*pb.HealthResponse, error) {
	// Simple health check - always return serving status
	return &pb.HealthResponse{
		Status:    pb.HealthResponse_SERVING,
		Message:   "Service is healthy",
		Timestamp: time.Now().Unix(),
	}, nil
}

func (s *HealthService) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	// Get server hostname as server ID
	serverId, err := os.Hostname()
	if err != nil {
		serverId = "unknown-server"
	}

	return &pb.HeartbeatResponse{
		Status:    "ok",
		Timestamp: time.Now().Unix(),
		ServerId:  serverId,
	}, nil
}