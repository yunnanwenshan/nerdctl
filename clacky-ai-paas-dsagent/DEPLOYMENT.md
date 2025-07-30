# dsagent - Kratos gRPC/HTTP Server

## Overview
This is a Go-Kratos based microservice that provides both gRPC and HTTP servers with health check and heartbeat functionality.

## Features
- **Dual Protocol Support**: Both gRPC and HTTP servers running simultaneously
- **Health Check Endpoint**: Monitor service health status
- **Heartbeat Endpoint**: Keep-alive and status monitoring
- **Dependency Injection**: Using Uber's dig for clean dependency management
- **Configuration Management**: YAML-based configuration with Kratos config

## Server Configuration
- **HTTP Server**: Running on `0.0.0.0:8000`
- **gRPC Server**: Running on `0.0.0.0:9000`

## API Endpoints

### HTTP Endpoints
1. **Health Check**: `GET /health`
   - Response: `{"status":"SERVING","message":"Service is healthy","timestamp":1753846127}`

2. **Heartbeat**: `GET /heartbeat`
   - Optional query parameter: `client_id`
   - Response: `{"status":"ok","timestamp":1753846131,"serverId":"clackyai-machine"}`

### gRPC Endpoints
1. **Health Check**: `health.v1.Health/Check`
   - Request: `{"service":"dsagent"}`
   - Response: `{"status":"SERVING","message":"Service is healthy","timestamp":"1753846140"}`

2. **Heartbeat**: `health.v1.Health/Heartbeat`
   - Request: `{"client_id":"test-client"}`
   - Response: `{"status":"ok","timestamp":"1753846143","serverId":"clackyai-machine"}`

## Build and Run

### Build
```bash
make build
```

### Run
```bash
./bin/dsagent -conf ./configs
```

### Test HTTP Endpoints
```bash
# Health check
curl -X GET http://localhost:8000/health

# Heartbeat
curl -X GET http://localhost:8000/heartbeat
curl -X GET "http://localhost:8000/heartbeat?client_id=test-client"
```

### Test gRPC Endpoints
```bash
# Health check
grpcurl -plaintext -d '{"service":"dsagent"}' localhost:9000 health.v1.Health/Check

# Heartbeat
grpcurl -plaintext -d '{"client_id":"test-client"}' localhost:9000 health.v1.Health/Heartbeat
```

## Project Structure
```
clacky-ai-paas-dsagent/
├── api/                    # Protocol Buffer definitions
│   └── health/v1/          # Health service API definitions
├── cmd/dsagent/            # Main application entry point
├── configs/                # Configuration files
├── internal/               # Internal application code
│   ├── biz/               # Business logic layer
│   ├── conf/              # Configuration structures
│   ├── container/         # Dependency injection container
│   ├── data/              # Data access layer
│   ├── server/            # Server setup (gRPC & HTTP)
│   └── service/           # Service implementations
└── bin/                   # Built binaries
```

## Dependencies
- **go-kratos/kratos/v2**: Main framework
- **uber/dig**: Dependency injection
- **google/protobuf**: Protocol Buffers
- **grpc**: gRPC support

## Docker Support
Build and run with Docker:
```bash
docker build -t dsagent .
docker run --rm -p 8000:8000 -p 9000:9000 dsagent
```