package types

import (
	"time"
)

// Container management request/response types
// These types serve as the internal representation that bridges gRPC protobuf messages
// and the nerdctl CLI command execution

// CreateContainerRequest represents a container creation request
type CreateContainerRequest struct {
	Image         string            `json:"image"`
	Name          string            `json:"name,omitempty"`
	Command       []string          `json:"command,omitempty"`
	Args          []string          `json:"args,omitempty"`
	Env           []string          `json:"env,omitempty"`
	WorkingDir    string            `json:"working_dir,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	NetworkConfig *NetworkConfig    `json:"network_config,omitempty"`
	Mounts        []*VolumeMount    `json:"mounts,omitempty"`
	Resources     *ResourceLimits   `json:"resources,omitempty"`
	Security      *SecurityOptions  `json:"security,omitempty"`
	HealthCheck   *HealthCheck      `json:"health_check,omitempty"`
	LogConfig     *LogConfig        `json:"log_config,omitempty"`
	RestartPolicy *RestartPolicy    `json:"restart_policy,omitempty"`
	Platform      *Platform         `json:"platform,omitempty"`
	Namespace     string            `json:"namespace,omitempty"`
	TTY           bool              `json:"tty,omitempty"`
	Stdin         bool              `json:"stdin,omitempty"`
	Init          bool              `json:"init,omitempty"`
	InitBinary    string            `json:"init_binary,omitempty"`
	CapAdd        []string          `json:"cap_add,omitempty"`
	CapDrop       []string          `json:"cap_drop,omitempty"`
	Devices       []string          `json:"devices,omitempty"`
	CgroupParent  string            `json:"cgroup_parent,omitempty"`
	IPCMode       string            `json:"ipc_mode,omitempty"`
	PidMode       string            `json:"pid_mode,omitempty"`
	UTSMode       string            `json:"uts_mode,omitempty"`
	Sysctl        []string          `json:"sysctl,omitempty"`
	Tmpfs         []string          `json:"tmpfs,omitempty"`
	ShmSize       string            `json:"shm_size,omitempty"`
	StopSignal    string            `json:"stop_signal,omitempty"`
	StopTimeout   int32             `json:"stop_timeout,omitempty"`
}

type CreateContainerResponse struct {
	ContainerID string   `json:"container_id"`
	Warnings    []string `json:"warnings,omitempty"`
}

// StartContainerRequest represents a container start request
type StartContainerRequest struct {
	ContainerID string   `json:"container_id"`
	Namespace   string   `json:"namespace,omitempty"`
	Attach      []string `json:"attach,omitempty"`
	DetachKeys  string   `json:"detach_keys,omitempty"`
}

type StartContainerResponse struct {
	Started  bool     `json:"started"`
	Warnings []string `json:"warnings,omitempty"`
}

// StopContainerRequest represents a container stop request
type StopContainerRequest struct {
	ContainerID string `json:"container_id"`
	Namespace   string `json:"namespace,omitempty"`
	Timeout     int32  `json:"timeout,omitempty"`
}

type StopContainerResponse struct {
	Stopped  bool     `json:"stopped"`
	Warnings []string `json:"warnings,omitempty"`
}

// RestartContainerRequest represents a container restart request
type RestartContainerRequest struct {
	ContainerID string `json:"container_id"`
	Namespace   string `json:"namespace,omitempty"`
	Timeout     int32  `json:"timeout,omitempty"`
}

type RestartContainerResponse struct {
	Restarted bool     `json:"restarted"`
	Warnings  []string `json:"warnings,omitempty"`
}

// RemoveContainerRequest represents a container removal request
type RemoveContainerRequest struct {
	ContainerID string `json:"container_id"`
	Namespace   string `json:"namespace,omitempty"`
	Force       bool   `json:"force,omitempty"`
	Volumes     bool   `json:"volumes,omitempty"`
	Link        bool   `json:"link,omitempty"`
}

type RemoveContainerResponse struct {
	Removed  bool     `json:"removed"`
	Warnings []string `json:"warnings,omitempty"`
}

// KillContainerRequest represents a container kill request
type KillContainerRequest struct {
	ContainerID string `json:"container_id"`
	Namespace   string `json:"namespace,omitempty"`
	Signal      string `json:"signal,omitempty"`
}

type KillContainerResponse struct {
	Killed   bool     `json:"killed"`
	Warnings []string `json:"warnings,omitempty"`
}

// PauseContainerRequest represents a container pause request
type PauseContainerRequest struct {
	ContainerID string `json:"container_id"`
	Namespace   string `json:"namespace,omitempty"`
}

type PauseContainerResponse struct {
	Paused   bool     `json:"paused"`
	Warnings []string `json:"warnings,omitempty"`
}

// UnpauseContainerRequest represents a container unpause request
type UnpauseContainerRequest struct {
	ContainerID string `json:"container_id"`
	Namespace   string `json:"namespace,omitempty"`
}

type UnpauseContainerResponse struct {
	Unpaused bool     `json:"unpaused"`
	Warnings []string `json:"warnings,omitempty"`
}

// RunContainerRequest represents a container run request (create + start)
type RunContainerRequest struct {
	*CreateContainerRequest
	Detach       bool   `json:"detach,omitempty"`
	Remove       bool   `json:"remove,omitempty"`
	Interactive  bool   `json:"interactive,omitempty"`
	Quiet        bool   `json:"quiet,omitempty"`
	Pull         string `json:"pull,omitempty"` // "always", "missing", "never"
	SigProxy     bool   `json:"sig_proxy,omitempty"`
	Platform     string `json:"platform,omitempty"`
	Entrypoint   string `json:"entrypoint,omitempty"`
}

type RunContainerResponse struct {
	ContainerID string   `json:"container_id"`
	ExitCode    int32    `json:"exit_code,omitempty"`
	Output      string   `json:"output,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
}

// RunContainerStreamResponse represents streaming output from container run
type RunContainerStreamResponse struct {
	Stream     string `json:"stream"`      // "stdout", "stderr"
	Data       []byte `json:"data"`
	Timestamp  time.Time `json:"timestamp"`
	Finished   bool   `json:"finished,omitempty"`
	ExitCode   int32  `json:"exit_code,omitempty"`
}

// ListContainersRequest represents a container list request
type ListContainersRequest struct {
	All       bool              `json:"all,omitempty"`
	Latest    bool              `json:"latest,omitempty"`
	Quiet     bool              `json:"quiet,omitempty"`
	Size      bool              `json:"size,omitempty"`
	Filters   map[string]string `json:"filters,omitempty"`
	Format    string            `json:"format,omitempty"`
	Namespace string            `json:"namespace,omitempty"`
	Limit     int32             `json:"limit,omitempty"`
}

type ListContainersResponse struct {
	Containers []*ContainerInfo `json:"containers"`
}

// InspectContainerRequest represents a container inspect request
type InspectContainerRequest struct {
	ContainerID string `json:"container_id"`
	Namespace   string `json:"namespace,omitempty"`
	Size        bool   `json:"size,omitempty"`
	Format      string `json:"format,omitempty"`
}

type InspectContainerResponse struct {
	Container *ContainerInfo `json:"container"`
}

// ContainerInfo represents detailed container information
type ContainerInfo struct {
	ID            string            `json:"id"`
	ShortID       string            `json:"short_id"`
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	ImageID       string            `json:"image_id"`
	Platform      string            `json:"platform"`
	Status        ContainerStatus   `json:"status"`
	ExitCode      int32             `json:"exit_code,omitempty"`
	Created       time.Time         `json:"created"`
	Started       time.Time         `json:"started,omitempty"`
	Finished      time.Time         `json:"finished,omitempty"`
	Command       []string          `json:"command"`
	Labels        map[string]string `json:"labels"`
	NetworkConfig *NetworkConfig    `json:"network_config,omitempty"`
	Mounts        []*VolumeMount    `json:"mounts,omitempty"`
	Resources     *ResourceLimits   `json:"resources,omitempty"`
	Security      *SecurityOptions  `json:"security,omitempty"`
	SizeRw        int64             `json:"size_rw,omitempty"`
	SizeRootFs    int64             `json:"size_root_fs,omitempty"`
}

// ContainerStatus represents container state
type ContainerStatus string

const (
	ContainerStatusCreated     ContainerStatus = "created"
	ContainerStatusRunning     ContainerStatus = "running"
	ContainerStatusPaused      ContainerStatus = "paused"
	ContainerStatusRestarting  ContainerStatus = "restarting"
	ContainerStatusRemoving    ContainerStatus = "removing"
	ContainerStatusExited      ContainerStatus = "exited"
	ContainerStatusDead        ContainerStatus = "dead"
)

// GetContainerLogsRequest represents a container logs request
type GetContainerLogsRequest struct {
	ContainerID string `json:"container_id"`
	Namespace   string `json:"namespace,omitempty"`
	Follow      bool   `json:"follow,omitempty"`
	Since       string `json:"since,omitempty"`
	Until       string `json:"until,omitempty"`
	Timestamps  bool   `json:"timestamps,omitempty"`
	Tail        string `json:"tail,omitempty"`
	Details     bool   `json:"details,omitempty"`
}

// LogEntry represents a single log entry
type LogEntry struct {
	Stream    string    `json:"stream"`    // "stdout", "stderr"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source,omitempty"`
}

// Additional request/response types for other operations would be defined here...