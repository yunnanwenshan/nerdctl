package types

import "time"

// Common types used across different services

// NetworkConfig represents network configuration for containers
type NetworkConfig struct {
	Mode         string         `json:"mode,omitempty"`          // "bridge", "host", "none", "container"
	Networks     []string       `json:"networks,omitempty"`      // Network names to connect
	Ports        []*PortMapping `json:"ports,omitempty"`         // Port mappings
	DNSServers   []string       `json:"dns_servers,omitempty"`   // Custom DNS servers
	DNSSearch    []string       `json:"dns_search,omitempty"`    // DNS search domains
	DNSOptions   []string       `json:"dns_options,omitempty"`   // DNS options
	Hostname     string         `json:"hostname,omitempty"`      // Container hostname
	DomainName   string         `json:"domain_name,omitempty"`   // Container domain name
	IP           string         `json:"ip,omitempty"`            // IPv4 address
	IP6          string         `json:"ip6,omitempty"`           // IPv6 address
	MacAddress   string         `json:"mac_address,omitempty"`   // MAC address
	ExtraHosts   []string       `json:"extra_hosts,omitempty"`   // Extra hosts entries
	NetworkMode  string         `json:"network_mode,omitempty"`  // Network mode details
}

// PortMapping represents port mapping configuration
type PortMapping struct {
	HostIP           string `json:"host_ip,omitempty"`
	HostPort         int32  `json:"host_port"`
	ContainerPort    int32  `json:"container_port"`
	Protocol         string `json:"protocol,omitempty"`        // "tcp", "udp", "sctp"
	HostPortEnd      int32  `json:"host_port_end,omitempty"`   // End of host port range
	ContainerPortEnd int32  `json:"container_port_end,omitempty"` // End of container port range
}

// VolumeMount represents volume mount configuration
type VolumeMount struct {
	Source          string   `json:"source"`                     // Source path (host) or volume name
	Destination     string   `json:"destination"`                // Destination path in container
	Type            string   `json:"type,omitempty"`            // "bind", "volume", "tmpfs"
	ReadOnly        bool     `json:"read_only,omitempty"`       // Read-only mount
	BindPropagation string   `json:"bind_propagation,omitempty"` // Bind propagation mode
	TmpfsMode       string   `json:"tmpfs_mode,omitempty"`      // Tmpfs mode for tmpfs mounts
	TmpfsSize       int64    `json:"tmpfs_size,omitempty"`      // Tmpfs size in bytes
	MountOptions    []string `json:"mount_options,omitempty"`   // Additional mount options
}

// ResourceLimits represents resource limits and constraints
type ResourceLimits struct {
	MemoryBytes           int64         `json:"memory_bytes,omitempty"`           // Memory limit in bytes
	MemorySwapBytes       int64         `json:"memory_swap_bytes,omitempty"`      // Memory + swap limit
	CPUQuota              float64       `json:"cpu_quota,omitempty"`              // CPU quota (e.g., 1.5 for 150%)
	CPUPeriod             int64         `json:"cpu_period,omitempty"`             // CPU period in microseconds
	CPUShares             int32         `json:"cpu_shares,omitempty"`             // CPU shares (relative weight)
	CpusetCpus            string        `json:"cpuset_cpus,omitempty"`            // CPUs in which to allow execution
	CpusetMems            string        `json:"cpuset_mems,omitempty"`            // Memory nodes to allow execution
	BlkioWeight           int64         `json:"blkio_weight,omitempty"`           // Block IO weight
	BlkioWeightDevice     []*BlkioDevice `json:"blkio_weight_device,omitempty"`   // Per-device weight
	BlkioDeviceReadBps    []*BlkioDevice `json:"blkio_device_read_bps,omitempty"` // Per-device read rate
	BlkioDeviceWriteBps   []*BlkioDevice `json:"blkio_device_write_bps,omitempty"` // Per-device write rate
	KernelMemory          int64         `json:"kernel_memory,omitempty"`          // Kernel memory limit
	MemoryReservation     int64         `json:"memory_reservation,omitempty"`     // Memory soft limit
	MemorySwappiness      int32         `json:"memory_swappiness,omitempty"`      // Memory swappiness (0-100)
	OOMKillDisable        bool          `json:"oom_kill_disable,omitempty"`       // Disable OOM killer
	PidsLimit             int32         `json:"pids_limit,omitempty"`             // PIDs limit
	Ulimits               []*UlimitSpec `json:"ulimits,omitempty"`                // Ulimit specifications
}

// BlkioDevice represents block IO device configuration
type BlkioDevice struct {
	Path string `json:"path"` // Device path
	Rate int64  `json:"rate"` // Rate limit
}

// UlimitSpec represents ulimit specification
type UlimitSpec struct {
	Name string `json:"name"` // Ulimit name (e.g., "nofile", "nproc")
	Soft int64  `json:"soft"` // Soft limit
	Hard int64  `json:"hard"` // Hard limit
}

// SecurityOptions represents security configuration
type SecurityOptions struct {
	User             string   `json:"user,omitempty"`               // User specification (user:group)
	Groups           []string `json:"groups,omitempty"`             // Additional groups
	CapAdd           []string `json:"cap_add,omitempty"`            // Capabilities to add
	CapDrop          []string `json:"cap_drop,omitempty"`           // Capabilities to drop
	Privileged       bool     `json:"privileged,omitempty"`         // Run in privileged mode
	SecurityOpt      string   `json:"security_opt,omitempty"`       // Security options (legacy)
	SecurityOpts     []string `json:"security_opts,omitempty"`      // Security options
	NoNewPrivileges  bool     `json:"no_new_privileges,omitempty"`  // Prevent escalation
	UsernsMode       string   `json:"userns_mode,omitempty"`        // User namespace mode
}

// HealthCheck represents health check configuration
type HealthCheck struct {
	Test        []string      `json:"test,omitempty"`         // Health check command
	Interval    time.Duration `json:"interval,omitempty"`     // Check interval
	Timeout     time.Duration `json:"timeout,omitempty"`      // Check timeout
	Retries     int32         `json:"retries,omitempty"`      // Number of retries
	StartPeriod time.Duration `json:"start_period,omitempty"` // Start period before checks
}

// LogConfig represents logging configuration
type LogConfig struct {
	Driver  string            `json:"driver,omitempty"`  // Log driver name
	Options map[string]string `json:"options,omitempty"` // Log driver options
}

// RestartPolicy represents container restart policy
type RestartPolicy struct {
	Name              string `json:"name,omitempty"`                // "no", "always", "unless-stopped", "on-failure"
	MaximumRetryCount int32  `json:"maximum_retry_count,omitempty"` // Max retry count for "on-failure"
}

// Platform represents target platform specification
type Platform struct {
	Architecture string `json:"architecture,omitempty"` // Target architecture
	OS           string `json:"os,omitempty"`           // Target operating system
	Variant      string `json:"variant,omitempty"`      // Platform variant
}

// ImageInfo represents image information
type ImageInfo struct {
	ID           string            `json:"id"`
	Digest       string            `json:"digest,omitempty"`
	RepoTags     []string          `json:"repo_tags,omitempty"`
	RepoDigests  []string          `json:"repo_digests,omitempty"`
	Size         int64             `json:"size"`
	Created      time.Time         `json:"created"`
	Architecture string            `json:"architecture,omitempty"`
	OS           string            `json:"os,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	Env          []string          `json:"env,omitempty"`
	Cmd          []string          `json:"cmd,omitempty"`
	Entrypoint   []string          `json:"entrypoint,omitempty"`
	ExposedPorts []string          `json:"exposed_ports,omitempty"`
	Volumes      []string          `json:"volumes,omitempty"`
	WorkingDir   string            `json:"working_dir,omitempty"`
	User         string            `json:"user,omitempty"`
}

// OperationStatus represents the status of long-running operations
type OperationStatus string

const (
	OperationStatusPending     OperationStatus = "pending"
	OperationStatusRunning     OperationStatus = "running"
	OperationStatusCompleted   OperationStatus = "completed"
	OperationStatusFailed      OperationStatus = "failed"
	OperationStatusCancelled   OperationStatus = "cancelled"
)

// Progress represents operation progress information
type Progress struct {
	ID          string  `json:"id,omitempty"`
	Action      string  `json:"action,omitempty"`
	Status      string  `json:"status,omitempty"`
	Progress    string  `json:"progress,omitempty"`
	Current     int64   `json:"current,omitempty"`
	Total       int64   `json:"total,omitempty"`
	Percentage  float32 `json:"percentage,omitempty"`
}

// Error represents an error with additional context
type Error struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// Event represents system events
type Event struct {
	Type      string                 `json:"type"`
	Action    string                 `json:"action"`
	Actor     map[string]interface{} `json:"actor,omitempty"`
	Time      time.Time              `json:"time"`
	TimeNano  int64                  `json:"time_nano"`
}

// ContainerEvent represents container-specific events
type ContainerEvent struct {
	*Event
	ContainerID   string `json:"container_id"`
	ContainerName string `json:"container_name,omitempty"`
	Image         string `json:"image,omitempty"`
}

// ContainerStats represents container statistics
type ContainerStats struct {
	ContainerID string    `json:"container_id"`
	Name        string    `json:"name"`
	CPU         *CPUStats `json:"cpu,omitempty"`
	Memory      *MemoryStats `json:"memory,omitempty"`
	BlockIO     *BlockIOStats `json:"block_io,omitempty"`
	Network     *NetworkStats `json:"network,omitempty"`
	PIDs        *PIDStats `json:"pids,omitempty"`
	Read        time.Time `json:"read"`
}

// CPUStats represents CPU statistics
type CPUStats struct {
	Usage            *CPUUsage `json:"usage,omitempty"`
	SystemUsage      uint64    `json:"system_usage,omitempty"`
	OnlineCPUs       uint32    `json:"online_cpus,omitempty"`
	ThrottledTime    uint64    `json:"throttled_time,omitempty"`
}

// CPUUsage represents CPU usage statistics
type CPUUsage struct {
	TotalUsage        uint64   `json:"total_usage"`
	UsageInUsermode   uint64   `json:"usage_in_usermode"`
	UsageInKernelmode uint64   `json:"usage_in_kernelmode"`
	PerCPUUsage       []uint64 `json:"per_cpu_usage,omitempty"`
}

// MemoryStats represents memory statistics
type MemoryStats struct {
	Usage    uint64 `json:"usage"`
	MaxUsage uint64 `json:"max_usage"`
	Limit    uint64 `json:"limit"`
	Stats    map[string]uint64 `json:"stats,omitempty"`
}

// BlockIOStats represents block I/O statistics
type BlockIOStats struct {
	IoServiceBytesRecursive []*BlkioStatEntry `json:"io_service_bytes_recursive,omitempty"`
	IoServicedRecursive     []*BlkioStatEntry `json:"io_serviced_recursive,omitempty"`
}

// BlkioStatEntry represents a single block I/O stat entry
type BlkioStatEntry struct {
	Major uint64 `json:"major"`
	Minor uint64 `json:"minor"`
	Op    string `json:"op"`
	Value uint64 `json:"value"`
}

// NetworkStats represents network statistics
type NetworkStats struct {
	RxBytes   uint64 `json:"rx_bytes"`
	RxPackets uint64 `json:"rx_packets"`
	RxErrors  uint64 `json:"rx_errors"`
	RxDropped uint64 `json:"rx_dropped"`
	TxBytes   uint64 `json:"tx_bytes"`
	TxPackets uint64 `json:"tx_packets"`
	TxErrors  uint64 `json:"tx_errors"`
	TxDropped uint64 `json:"tx_dropped"`
}

// PIDStats represents PID statistics
type PIDStats struct {
	Current uint64 `json:"current,omitempty"`
	Limit   uint64 `json:"limit,omitempty"`
}