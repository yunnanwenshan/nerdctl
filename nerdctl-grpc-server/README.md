# nerdctl gRPC Server

A production-ready gRPC server that provides all nerdctl functionality via gRPC API.

## Features

- 🐳 **Complete nerdctl API**: Full compatibility with all nerdctl commands
- 🔒 **Enterprise Security**: JWT authentication, RBAC authorization, TLS encryption
- 📊 **Observability**: Prometheus metrics, structured logging, distributed tracing
- 🚀 **High Performance**: Connection pooling, streaming operations, async processing
- ☸️ **Cloud Native**: Kubernetes-ready with Helm charts and operators
- 🧪 **Well Tested**: Comprehensive unit, integration, and e2e test suites

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   gRPC Client   │    │  nerdctl-grpc    │    │   containerd    │
│                 │───▶│     Server       │───▶│                 │
│  (Any Language) │    │                  │    │  (Container     │
└─────────────────┘    └──────────────────┘    │   Runtime)      │
                                               └─────────────────┘
```

## Services

- **ContainerService**: Container lifecycle management (create, start, stop, etc.)
- **ImageService**: Image operations (pull, push, build, list, etc.)
- **NetworkService**: Network management (create, connect, disconnect, etc.)
- **VolumeService**: Volume operations (create, mount, unmount, etc.)
- **ComposeService**: Docker Compose compatibility
- **SystemService**: System information, events, cleanup
- **AuthService**: Authentication and authorization
- **NamespaceService**: Multi-tenancy support

## Quick Start

### Prerequisites

- Go 1.23+
- containerd running on system
- CNI plugins installed

### Build and Run

```bash
# Build
make build

# Run server
./bin/nerdctl-grpc-server --config config.yaml

# Run with development settings
./bin/nerdctl-grpc-server --dev
```

### Configuration

```yaml
# config.yaml
server:
  address: "0.0.0.0"
  port: 9090
  tls:
    enabled: true
    cert_file: "/etc/certs/server.crt"
    key_file: "/etc/certs/server.key"

containerd:
  address: "/run/containerd/containerd.sock"
  namespace: "default"

auth:
  enabled: true
  jwt_secret: "your-secret-key"
  
monitoring:
  prometheus_enabled: true
  metrics_port: 9091
```

## API Usage

### gRPC Client (Go)

```go
package main

import (
    "context"
    pb "github.com/containerd/nerdctl-grpc-server/api/proto"
    "google.golang.org/grpc"
)

func main() {
    conn, err := grpc.Dial("localhost:9090", grpc.WithInsecure())
    if err != nil {
        panic(err)
    }
    defer conn.Close()

    client := pb.NewContainerServiceClient(conn)
    
    // Run container
    resp, err := client.RunContainer(context.Background(), &pb.RunContainerRequest{
        Image: "nginx:latest",
        Detached: true,
        Ports: []*pb.PortMapping{{HostPort: 8080, ContainerPort: 80}},
    })
    
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Container created: %s\n", resp.ContainerId)
}
```

### gRPC Client (Python)

```python
import grpc
import container_service_pb2 as pb2
import container_service_pb2_grpc as pb2_grpc

def main():
    channel = grpc.insecure_channel('localhost:9090')
    stub = pb2_grpc.ContainerServiceStub(channel)
    
    request = pb2.RunContainerRequest(
        image="nginx:latest",
        detached=True,
        ports=[pb2.PortMapping(host_port=8080, container_port=80)]
    )
    
    response = stub.RunContainer(request)
    print(f"Container created: {response.container_id}")

if __name__ == "__main__":
    main()
```

## Testing

```bash
# Run all tests
make test

# Unit tests only
make test-unit

# Integration tests
make test-integration

# End-to-end tests
make test-e2e

# Coverage report
make coverage
```

## Deployment

### Docker

```bash
docker run -d \
  --name nerdctl-grpc \
  -p 9090:9090 \
  -v /run/containerd:/run/containerd \
  nerdctl-grpc:latest
```

### Kubernetes

```bash
# Install with Helm
helm install nerdctl-grpc ./deploy/helm/nerdctl-grpc

# Or apply manifests
kubectl apply -f deploy/k8s/
```

## Development

### Project Structure

```
nerdctl-grpc-server/
├── api/proto/           # Protocol buffer definitions
├── cmd/                 # Main applications
│   ├── server/         # gRPC server
│   └── client/         # Example client
├── internal/           # Private application code
│   ├── adapter/        # nerdctl integration adapters
│   ├── auth/           # Authentication & authorization  
│   ├── config/         # Configuration management
│   ├── metrics/        # Prometheus metrics
│   └── server/         # gRPC service implementations
├── pkg/                # Public libraries
│   ├── client/         # gRPC client library
│   └── types/          # Shared types
├── test/               # Test suites
├── deploy/             # Deployment configurations
└── scripts/            # Build and utility scripts
```

### Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## License

Apache License 2.0 - see [LICENSE](../LICENSE) file for details.