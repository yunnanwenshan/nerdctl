# nerdctl gRPC 服务解耦架构设计

## 架构原则

### 1. 解耦设计
- gRPC 服务独立于 nerdctl，通过适配器层交互
- 支持多个 nerdctl 版本，通过版本适配器实现
- 核心业务逻辑与具体实现解耦

### 2. 版本适配
- 支持 nerdctl v1.x 和 v2.x 版本
- 动态适配器选择机制
- 版本兼容性检测和警告

## 分层架构

```
┌─────────────────────────────────────────────────────────────┐
│                    gRPC Service Layer                       │
│  (container_service, image_service, network_service, etc.) │
└─────────────────────────┬───────────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────────┐
│                 Abstract Interface Layer                    │
│     (ContainerManager, ImageManager, NetworkManager)       │
└─────────────────────────┬───────────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────────┐
│                  Adapter Factory Layer                      │
│              (Version Detection & Selection)                │
└─────────────┬───────────────────────────────┬───────────────┘
              │                               │
┌─────────────▼─────────────┐    ┌───────────▼─────────────┐
│    nerdctl v1.x Adapter   │    │    nerdctl v2.x Adapter │
│                           │    │                         │
│  ├─ ContainerAdapterV1    │    │  ├─ ContainerAdapterV2  │
│  ├─ ImageAdapterV1        │    │  ├─ ImageAdapterV2      │
│  └─ NetworkAdapterV1      │    │  └─ NetworkAdapterV2    │
└─────────────┬─────────────┘    └───────────┬─────────────┘
              │                               │
┌─────────────▼─────────────┐    ┌───────────▼─────────────┐
│      nerdctl v1.x         │    │      nerdctl v2.x       │
│    (External Dependency)  │    │    (External Dependency) │
└───────────────────────────┘    └─────────────────────────┘
```

## 关键组件

### 1. Abstract Interface Layer
定义与 nerdctl 版本无关的通用接口，包含所有容器、镜像、网络等操作的抽象定义。

### 2. Adapter Factory
- 检测当前 nerdctl 版本
- 选择对应的适配器实现
- 提供适配器生命周期管理

### 3. Version-Specific Adapters
- 每个 nerdctl 版本的专门适配器
- 将通用接口适配到具体版本的 API
- 处理版本差异和兼容性问题

### 4. Configuration Management
- 支持多版本配置
- 适配器选择策略
- 兼容性检测配置

## 版本支持策略

### 支持的版本
- **nerdctl v1.7.x**: 通过 v1 适配器
- **nerdctl v2.0.x**: 通过 v2 适配器
- **Future versions**: 通过扩展新适配器

### 版本检测
1. 执行 `nerdctl version` 命令
2. 解析版本信息
3. 选择匹配的适配器
4. 验证功能兼容性

### 降级策略
- 新功能在旧版本中的降级处理
- 不支持功能的优雅错误返回
- 版本升级建议

## 扩展性设计

### 添加新版本支持
1. 实现新的适配器接口
2. 注册到适配器工厂
3. 添加版本检测规则
4. 更新配置和测试

### 添加新功能
1. 在抽象接口中添加方法
2. 在各版本适配器中实现
3. 处理版本兼容性
4. 更新 gRPC 服务层