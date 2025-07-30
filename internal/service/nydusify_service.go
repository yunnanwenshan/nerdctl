package service

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/containerd/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "dsagent/api/nydusify/v1"
)

// NydusifyService implements the gRPC nydusify service interface
type NydusifyService struct {
	pb.UnimplementedNydusifyServiceServer
	
	// Task management
	taskManager *TaskManager
	
	// Background workers
	workerPool *WorkerPool
	
	// Callback handler
	callbackHandler *CallbackHandler
	
	// Nydusify executor
	nydusifyExecutor *NydusifyExecutor
}

// NewNydusifyService creates a new nydusify service instance
func NewNydusifyService() (*NydusifyService, error) {
	return &NydusifyService{
		taskManager:      NewTaskManager(),
		callbackHandler:  NewCallbackHandler(),
		nydusifyExecutor: NewNydusifyExecutor(),
		// workerPool will be initialized lazily when first task is submitted
	}, nil
}

// SubmitCommitTask submits a new nydusify commit task for async execution
func (s *NydusifyService) SubmitCommitTask(ctx context.Context, req *pb.SubmitCommitTaskRequest) (*pb.SubmitCommitTaskResponse, error) {
	log.L.Infof("Submitting nydusify commit task for container: %s, target: %s", req.ContainerId, req.TargetRepo)

	// Validate request
	if req.ContainerId == "" {
		return nil, fmt.Errorf("container_id is required")
	}
	if req.TargetRepo == "" {
		return nil, fmt.Errorf("target_repo is required")
	}

	// Create new task
	task := &Task{
		ID:             generateTaskID(),
		Type:           "commit",
		Status:         pb.TaskStatus_TASK_STATUS_PENDING,
		ContainerID:    req.ContainerId,
		TargetRepo:     req.TargetRepo,
		Priority:       req.Priority,
		TimeoutSeconds: req.TimeoutSeconds,
		CreatedAt:      time.Now(),
		Metadata:       req.Metadata,
		Callback:       req.Callback,
	}

	// Set defaults
	if task.Priority <= 0 {
		task.Priority = 5 // Default priority
	}
	if task.TimeoutSeconds <= 0 {
		task.TimeoutSeconds = 3600 // Default 1 hour timeout
	}

	// Store task
	s.taskManager.StoreTask(task)

	// Initialize worker pool if not already done
	if s.workerPool == nil {
		s.workerPool = NewWorkerPool(5, s)
		go s.workerPool.Start()
	}

	// Submit to worker pool
	s.workerPool.SubmitTask(task)

	log.L.Infof("Task %s submitted successfully", task.ID)

	return &pb.SubmitCommitTaskResponse{
		TaskId:    task.ID,
		Status:    task.Status,
		CreatedAt: timestamppb.New(task.CreatedAt),
		EstimatedStartTime: timestamppb.New(time.Now().Add(time.Minute)), // Estimate 1 minute
	}, nil
}

// GetTaskStatus retrieves task status and results
func (s *NydusifyService) GetTaskStatus(ctx context.Context, req *pb.GetTaskStatusRequest) (*pb.GetTaskStatusResponse, error) {
	log.L.Debugf("Getting status for task: %s", req.TaskId)

	task := s.taskManager.GetTask(req.TaskId)
	if task == nil {
		return nil, fmt.Errorf("task not found: %s", req.TaskId)
	}

	taskInfo := s.convertTaskToProto(task)

	return &pb.GetTaskStatusResponse{
		Task: taskInfo,
	}, nil
}

// ListTasks lists all tasks with optional filtering
func (s *NydusifyService) ListTasks(ctx context.Context, req *pb.ListTasksRequest) (*pb.ListTasksResponse, error) {
	log.L.Debugf("Listing tasks with filters")

	// Set defaults
	pageSize := req.PageSize
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	tasks := s.taskManager.ListTasks(&TaskFilter{
		Status:        req.StatusFilter,
		ContainerID:   req.ContainerId,
		CreatedAfter:  req.CreatedAfter.AsTime(),
		CreatedBefore: req.CreatedBefore.AsTime(),
		PageSize:      int(pageSize),
		PageToken:     req.PageToken,
	})

	// Convert to protobuf
	var protoTasks []*pb.TaskInfo
	for _, task := range tasks.Tasks {
		protoTasks = append(protoTasks, s.convertTaskToProto(task))
	}

	return &pb.ListTasksResponse{
		Tasks:         protoTasks,
		NextPageToken: tasks.NextPageToken,
		TotalCount:    int32(tasks.TotalCount),
	}, nil
}

// CancelTask cancels a running task
func (s *NydusifyService) CancelTask(ctx context.Context, req *pb.CancelTaskRequest) (*pb.CancelTaskResponse, error) {
	log.L.Infof("Cancelling task: %s", req.TaskId)

	task := s.taskManager.GetTask(req.TaskId)
	if task == nil {
		return &pb.CancelTaskResponse{
			Success: false,
			Message: "Task not found",
			Status:  pb.TaskStatus_TASK_STATUS_UNSPECIFIED,
		}, nil
	}

	// Only cancel pending or running tasks
	if task.Status != pb.TaskStatus_TASK_STATUS_PENDING && task.Status != pb.TaskStatus_TASK_STATUS_RUNNING {
		return &pb.CancelTaskResponse{
			Success: false,
			Message: fmt.Sprintf("Task cannot be cancelled, current status: %v", task.Status),
			Status:  task.Status,
		}, nil
	}

	// Cancel the task
	task.Status = pb.TaskStatus_TASK_STATUS_CANCELLED
	task.CompletedAt = time.Now()
	task.ErrorMessage = req.Reason
	s.taskManager.UpdateTask(task)

	// Cancel running execution if exists
	if task.CancelFunc != nil {
		task.CancelFunc()
	}

	// Also cancel in worker pool
	s.workerPool.CancelTask(req.TaskId)

	return &pb.CancelTaskResponse{
		Success: true,
		Message: "Task cancelled successfully",
		Status:  task.Status,
	}, nil
}

// RetryCallback retries the callback for a completed task
func (s *NydusifyService) RetryCallback(ctx context.Context, req *pb.RetryCallbackRequest) (*pb.RetryCallbackResponse, error) {
	log.L.Infof("Retrying callback for task: %s", req.TaskId)

	task := s.taskManager.GetTask(req.TaskId)
	if task == nil {
		return &pb.RetryCallbackResponse{
			Success: false,
			Message: "Task not found",
		}, nil
	}

	// Only retry callbacks for completed tasks
	if task.Status != pb.TaskStatus_TASK_STATUS_SUCCESS && task.Status != pb.TaskStatus_TASK_STATUS_FAILED {
		return &pb.RetryCallbackResponse{
			Success: false,
			Message: "Task is not in completed state",
		}, nil
	}

	// Reset callback status if force retry
	if req.Force {
		task.CallbackStatus.Attempts = 0
		task.CallbackStatus.Success = false
	}

	// Retry callback
	go s.sendCallback(task)

	return &pb.RetryCallbackResponse{
		Success:    true,
		Message:    "Callback retry initiated",
		RetryCount: int32(task.CallbackStatus.Attempts),
	}, nil
}

// convertTaskToProto converts internal Task to protobuf TaskInfo
func (s *NydusifyService) convertTaskToProto(task *Task) *pb.TaskInfo {
	taskInfo := &pb.TaskInfo{
		TaskId:         task.ID,
		TaskType:       task.Type,
		Status:         task.Status,
		ContainerId:    task.ContainerID,
		TargetRepo:     task.TargetRepo,
		CreatedAt:      timestamppb.New(task.CreatedAt),
		Priority:       task.Priority,
		TimeoutSeconds: task.TimeoutSeconds,
		Metadata:       task.Metadata,
		ErrorMessage:   task.ErrorMessage,
	}

	if !task.StartedAt.IsZero() {
		taskInfo.StartedAt = timestamppb.New(task.StartedAt)
	}
	if !task.CompletedAt.IsZero() {
		taskInfo.CompletedAt = timestamppb.New(task.CompletedAt)
		taskInfo.DurationSeconds = int32(task.CompletedAt.Sub(task.StartedAt).Seconds())
	}

	if task.Result != nil {
		taskInfo.Result = &pb.TaskResult{
			ExitCode:    int32(task.Result.ExitCode),
			Stdout:      task.Result.Stdout,
			Stderr:      task.Result.Stderr,
			ImageDigest: task.Result.ImageDigest,
			TargetUrl:   task.Result.TargetURL,
		}

		if task.Result.SizeInfo != nil {
			taskInfo.Result.SizeInfo = &pb.ImageSizeInfo{
				OriginalSize:     task.Result.SizeInfo.OriginalSize,
				NydusSize:        task.Result.SizeInfo.NydusSize,
				CompressionRatio: task.Result.SizeInfo.CompressionRatio,
				SizeReduction:    task.Result.SizeInfo.SizeReduction,
			}
		}

		if task.Result.Metrics != nil {
			taskInfo.Result.Metrics = &pb.ExecutionMetrics{
				CpuUsage:        task.Result.Metrics.CPUUsage,
				MemoryUsage:     task.Result.Metrics.MemoryUsage,
				DiskReadBytes:   task.Result.Metrics.DiskReadBytes,
				DiskWriteBytes:  task.Result.Metrics.DiskWriteBytes,
				NetworkIoBytes:  task.Result.Metrics.NetworkIOBytes,
			}
		}
	}

	if task.Callback != nil {
		taskInfo.Callback = task.Callback
	}

	if task.CallbackStatus != nil {
		taskInfo.CallbackStatus = &pb.CallbackStatus{
			Attempts:       int32(task.CallbackStatus.Attempts),
			LastStatusCode: int32(task.CallbackStatus.LastStatusCode),
			LastResponse:   task.CallbackStatus.LastResponse,
			LastError:      task.CallbackStatus.LastError,
			Success:        task.CallbackStatus.Success,
		}

		if !task.CallbackStatus.LastAttempt.IsZero() {
			taskInfo.CallbackStatus.LastAttempt = timestamppb.New(task.CallbackStatus.LastAttempt)
		}
		if !task.CallbackStatus.NextRetry.IsZero() {
			taskInfo.CallbackStatus.NextRetry = timestamppb.New(task.CallbackStatus.NextRetry)
		}
	}

	return taskInfo
}

// sendCallback sends callback notification to the configured URL
func (s *NydusifyService) sendCallback(task *Task) {
	if task.Callback == nil || task.Callback.Url == "" {
		return
	}

	log.L.Infof("Sending callback for task %s to %s", task.ID, task.Callback.Url)

	// Initialize callback status if not exists
	if task.CallbackStatus == nil {
		task.CallbackStatus = &CallbackStatus{}
	}

	// Check retry limits
	maxAttempts := int(task.Callback.Retry.MaxAttempts)
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	if task.CallbackStatus.Attempts >= maxAttempts && !task.CallbackStatus.Success {
		log.L.Warnf("Max callback attempts reached for task %s", task.ID)
		return
	}

	// Increment attempts
	task.CallbackStatus.Attempts++
	task.CallbackStatus.LastAttempt = time.Now()

	// Create payload
	payload := &pb.CallbackPayload{
		TaskId:          task.ID,
		Status:          task.Status,
		ContainerId:     task.ContainerID,
		TargetRepo:      task.TargetRepo,
		CompletedAt:     timestamppb.New(task.CompletedAt),
		DurationSeconds: int32(task.CompletedAt.Sub(task.StartedAt).Seconds()),
		ErrorMessage:    task.ErrorMessage,
		Metadata:        task.Metadata,
		CallbackAttempt: int32(task.CallbackStatus.Attempts),
	}

	if task.Result != nil {
		payload.Result = &pb.TaskResult{
			ExitCode:    int32(task.Result.ExitCode),
			Stdout:      task.Result.Stdout,
			Stderr:      task.Result.Stderr,
			ImageDigest: task.Result.ImageDigest,
			TargetUrl:   task.Result.TargetURL,
		}
	}

	// Send HTTP callback using callback handler
	success, statusCode, response, err := s.callbackHandler.SendCallback(task.Callback, payload)

	// Update callback status
	task.CallbackStatus.LastStatusCode = statusCode
	task.CallbackStatus.LastResponse = response
	task.CallbackStatus.Success = success

	if err != nil {
		task.CallbackStatus.LastError = err.Error()
		log.L.Errorf("Callback failed for task %s (attempt %d): %v", task.ID, task.CallbackStatus.Attempts, err)
	} else {
		log.L.Infof("Callback succeeded for task %s (attempt %d): status=%d", task.ID, task.CallbackStatus.Attempts, statusCode)
	}

	// Update task in manager
	s.taskManager.UpdateTask(task)

	// Schedule retry if failed and retries remaining
	if !success && task.CallbackStatus.Attempts < maxAttempts {
		retryDelay := time.Duration(task.Callback.Retry.DelaySeconds) * time.Second
		if retryDelay <= 0 {
			// Use exponential backoff
			retryDelay = time.Duration(task.CallbackStatus.Attempts*task.CallbackStatus.Attempts) * time.Second
		}

		task.CallbackStatus.NextRetry = time.Now().Add(retryDelay)
		
		log.L.Infof("Scheduling callback retry for task %s in %v", task.ID, retryDelay)
		
		// Schedule retry
		go func() {
			time.Sleep(retryDelay)
			s.sendCallback(task)
		}()
	}
}

// Task represents a nydusify task
type Task struct {
	ID             string               `json:"id"`
	Type           string               `json:"type"`
	Status         pb.TaskStatus        `json:"status"`
	ContainerID    string               `json:"container_id"`
	TargetRepo     string               `json:"target_repo"`
	Priority       int32                `json:"priority"`
	TimeoutSeconds int32                `json:"timeout_seconds"`
	CreatedAt      time.Time            `json:"created_at"`
	StartedAt      time.Time            `json:"started_at"`
	CompletedAt    time.Time            `json:"completed_at"`
	Metadata       map[string]string    `json:"metadata"`
	ErrorMessage   string               `json:"error_message"`
	Result         *TaskResult          `json:"result"`
	Callback       *pb.CallbackConfig   `json:"callback"`
	CallbackStatus *CallbackStatus      `json:"callback_status"`
	
	// Runtime fields (not serialized)
	CancelFunc context.CancelFunc `json:"-"`
}

// TaskResult represents the result of a nydusify execution
type TaskResult struct {
	ExitCode    int                `json:"exit_code"`
	Stdout      string             `json:"stdout"`
	Stderr      string             `json:"stderr"`
	ImageDigest string             `json:"image_digest"`
	TargetURL   string             `json:"target_url"`
	SizeInfo    *ImageSizeInfo     `json:"size_info"`
	Metrics     *ExecutionMetrics  `json:"metrics"`
}

// ImageSizeInfo represents size information about the processed image
type ImageSizeInfo struct {
	OriginalSize     int64   `json:"original_size"`
	NydusSize        int64   `json:"nydus_size"`
	CompressionRatio float32 `json:"compression_ratio"`
	SizeReduction    float32 `json:"size_reduction"`
}

// ExecutionMetrics represents execution metrics
type ExecutionMetrics struct {
	CPUUsage        float32 `json:"cpu_usage"`
	MemoryUsage     int64   `json:"memory_usage"`
	DiskReadBytes   int64   `json:"disk_read_bytes"`
	DiskWriteBytes  int64   `json:"disk_write_bytes"`
	NetworkIOBytes  int64   `json:"network_io_bytes"`
}

// CallbackStatus represents the status of callback attempts
type CallbackStatus struct {
	Attempts       int       `json:"attempts"`
	LastStatusCode int       `json:"last_status_code"`
	LastResponse   string    `json:"last_response"`
	LastError      string    `json:"last_error"`
	LastAttempt    time.Time `json:"last_attempt"`
	NextRetry      time.Time `json:"next_retry"`
	Success        bool      `json:"success"`
}

// TaskManager manages task storage and retrieval
type TaskManager struct {
	mu    sync.RWMutex
	tasks map[string]*Task
}

// NewTaskManager creates a new task manager
func NewTaskManager() *TaskManager {
	return &TaskManager{
		tasks: make(map[string]*Task),
	}
}

// StoreTask stores a task
func (tm *TaskManager) StoreTask(task *Task) {
	tm.mu.Lock()
	tm.tasks[task.ID] = task
	tm.mu.Unlock()
}

// GetTask retrieves a task by ID
func (tm *TaskManager) GetTask(id string) *Task {
	tm.mu.RLock()
	task := tm.tasks[id]
	tm.mu.RUnlock()
	return task
}

// UpdateTask updates a task
func (tm *TaskManager) UpdateTask(task *Task) {
	tm.StoreTask(task)
}

// TaskFilter represents task filtering options
type TaskFilter struct {
	Status        pb.TaskStatus `json:"status"`
	ContainerID   string        `json:"container_id"`
	CreatedAfter  time.Time     `json:"created_after"`
	CreatedBefore time.Time     `json:"created_before"`
	PageSize      int           `json:"page_size"`
	PageToken     string        `json:"page_token"`
}

// TaskList represents a paginated list of tasks
type TaskList struct {
	Tasks         []*Task `json:"tasks"`
	NextPageToken string  `json:"next_page_token"`
	TotalCount    int     `json:"total_count"`
}

// ListTasks lists tasks with filtering
func (tm *TaskManager) ListTasks(filter *TaskFilter) *TaskList {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var filteredTasks []*Task
	
	for _, task := range tm.tasks {
		// Apply filters
		if filter.Status != pb.TaskStatus_TASK_STATUS_UNSPECIFIED && task.Status != filter.Status {
			continue
		}
		if filter.ContainerID != "" && task.ContainerID != filter.ContainerID {
			continue
		}
		if !filter.CreatedAfter.IsZero() && task.CreatedAt.Before(filter.CreatedAfter) {
			continue
		}
		if !filter.CreatedBefore.IsZero() && task.CreatedAt.After(filter.CreatedBefore) {
			continue
		}
		
		filteredTasks = append(filteredTasks, task)
	}

	// Simple pagination (in real implementation, would be more sophisticated)
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	
	totalCount := len(filteredTasks)
	startIdx := 0
	
	// For simplicity, just return first page
	endIdx := startIdx + pageSize
	if endIdx > totalCount {
		endIdx = totalCount
	}
	
	var pageItems []*Task
	if startIdx < totalCount {
		pageItems = filteredTasks[startIdx:endIdx]
	}

	return &TaskList{
		Tasks:         pageItems,
		NextPageToken: "", // Simplified - no pagination tokens
		TotalCount:    totalCount,
	}
}

// WorkerPool manages background workers for task execution
type WorkerPool struct {
	workerCount int
	taskQueue   chan *Task
	workers     []*Worker
	cancelTasks map[string]context.CancelFunc
	mu          sync.RWMutex
	service     *NydusifyService
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(workerCount int, service *NydusifyService) *WorkerPool {
	return &WorkerPool{
		workerCount: workerCount,
		taskQueue:   make(chan *Task, 100), // Buffered queue
		workers:     make([]*Worker, workerCount),
		cancelTasks: make(map[string]context.CancelFunc),
		service:     service,
	}
}

// Start starts all workers in the pool
func (wp *WorkerPool) Start() {
	for i := 0; i < wp.workerCount; i++ {
		worker := NewWorker(i, wp.taskQueue, wp.service)
		wp.workers[i] = worker
		go worker.Start()
	}
	log.L.Infof("Started %d workers in worker pool", wp.workerCount)
}

// SubmitTask submits a task to the worker pool
func (wp *WorkerPool) SubmitTask(task *Task) {
	select {
	case wp.taskQueue <- task:
		log.L.Debugf("Task %s submitted to worker pool", task.ID)
	default:
		log.L.Warnf("Worker pool queue is full, task %s dropped", task.ID)
	}
}

// CancelTask cancels a running task
func (wp *WorkerPool) CancelTask(taskID string) {
	wp.mu.RLock()
	cancelFunc, exists := wp.cancelTasks[taskID]
	wp.mu.RUnlock()
	
	if exists && cancelFunc != nil {
		cancelFunc()
		
		wp.mu.Lock()
		delete(wp.cancelTasks, taskID)
		wp.mu.Unlock()
		
		log.L.Infof("Task %s cancelled", taskID)
	}
}

// registerCancelFunc registers a cancel function for a task
func (wp *WorkerPool) registerCancelFunc(taskID string, cancelFunc context.CancelFunc) {
	wp.mu.Lock()
	wp.cancelTasks[taskID] = cancelFunc
	wp.mu.Unlock()
}

// unregisterCancelFunc unregisters a cancel function for a task
func (wp *WorkerPool) unregisterCancelFunc(taskID string) {
	wp.mu.Lock()
	delete(wp.cancelTasks, taskID)
	wp.mu.Unlock()
}

// Worker represents a background worker
type Worker struct {
	id        int
	taskQueue <-chan *Task
	service   *NydusifyService
}

// NewWorker creates a new worker
func NewWorker(id int, taskQueue <-chan *Task, service *NydusifyService) *Worker {
	return &Worker{
		id:        id,
		taskQueue: taskQueue,
		service:   service,
	}
}

// Start starts the worker
func (w *Worker) Start() {
	log.L.Infof("Worker %d started", w.id)
	
	for task := range w.taskQueue {
		log.L.Infof("Worker %d processing task %s", w.id, task.ID)
		w.processTask(task)
	}
}

// processTask processes a single task
func (w *Worker) processTask(task *Task) {
	// Update task status
	task.Status = pb.TaskStatus_TASK_STATUS_RUNNING
	task.StartedAt = time.Now()

	// Create execution context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(task.TimeoutSeconds)*time.Second)
	defer cancel()

	task.CancelFunc = cancel

	// Execute nydusify command
	result, err := w.executeNydusifyCommand(ctx, task)
	
	// Update task with result
	if err != nil {
		task.Status = pb.TaskStatus_TASK_STATUS_FAILED
		task.ErrorMessage = err.Error()
		log.L.Errorf("Task %s failed: %v", task.ID, err)
	} else {
		task.Status = pb.TaskStatus_TASK_STATUS_SUCCESS
		task.Result = result
		log.L.Infof("Task %s completed successfully", task.ID)
	}
	
	task.CompletedAt = time.Now()
	task.CancelFunc = nil
	
	// Send callback notification if configured
	if task.Callback != nil && task.Callback.Url != "" {
		// Schedule callback in background
		go func() {
			time.Sleep(1 * time.Second) // Brief delay to ensure task is updated
			w.service.sendCallback(task)
		}()
	}
	
	log.L.Infof("Worker %d finished processing task %s with status %v", w.id, task.ID, task.Status)
}

// executeNydusifyCommand executes the nydusify commit command
func (w *Worker) executeNydusifyCommand(ctx context.Context, task *Task) (*TaskResult, error) {
	// Use the dedicated nydusify executor for better handling
	executor := NewNydusifyExecutor()
	return executor.ExecuteCommit(ctx, task.ContainerID, task.TargetRepo)
}

// generateTaskID generates a unique task ID
func generateTaskID() string {
	return fmt.Sprintf("task-%d-%d", time.Now().UnixNano(), rand.Intn(10000))
}