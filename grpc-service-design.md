# nerdctl gRPC Service Proxy Design Document

## 6. Error Handling and Status Management (IN PROGRESS)

### 6.1 Error Code System

#### gRPC Status Code Mapping
```go
// Error code mapping from nerdctl errors to gRPC status codes
const (
    // Success
    StatusOK = codes.OK
    
    // Client Errors (4xx equivalent)
    StatusInvalidArgument = codes.InvalidArgument  // Invalid parameters
    StatusNotFound       = codes.NotFound          // Container/image not found
    StatusAlreadyExists  = codes.AlreadyExists     // Resource already exists
    StatusPermissionDenied = codes.PermissionDenied // Access denied
    StatusUnauthenticated = codes.Unauthenticated  // Authentication failed
    
    // Server Errors (5xx equivalent)
    StatusInternal       = codes.Internal          // Internal server error
    StatusUnavailable    = codes.Unavailable       // Service unavailable
    StatusDeadlineExceeded = codes.DeadlineExceeded // Timeout
    StatusResourceExhausted = codes.ResourceExhausted // Resource limits
    
    // Specific containerd/nerdctl errors
    StatusContainerNotRunning = codes.FailedPrecondition
    StatusImagePullFailed     = codes.Unavailable
    StatusNetworkError        = codes.Internal
)
```

#### Custom Error Details
```protobuf
// Custom error details for richer error information
message ErrorDetail {
  string error_code = 1;        // Internal error code
  string message = 2;           // Human readable message
  string component = 3;         // Component that generated error (containerd, CNI, etc.)
  map<string, string> metadata = 4; // Additional context
  repeated string suggestions = 5;   // Suggested fixes
}

message ValidationError {
  repeated FieldError field_errors = 1;
}

message FieldError {
  string field = 1;
  string message = 2;
  string code = 3;
}
```

### 6.2 Connection and State Management

#### Connection Pool Management
```go
type ConnectionManager struct {
    containerdClient *containerd.Client
    cniManager       *netutil.CNIManager
    imageStore       *imgutil.Store
    volumeStore      *volumestore.VolumeStore
    
    // Connection health checking
    healthChecker    *HealthChecker
    reconnectPolicy  *ReconnectPolicy
}

type HealthChecker struct {
    interval        time.Duration
    timeout         time.Duration
    failureThreshold int
    listeners       []HealthListener
}

type ReconnectPolicy struct {
    maxRetries      int
    backoffStrategy BackoffStrategy
    circuitBreaker  *CircuitBreaker
}
```

#### State Synchronization
```go
// State management for long-running operations
type OperationState struct {
    ID          string
    Type        OperationType
    Status      OperationStatus
    Progress    *Progress
    Error       *ErrorDetail
    StartTime   time.Time
    UpdateTime  time.Time
    Metadata    map[string]string
}

type OperationManager struct {
    operations  sync.Map // map[string]*OperationState
    subscribers sync.Map // map[string][]chan *OperationState
    
    // Cleanup old operations
    cleaner     *StateCleaner
}
```

### 6.3 Context and Timeout Management

#### Request Context Handling
```go
type RequestContext struct {
    ctx           context.Context
    timeout       time.Duration
    namespace     string
    user          *AuthUser
    traceID       string
    operationID   string
    clientInfo    *ClientInfo
}

func (rc *RequestContext) WithTimeout(timeout time.Duration) *RequestContext {
    ctx, cancel := context.WithTimeout(rc.ctx, timeout)
    return &RequestContext{
        ctx:         ctx,
        timeout:     timeout,
        namespace:   rc.namespace,
        user:        rc.user,
        traceID:     rc.traceID,
        operationID: rc.operationID,
        clientInfo:  rc.clientInfo,
    }
}
```

#### Graceful Shutdown
```go
type ShutdownManager struct {
    servers         []GRPCServer
    activeRequests  sync.WaitGroup
    shutdownTimeout time.Duration
    gracePeriod     time.Duration
}

func (sm *ShutdownManager) Shutdown(ctx context.Context) error {
    // 1. Stop accepting new requests
    for _, server := range sm.servers {
        server.GracefulStop()
    }
    
    // 2. Wait for active requests to complete
    done := make(chan struct{})
    go func() {
        sm.activeRequests.Wait()
        close(done)
    }()
    
    // 3. Force shutdown if timeout exceeded
    select {
    case <-done:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

### 6.4 Error Recovery Mechanisms

#### Circuit Breaker Pattern
```go
type CircuitBreaker struct {
    state           CircuitState
    failureCount    int64
    failureThreshold int64
    resetTimeout    time.Duration
    lastFailTime    time.Time
    onStateChange   func(CircuitState)
}

func (cb *CircuitBreaker) Execute(fn func() error) error {
    state := cb.getState()
    
    switch state {
    case CircuitClosed:
        return cb.executeRequest(fn)
    case CircuitOpen:
        if time.Since(cb.lastFailTime) > cb.resetTimeout {
            cb.setState(CircuitHalfOpen)
            return cb.executeRequest(fn)
        }
        return ErrCircuitOpen
    case CircuitHalfOpen:
        return cb.executeRequest(fn)
    }
    
    return nil
}
```

#### Retry Strategy
```go
type RetryConfig struct {
    MaxAttempts    int
    InitialDelay   time.Duration
    MaxDelay       time.Duration
    Multiplier     float64
    RetryableErrors []codes.Code
}

func (rc *RetryConfig) ShouldRetry(err error) bool {
    status, ok := status.FromError(err)
    if !ok {
        return false
    }
    
    for _, code := range rc.RetryableErrors {
        if status.Code() == code {
            return true
        }
    }
    return false
}
```

## 7. Configuration and Deployment

### 7.1 Service Configuration

#### Configuration Structure
```go
type Config struct {
    Server     ServerConfig     `yaml:"server"`
    Containerd ContainerdConfig `yaml:"containerd"`
    Network    NetworkConfig    `yaml:"network"`
    Storage    StorageConfig    `yaml:"storage"`
    Security   SecurityConfig   `yaml:"security"`
    Logging    LoggingConfig    `yaml:"logging"`
    Monitoring MonitoringConfig `yaml:"monitoring"`
}

type ServerConfig struct {
    Address          string        `yaml:"address"`
    Port             int           `yaml:"port"`
    MaxConnections   int           `yaml:"max_connections"`
    RequestTimeout   time.Duration `yaml:"request_timeout"`
    KeepAlive        KeepAliveConfig `yaml:"keep_alive"`
}

type ContainerdConfig struct {
    Address     string `yaml:"address"`
    Namespace   string `yaml:"namespace"`
    PluginDir   string `yaml:"plugin_dir"`
    Root        string `yaml:"root"`
    State       string `yaml:"state"`
}
```

#### Environment-based Configuration
```yaml
# config.yaml
server:
  address: ${NERDCTL_GRPC_HOST:0.0.0.0}
  port: ${NERDCTL_GRPC_PORT:9090}
  max_connections: ${MAX_CONNECTIONS:1000}
  request_timeout: ${REQUEST_TIMEOUT:30s}

containerd:
  address: ${CONTAINERD_ADDRESS:/run/containerd/containerd.sock}
  namespace: ${CONTAINERD_NAMESPACE:default}

security:
  tls_enabled: ${TLS_ENABLED:true}
  cert_file: ${TLS_CERT_FILE:/etc/nerdctl-grpc/tls/server.crt}
  key_file: ${TLS_KEY_FILE:/etc/nerdctl-grpc/tls/server.key}
  ca_file: ${TLS_CA_FILE:/etc/nerdctl-grpc/tls/ca.crt}
```

### 7.2 Container Deployment

#### Docker Compose Deployment
```yaml
version: '3.8'

services:
  nerdctl-grpc:
    image: nerdctl-grpc:latest
    ports:
      - "9090:9090"
    volumes:
      - /run/containerd/containerd.sock:/run/containerd/containerd.sock
      - /var/lib/containerd:/var/lib/containerd
      - /etc/cni:/etc/cni
      - ./config:/etc/nerdctl-grpc
      - ./certs:/etc/nerdctl-grpc/tls
    environment:
      - CONTAINERD_ADDRESS=/run/containerd/containerd.sock
      - NERDCTL_GRPC_PORT=9090
      - TLS_ENABLED=true
    depends_on:
      - containerd
    restart: unless-stopped
    
  containerd:
    image: containerd/containerd:latest
    privileged: true
    volumes:
      - /var/lib/containerd:/var/lib/containerd
      - /run/containerd:/run/containerd
    restart: unless-stopped
```

#### Kubernetes Deployment
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nerdctl-grpc
  namespace: nerdctl-system
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nerdctl-grpc
  template:
    metadata:
      labels:
        app: nerdctl-grpc
    spec:
      serviceAccountName: nerdctl-grpc
      containers:
      - name: nerdctl-grpc
        image: nerdctl-grpc:v1.0.0
        ports:
        - containerPort: 9090
          name: grpc
        - containerPort: 9091
          name: metrics
        env:
        - name: CONTAINERD_ADDRESS
          value: /run/containerd/containerd.sock
        - name: NERDCTL_GRPC_PORT
          value: "9090"
        volumeMounts:
        - name: containerd-sock
          mountPath: /run/containerd
        - name: containerd-root
          mountPath: /var/lib/containerd
        - name: cni-config
          mountPath: /etc/cni
        - name: config
          mountPath: /etc/nerdctl-grpc
        - name: tls-certs
          mountPath: /etc/nerdctl-grpc/tls
        livenessProbe:
          grpc:
            port: 9090
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          grpc:
            port: 9090
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: containerd-sock
        hostPath:
          path: /run/containerd
      - name: containerd-root
        hostPath:
          path: /var/lib/containerd
      - name: cni-config
        hostPath:
          path: /etc/cni
      - name: config
        configMap:
          name: nerdctl-grpc-config
      - name: tls-certs
        secret:
          secretName: nerdctl-grpc-tls
---
apiVersion: v1
kind: Service
metadata:
  name: nerdctl-grpc
  namespace: nerdctl-system
spec:
  type: ClusterIP
  ports:
  - port: 9090
    targetPort: grpc
    name: grpc
  - port: 9091
    targetPort: metrics
    name: metrics
  selector:
    app: nerdctl-grpc
```

### 7.3 High Availability Setup

#### Load Balancer Configuration
```yaml
# HAProxy configuration for nerdctl-grpc
global:
  daemon
  
defaults:
  mode tcp
  timeout connect 5000ms
  timeout client 50000ms
  timeout server 50000ms

frontend nerdctl_grpc_frontend
  bind *:9090
  default_backend nerdctl_grpc_backend

backend nerdctl_grpc_backend
  balance roundrobin
  option tcp-check
  server grpc1 nerdctl-grpc-1:9090 check
  server grpc2 nerdctl-grpc-2:9090 check
  server grpc3 nerdctl-grpc-3:9090 check
```

#### Service Mesh Integration (Istio)
```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: nerdctl-grpc
spec:
  hosts:
  - nerdctl-grpc
  http:
  - match:
    - headers:
        content-type:
          prefix: "application/grpc"
    route:
    - destination:
        host: nerdctl-grpc
        port:
          number: 9090
    timeout: 300s
    retries:
      attempts: 3
      perTryTimeout: 30s
```

## 8. Monitoring and Logging

### 8.1 Metrics Collection

#### Prometheus Metrics
```go
type MetricsCollector struct {
    requestDuration *prometheus.HistogramVec
    requestTotal    *prometheus.CounterVec
    activeConnections prometheus.Gauge
    errorTotal      *prometheus.CounterVec
    operationDuration *prometheus.HistogramVec
}

func NewMetricsCollector() *MetricsCollector {
    return &MetricsCollector{
        requestDuration: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "nerdctl_grpc_request_duration_seconds",
                Help: "Duration of gRPC requests",
                Buckets: prometheus.DefBuckets,
            },
            []string{"method", "status"},
        ),
        requestTotal: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "nerdctl_grpc_requests_total",
                Help: "Total number of gRPC requests",
            },
            []string{"method", "status"},
        ),
        activeConnections: prometheus.NewGauge(
            prometheus.GaugeOpts{
                Name: "nerdctl_grpc_active_connections",
                Help: "Number of active gRPC connections",
            },
        ),
        errorTotal: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "nerdctl_grpc_errors_total",
                Help: "Total number of gRPC errors",
            },
            []string{"method", "error_code"},
        ),
    }
}
```

#### Custom Business Metrics
```go
// Container-specific metrics
var (
    containersTotal = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "nerdctl_containers_total",
            Help: "Total number of containers by state",
        },
        []string{"state", "namespace"},
    )
    
    imagesTotal = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "nerdctl_images_total",
            Help: "Total number of images",
        },
    )
    
    networkTotal = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "nerdctl_networks_total",
            Help: "Total number of networks",
        },
    )
    
    volumeTotal = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "nerdctl_volumes_total",
            Help: "Total number of volumes",
        },
    )
)
```

### 8.2 Logging Architecture

#### Structured Logging
```go
type Logger struct {
    *logrus.Logger
    traceProvider trace.TracerProvider
}

func (l *Logger) WithContext(ctx context.Context) *logrus.Entry {
    entry := l.WithContext(ctx)
    
    // Add trace information
    if span := trace.SpanFromContext(ctx); span.IsRecording() {
        spanCtx := span.SpanContext()
        entry = entry.WithFields(logrus.Fields{
            "trace_id": spanCtx.TraceID().String(),
            "span_id":  spanCtx.SpanID().String(),
        })
    }
    
    // Add request information
    if reqCtx := RequestContextFromContext(ctx); reqCtx != nil {
        entry = entry.WithFields(logrus.Fields{
            "namespace":    reqCtx.namespace,
            "user":         reqCtx.user.Username,
            "operation_id": reqCtx.operationID,
            "client_info":  reqCtx.clientInfo,
        })
    }
    
    return entry
}
```

#### Audit Logging
```go
type AuditLogger struct {
    logger    *Logger
    enabled   bool
    sensitive []string // Fields to mask in logs
}

type AuditEntry struct {
    Timestamp   time.Time         `json:"timestamp"`
    TraceID     string           `json:"trace_id"`
    User        string           `json:"user"`
    Method      string           `json:"method"`
    Resource    string           `json:"resource"`
    Action      string           `json:"action"`
    Result      string           `json:"result"`
    ErrorCode   string           `json:"error_code,omitempty"`
    Duration    time.Duration    `json:"duration"`
    ClientIP    string           `json:"client_ip"`
    UserAgent   string           `json:"user_agent"`
    Metadata    map[string]string `json:"metadata,omitempty"`
}

func (al *AuditLogger) LogRequest(ctx context.Context, req interface{}, resp interface{}, err error, duration time.Duration) {
    if !al.enabled {
        return
    }
    
    entry := &AuditEntry{
        Timestamp: time.Now(),
        Duration:  duration,
        Result:    "success",
    }
    
    if err != nil {
        entry.Result = "error"
        if status, ok := status.FromError(err); ok {
            entry.ErrorCode = status.Code().String()
        }
    }
    
    // Populate from context
    if reqCtx := RequestContextFromContext(ctx); reqCtx != nil {
        entry.TraceID = reqCtx.traceID
        entry.User = reqCtx.user.Username
    }
    
    // Extract method and resource information from request
    entry.Method, entry.Resource, entry.Action = al.extractRequestInfo(req)
    entry.Metadata = al.extractMetadata(req, resp)
    
    al.logger.WithFields(logrus.Fields{
        "audit": true,
        "entry": entry,
    }).Info("API request")
}
```

### 8.3 Distributed Tracing

#### OpenTelemetry Integration
```go
func InitTracing(serviceName, endpoint string) (trace.TracerProvider, error) {
    exporter, err := otlptracegrpc.New(
        context.Background(),
        otlptracegrpc.WithEndpoint(endpoint),
        otlptracegrpc.WithInsecure(),
    )
    if err != nil {
        return nil, err
    }
    
    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceNameKey.String(serviceName),
            semconv.ServiceVersionKey.String(version.Version),
        )),
    )
    
    otel.SetTracerProvider(tp)
    otel.SetTextMapPropagator(
        propagation.NewCompositeTextMapPropagator(
            propagation.TraceContext{},
            propagation.Baggage{},
        ),
    )
    
    return tp, nil
}
```

#### gRPC Interceptor for Tracing
```go
func TracingInterceptor(tracer trace.Tracer) grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
        ctx, span := tracer.Start(ctx, info.FullMethod,
            trace.WithSpanKind(trace.SpanKindServer),
            trace.WithAttributes(
                semconv.RPCSystemGRPC,
                semconv.RPCServiceKey.String(extractService(info.FullMethod)),
                semconv.RPCMethodKey.String(extractMethod(info.FullMethod)),
            ),
        )
        defer span.End()
        
        // Add request information to span
        span.SetAttributes(
            attribute.String("request.size", fmt.Sprintf("%d", proto.Size(req.(proto.Message)))),
        )
        
        resp, err := handler(ctx, req)
        
        if err != nil {
            span.RecordError(err)
            span.SetStatus(codes.Error, err.Error())
        } else {
            span.SetStatus(codes.Ok, "")
            span.SetAttributes(
                attribute.String("response.size", fmt.Sprintf("%d", proto.Size(resp.(proto.Message)))),
            )
        }
        
        return resp, err
    }
}
```

### 8.4 Health Checks and Monitoring

#### Health Check Implementation
```go
type HealthChecker struct {
    containerdClient *containerd.Client
    dependencies     map[string]HealthCheck
}

type HealthCheck interface {
    Check(ctx context.Context) error
    Name() string
}

func (hc *HealthChecker) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
    if req.Service == "" {
        // Overall service health
        return hc.checkOverallHealth(ctx)
    }
    
    // Specific service health
    return hc.checkServiceHealth(ctx, req.Service)
}

func (hc *HealthChecker) checkOverallHealth(ctx context.Context) (*grpc_health_v1.HealthCheckResponse, error) {
    // Check containerd connection
    if err := hc.containerdClient.IsServing(ctx); err != nil {
        return &grpc_health_v1.HealthCheckResponse{
            Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
        }, nil
    }
    
    // Check all dependencies
    for name, check := range hc.dependencies {
        if err := check.Check(ctx); err != nil {
            log.WithError(err).Warnf("Dependency %s failed health check", name)
            return &grpc_health_v1.HealthCheckResponse{
                Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
            }, nil
        }
    }
    
    return &grpc_health_v1.HealthCheckResponse{
        Status: grpc_health_v1.HealthCheckResponse_SERVING,
    }, nil
}
```

#### Alerting Configuration
```yaml
# Prometheus alerting rules
groups:
- name: nerdctl-grpc
  rules:
  - alert: NerdctlGrpcHighErrorRate
    expr: rate(nerdctl_grpc_errors_total[5m]) > 0.1
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "High error rate in nerdctl-grpc service"
      description: "Error rate is {{ $value }} errors per second"
      
  - alert: NerdctlGrpcHighLatency
    expr: histogram_quantile(0.95, rate(nerdctl_grpc_request_duration_seconds_bucket[5m])) > 1.0
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High latency in nerdctl-grpc service"
      description: "95th percentile latency is {{ $value }} seconds"
      
  - alert: NerdctlGrpcDown
    expr: up{job="nerdctl-grpc"} == 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "nerdctl-grpc service is down"
      description: "nerdctl-grpc service has been down for more than 1 minute"
```

---

## Design Completion Summary

The nerdctl gRPC Service Proxy design is now complete, covering all critical aspects:

### ✅ Completed Components:
1. **Overall Service Architecture** - Layered architecture with clear separation of concerns
2. **gRPC API Interfaces** - Complete protobuf definitions for all nerdctl functionality
3. **Service Proxy Layer** - Command mapping and adapter patterns
4. **Streaming Processing** - Handling long-running operations with proper streaming
5. **Authentication & Security** - JWT, RBAC, TLS with comprehensive security model
6. **Error Handling & Status Management** - Robust error mapping and state management
7. **Configuration & Deployment** - Production-ready deployment configurations
8. **Monitoring & Logging** - Comprehensive observability stack

### 🎯 Key Design Principles Achieved:
- **Docker Compatibility**: Complete API parity with nerdctl CLI
- **Production Ready**: Comprehensive security, monitoring, and deployment
- **Scalable Architecture**: Modular design supporting horizontal scaling
- **Cloud Native**: Kubernetes-ready with service mesh integration
- **Observable**: Full tracing, metrics, and audit logging
- **Resilient**: Circuit breakers, retries, and graceful degradation

The design provides a solid foundation for implementing a production-grade gRPC service that exposes all nerdctl functionality via a secure, scalable, and observable API.