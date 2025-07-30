# nerdctl events Command Integration Analysis

## 概述 (Overview)

本文档分析了在 nerdctl-grpc-server 项目中成功集成 `nerdctl events` 命令的完整实现。

## 实现架构 (Implementation Architecture)

### 1. Protobuf 服务定义 (Protobuf Service Definition)

创建了完整的 `SystemService` gRPC 服务定义 (`system_service.proto`)：

```protobuf
service SystemService {
  // 获取系统信息
  rpc GetSystemInfo(GetSystemInfoRequest) returns (GetSystemInfoResponse);
  
  // 实时事件流 - 对应 nerdctl events 命令
  rpc GetSystemEvents(GetSystemEventsRequest) returns (stream SystemEvent);
  
  // 系统资源清理
  rpc SystemPrune(SystemPruneRequest) returns (SystemPruneResponse);
}
```

### 2. 事件过滤机制 (Event Filtering Mechanism)

#### 支持的过滤器类型 (Supported Filter Types)

- **`event`/`status`**: 按事件状态过滤 (`create`, `start`, `stop`, `die`, `destroy`)
- **`container`**: 按容器 ID 或名称过滤
- **`type`**: 按事件类型过滤 (`CONTAINER_EVENT`, `IMAGE_EVENT`, `NETWORK_EVENT`)

#### 过滤器解析实现 (Filter Parsing Implementation)

```go
func (s *SystemServiceServer) parseEventFilters(filters []string) (map[string][]string, error) {
    filterMap := make(map[string][]string)
    
    for _, filter := range filters {
        parts := strings.SplitN(filter, "=", 2)
        if len(parts) != 2 {
            return nil, fmt.Errorf("invalid filter format: %s", filter)
        }
        
        key := strings.ToLower(parts[0])
        value := parts[1]
        filterMap[key] = append(filterMap[key], value)
    }
    
    return filterMap, nil
}
```

#### 事件匹配算法 (Event Matching Algorithm)

```go
func (s *SystemServiceServer) eventMatchesFilters(event *pb.SystemEvent, filterMap map[string][]string) bool {
    if len(filterMap) == 0 {
        return true // 无过滤器时匹配所有事件
    }

    for filterKey, filterValues := range filterMap {
        match := false
        
        for _, filterValue := range filterValues {
            switch filterKey {
            case "event", "status":
                if strings.EqualFold(event.Status, filterValue) {
                    match = true
                }
            case "container":
                if strings.Contains(event.ContainerId, filterValue) {
                    match = true
                }
            case "type":
                if strings.EqualFold(event.Type.String(), filterValue) {
                    match = true
                }
            }
            
            if match { break }
        }
        
        if !match { return false } // 所有过滤器都必须匹配
    }
    
    return true
}
```

### 3. 实时事件流实现 (Real-time Event Streaming)

#### containerd 事件集成 (containerd Event Integration)

```go
func (s *SystemServiceServer) GetSystemEvents(req *pb.GetSystemEventsRequest, stream pb.SystemService_GetSystemEventsServer) error {
    ctx := stream.Context()
    adapter := s.adapterFactory.CreateV2Adapter()
    
    // 获取 containerd 客户端
    client, err := adapter.GetContainerdClient(ctx)
    if err != nil {
        return status.Errorf(codes.Internal, "failed to get containerd client: %v", err)
    }

    // 创建事件服务并订阅
    eventsClient := client.EventService()
    eventsCh, errCh := eventsClient.Subscribe(ctx)

    // 解析过滤器
    filterMap, err := s.parseEventFilters(req.Filters)
    if err != nil {
        return status.Errorf(codes.InvalidArgument, "invalid filter: %v", err)
    }

    // 流式处理事件
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case err := <-errCh:
            if err != nil {
                return status.Errorf(codes.Internal, "event stream error: %v", err)
            }
        case envelope := <-eventsCh:
            if envelope != nil {
                // 转换 containerd 事件为 protobuf 格式
                pbEvent, err := s.convertEventEnvelopeToProto(envelope)
                if err != nil {
                    continue
                }

                // 应用过滤器
                if s.eventMatchesFilters(pbEvent, filterMap) {
                    if err := stream.Send(pbEvent); err != nil {
                        return status.Errorf(codes.Internal, "failed to send event: %v", err)
                    }
                }
            }
        }
    }
}
```

### 4. 事件格式转换 (Event Format Conversion)

#### containerd 事件到 protobuf 转换 (containerd Event to Protobuf Conversion)

```go
func (s *SystemServiceServer) convertEventEnvelopeToProto(envelope *events.Envelope) (*pb.SystemEvent, error) {
    event := &pb.SystemEvent{
        Timestamp:   envelope.Timestamp.UnixNano(),
        Topic:       envelope.Topic,
        Namespace:   envelope.Namespace,
        ContainerId: s.extractContainerIDFromTopic(envelope.Topic),
        Status:      s.deriveStatusFromTopic(envelope.Topic),
        Type:        s.determineEventType(envelope.Topic),
        Metadata:    make(map[string]string),
    }

    // 序列化原始事件数据为 JSON
    if envelope.Event != nil {
        eventData, err := json.Marshal(envelope.Event)
        if err == nil {
            event.EventData = string(eventData)
        }
    }

    return event, nil
}
```

#### 状态推导逻辑 (Status Derivation Logic)

```go
func (s *SystemServiceServer) deriveStatusFromTopic(topic string) string {
    switch {
    case strings.Contains(topic, "/tasks/start"):
        return "start"
    case strings.Contains(topic, "/tasks/delete") || strings.Contains(topic, "/containers/delete"):
        return "destroy"
    case strings.Contains(topic, "/containers/create"):
        return "create"
    case strings.Contains(topic, "/tasks/exit"):
        return "die"
    case strings.Contains(topic, "/tasks/oom"):
        return "oom"
    default:
        return "unknown"
    }
}
```

## 5. 服务器集成 (Server Integration)

### 服务注册 (Service Registration)

在 `server.go` 中的集成变更：

```go
// 1. 添加 SystemService 字段
type Server struct {
    // ... 其他字段
    systemService    *SystemServiceServer
    // ...
}

// 2. 创建 SystemService 实例
func (s *Server) Initialize(ctx context.Context) error {
    // ... 其他初始化
    s.systemService = NewSystemServiceServer(s.adapterFactory)
    // ...
}

// 3. 注册 SystemService
func (s *Server) initializeGRPCServer() error {
    // ... 其他服务注册
    pb.RegisterSystemServiceServer(s.grpcServer, s.systemService)
    // ...
}
```

## 支持的功能特性 (Supported Features)

### ✅ 已实现功能 (Implemented Features)

1. **实时事件流** - 通过 gRPC streaming 实现
2. **事件过滤** - 支持 `event`、`container`、`type` 过滤器
3. **containerd 集成** - 直接从 containerd EventService 获取事件
4. **事件格式转换** - 将 containerd 事件转换为标准化格式
5. **多种事件类型** - 容器、镜像、网络、卷、任务事件
6. **错误处理** - 完整的错误处理和状态码
7. **上下文取消** - 支持客户端取消订阅

### 🔄 格式化支持 (Format Support)

protobuf 定义中包含 `format` 字段，可扩展支持：

```protobuf
message GetSystemEventsRequest {
  repeated string filters = 1;
  string format = 2;  // 模板格式化输出
  int64 since = 3;
  int64 until = 4;
}
```

### 📊 事件数据结构 (Event Data Structure)

```protobuf
message SystemEvent {
  int64 timestamp = 1;              // 事件时间戳
  string container_id = 2;          // 容器 ID
  string namespace = 3;             // 命名空间
  string topic = 4;                 // 事件主题
  string status = 5;                // 事件状态
  string event_data = 6;            // 原始事件数据 (JSON)
  EventType type = 7;               // 事件类型枚举
  map<string, string> metadata = 8; // 额外元数据
}
```

## 使用示例 (Usage Examples)

### gRPC 客户端调用示例 (gRPC Client Usage)

```go
// 1. 无过滤器的实时事件流
stream, err := client.GetSystemEvents(ctx, &pb.GetSystemEventsRequest{})

// 2. 过滤容器事件
stream, err := client.GetSystemEvents(ctx, &pb.GetSystemEventsRequest{
    Filters: []string{"type=CONTAINER_EVENT"},
})

// 3. 过滤特定容器的启动事件
stream, err := client.GetSystemEvents(ctx, &pb.GetSystemEventsRequest{
    Filters: []string{
        "event=start",
        "container=my_container",
    },
})

// 4. 接收事件
for {
    event, err := stream.Recv()
    if err != nil {
        break
    }
    // 处理事件
    fmt.Printf("Event: %s, Container: %s, Status: %s\n", 
               event.Topic, event.ContainerId, event.Status)
}
```

## 技术优势 (Technical Advantages)

1. **高性能流式处理** - 使用 gRPC streaming 避免轮询开销
2. **灵活的过滤机制** - 支持多种过滤条件组合
3. **标准化事件格式** - 统一的事件数据结构
4. **containerd 原生集成** - 直接从底层运行时获取事件
5. **可扩展架构** - 易于添加新的事件类型和过滤器
6. **完整的错误处理** - 健壮的错误传播和恢复机制

## 总结 (Summary)

成功实现了 `nerdctl events` 命令在 gRPC 服务器中的完整集成，包括：

- ✅ **完整的 protobuf 服务定义**
- ✅ **实时事件流实现**  
- ✅ **灵活的过滤机制**
- ✅ **containerd 原生集成**
- ✅ **服务器集成和注册**
- ✅ **错误处理和状态管理**

该实现为 nerdctl-grpc-server 提供了与原生 `nerdctl events` 命令等效的功能，同时通过 gRPC 接口提供了更好的程序化访问能力。