package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/containerd/log"

	pb "dsagent/api/nydusify/v1"
)

// CallbackHandler handles HTTP callback notifications
type CallbackHandler struct {
	client *http.Client
}

// NewCallbackHandler creates a new callback handler
func NewCallbackHandler() *CallbackHandler {
	return &CallbackHandler{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SendCallback sends an HTTP callback with the provided payload
func (h *CallbackHandler) SendCallback(callback *pb.CallbackConfig, payload *pb.CallbackPayload) (bool, int, string, error) {
	if callback == nil || callback.Url == "" {
		return false, 0, "", fmt.Errorf("invalid callback configuration")
	}

	log.L.Infof("Sending HTTP callback to %s", callback.Url)

	// Marshal payload to JSON
	payloadBytes, err := h.marshalPayload(payload)
	if err != nil {
		return false, 0, "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create request
	ctx := context.Background()
	if callback.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(callback.TimeoutSeconds)*time.Second)
		defer cancel()
	}

	method := callback.Method
	if method == "" {
		method = "POST"
	}

	req, err := http.NewRequestWithContext(ctx, method, callback.Url, bytes.NewReader(payloadBytes))
	if err != nil {
		return false, 0, "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "dsagent-nydusify/1.0")

	// Add custom headers
	if callback.Headers != nil {
		for key, value := range callback.Headers {
			req.Header.Set(key, value)
		}
	}

	// Add authentication
	if err := h.addAuthentication(req, callback.Auth); err != nil {
		return false, 0, "", fmt.Errorf("failed to add authentication: %w", err)
	}

	// Send request
	resp, err := h.client.Do(req)
	if err != nil {
		return false, 0, "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.L.Warnf("Failed to read response body: %v", err)
		responseBody = []byte("failed to read response")
	}

	// Consider 2xx status codes as success
	success := resp.StatusCode >= 200 && resp.StatusCode < 300

	log.L.Infof("Callback response: status=%d, body=%s", resp.StatusCode, string(responseBody))

	return success, resp.StatusCode, string(responseBody), nil
}

// marshalPayload marshals the callback payload to JSON
func (h *CallbackHandler) marshalPayload(payload *pb.CallbackPayload) ([]byte, error) {
	// Convert protobuf to a more standard JSON format
	data := map[string]interface{}{
		"task_id":           payload.TaskId,
		"status":            payload.Status.String(),
		"container_id":      payload.ContainerId,
		"target_repo":       payload.TargetRepo,
		"completed_at":      payload.CompletedAt.AsTime().Format(time.RFC3339),
		"duration_seconds":  payload.DurationSeconds,
		"callback_attempt":  payload.CallbackAttempt,
	}

	if payload.ErrorMessage != "" {
		data["error_message"] = payload.ErrorMessage
	}

	if payload.Metadata != nil {
		data["metadata"] = payload.Metadata
	}

	if payload.Result != nil {
		resultData := map[string]interface{}{
			"exit_code":    payload.Result.ExitCode,
			"stdout":       payload.Result.Stdout,
			"stderr":       payload.Result.Stderr,
			"image_digest": payload.Result.ImageDigest,
			"target_url":   payload.Result.TargetUrl,
		}

		if payload.Result.SizeInfo != nil {
			resultData["size_info"] = map[string]interface{}{
				"original_size":     payload.Result.SizeInfo.OriginalSize,
				"nydus_size":        payload.Result.SizeInfo.NydusSize,
				"compression_ratio": payload.Result.SizeInfo.CompressionRatio,
				"size_reduction":    payload.Result.SizeInfo.SizeReduction,
			}
		}

		if payload.Result.Metrics != nil {
			resultData["metrics"] = map[string]interface{}{
				"cpu_usage":         payload.Result.Metrics.CpuUsage,
				"memory_usage":      payload.Result.Metrics.MemoryUsage,
				"disk_read_bytes":   payload.Result.Metrics.DiskReadBytes,
				"disk_write_bytes":  payload.Result.Metrics.DiskWriteBytes,
				"network_io_bytes":  payload.Result.Metrics.NetworkIoBytes,
			}
		}

		data["result"] = resultData
	}

	return json.Marshal(data)
}

// addAuthentication adds authentication to the request
func (h *CallbackHandler) addAuthentication(req *http.Request, auth *pb.CallbackAuth) error {
	if auth == nil {
		return nil
	}

	switch strings.ToLower(auth.Type) {
	case "bearer":
		if auth.Token != "" {
			req.Header.Set("Authorization", "Bearer "+auth.Token)
		}
	case "basic":
		if auth.Username != "" && auth.Password != "" {
			req.SetBasicAuth(auth.Username, auth.Password)
		}
	case "api_key":
		if auth.ApiKey != "" {
			headerName := auth.ApiKeyHeader
			if headerName == "" {
				headerName = "X-API-Key"
			}
			req.Header.Set(headerName, auth.ApiKey)
		}
	}

	return nil
}