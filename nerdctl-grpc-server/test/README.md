# nerdctl gRPC Server Testing Documentation

This document describes the comprehensive testing strategy for the nerdctl gRPC server project, including test organization, execution, and best practices.

## Table of Contents

- [Test Architecture](#test-architecture)
- [Test Types](#test-types)
- [Test Organization](#test-organization)
- [Running Tests](#running-tests)
- [Test Configuration](#test-configuration)
- [Mock System](#mock-system)
- [Integration Testing](#integration-testing)
- [Performance Testing](#performance-testing)
- [CI/CD Integration](#cicd-integration)
- [Best Practices](#best-practices)

## Test Architecture

Our testing strategy follows a layered approach that mirrors the application architecture:

```
Test Layer              | Coverage                    | Dependencies
------------------------|-----------------------------|-----------------
Unit Tests              | Individual components       | Mocks only
Integration Tests       | Component interactions      | Real nerdctl binaries
Stress Tests           | Load and reliability        | Real/mock systems
End-to-End Tests       | Full system workflows       | Complete environment
```

### Key Design Principles

1. **Decoupled Testing**: Tests for different adapter versions are completely independent
2. **Mock-First**: Unit tests use mocks to ensure fast, reliable execution
3. **Progressive Integration**: Integration tests validate real system interactions
4. **Performance Validation**: Stress tests ensure system reliability under load

## Test Types

### 1. Unit Tests

**Location**: `./adapters/`, `./mocks/`
**Purpose**: Test individual components in isolation
**Dependencies**: Mock objects only

```bash
# Run unit tests
make test-unit

# Run specific unit test
go test -v ./test/adapters/v1/executor_test.go
```

**Coverage**:
- Adapter factory logic
- Version detection
- Command execution
- Configuration management
- Error handling

### 2. Integration Tests

**Location**: `./integration/`
**Purpose**: Test interactions with real nerdctl binaries
**Dependencies**: Actual nerdctl installations

```bash
# Enable integration tests
export NERDCTL_INTEGRATION_TESTS=1

# Run integration tests
make test-integration

# Run with specific binary
NERDCTL_V2_PATH=/path/to/nerdctl make test-integration
```

**Coverage**:
- Real binary execution
- Version compatibility
- Command output parsing
- Error scenario handling
- Performance validation

### 3. Stress Tests

**Location**: `./stress/`
**Purpose**: Validate system behavior under load
**Dependencies**: Mock or real systems

```bash
# Run stress tests
make test-stress

# Run specific stress scenario
go test -v ./test/stress/ -run TestStress_ConcurrentAdapterCreation
```

**Coverage**:
- Concurrent adapter creation
- Memory leak detection  
- Long-running operations
- Context cancellation
- Resource cleanup

### 4. End-to-End Tests

**Location**: `./e2e/` (future)
**Purpose**: Complete workflow validation
**Dependencies**: Full system deployment

## Test Organization

### Directory Structure

```
test/
├── README.md                    # This documentation
├── config/
│   └── test_config.yaml        # Test configuration
├── adapters/                   # Adapter unit tests
│   ├── v1/
│   │   ├── executor_test.go
│   │   └── container_manager_test.go
│   ├── v2/
│   │   ├── executor_test.go
│   │   └── container_manager_test.go
│   └── factory/
│       └── factory_test.go
├── integration/                # Integration tests
│   └── adapter_integration_test.go
├── stress/                     # Stress tests
│   └── adapter_stress_test.go
├── mocks/                      # Mock implementations
│   ├── mock_executor.go
│   ├── mock_detector.go
│   └── mock_adapters.go
├── testdata/                   # Test data files
├── fixtures/                   # Test fixtures
└── test_runner.go             # Test orchestration
```

### Naming Conventions

- **Unit tests**: `*_test.go` in respective package directories
- **Integration tests**: `*_integration_test.go` 
- **Stress tests**: `*_stress_test.go`
- **Benchmark tests**: `*_benchmark_test.go`
- **Mock objects**: `mock_*.go`

## Running Tests

### Quick Start

```bash
# Install dependencies and set up development environment
make dev-setup

# Run all unit tests
make test

# Run complete test suite
make test-all

# Run quick tests (no race detection, no coverage)
make test-quick
```

### Test Environments

#### Development Environment

```bash
# Run development tests (fast feedback)
make dev-test

# Run with coverage
make test-cover

# Run with race detection
make test-race
```

#### CI Environment

```bash
# Run CI test suite
make ci-test

# Build for multiple platforms
make ci-build
```

#### Integration Environment

```bash
# Set up integration test environment
make test-env-setup

# Check environment prerequisites
make test-env-check

# Run integration tests
NERDCTL_INTEGRATION_TESTS=1 make test-integration
```

### Using the Test Runner

The test runner provides advanced test orchestration:

```bash
# Run tests using test runner
make test-with-runner

# Run specific test type
go run test/test_runner.go run unit
go run test/test_runner.go run integration
go run test/test_runner.go run stress
```

## Test Configuration

Tests are configured through `test/config/test_config.yaml`:

### Key Configuration Sections

- **Environment Settings**: Timeout, parallelism, coverage
- **Suite Definitions**: Test paths and requirements
- **Mock Configuration**: Auto-generation and interfaces
- **Binary Detection**: nerdctl binary discovery
- **Performance Settings**: Benchmark and load test parameters

### Environment Variables

```bash
# Enable integration tests
export NERDCTL_INTEGRATION_TESTS=1

# Specify binary paths
export NERDCTL_V1_PATH=/usr/bin/nerdctl
export NERDCTL_V2_PATH=/usr/local/bin/nerdctl

# Test configuration
export TEST_TIMEOUT=15m
export TEST_PARALLEL=true
export TEST_COVERAGE=true
```

## Mock System

Our mock system provides comprehensive test doubles for all external dependencies:

### Available Mocks

- **MockCommandExecutor**: Simulates nerdctl command execution
- **MockCompatibilityDetector**: Version detection simulation
- **MockAdapterFactory**: Adapter creation testing

### Mock Usage Example

```go
func TestContainerManager_CreateContainer(t *testing.T) {
    mockExecutor := &mocks.MockCommandExecutor{}
    manager := &v2.V2ContainerManager{Executor: mockExecutor}

    // Setup expectation
    mockExecutor.On("ExecuteCommand", mock.Anything, mock.MatchedBy(func(args []string) bool {
        return len(args) > 0 && args[0] == "run"
    })).Return(`{"id": "container-123", "name": "test"}`, nil)

    // Execute test
    result, err := manager.CreateContainer(ctx, config)
    
    // Verify
    assert.NoError(t, err)
    assert.Equal(t, "container-123", result.ID)
    mockExecutor.AssertExpectations(t)
}
```

## Integration Testing

Integration tests require actual nerdctl binaries and validate real system interactions:

### Prerequisites

1. **nerdctl Binaries**: v1.x and v2.x installations
2. **containerd**: Running containerd daemon
3. **Permissions**: Appropriate user permissions

### Binary Detection

The test system automatically discovers nerdctl binaries:

```bash
# Manual binary placement
mkdir -p ./tmp/nerdctl-v1 ./tmp/nerdctl-v2
# Place binaries in respective directories

# Automatic detection from PATH
# System automatically finds installed binaries
```

### Integration Test Scenarios

- **Adapter Creation**: Dynamic adapter selection
- **Container Lifecycle**: Create, start, stop, remove operations
- **Image Operations**: Pull, list, inspect, remove
- **Network Management**: Create, list, inspect, remove networks
- **Version Compatibility**: Cross-version operation validation

## Performance Testing

### Benchmarks

```bash
# Run all benchmarks
make bench

# Run specific benchmarks
go test -bench=BenchmarkAdapterCreation ./test/integration/

# Generate CPU and memory profiles
go test -bench=. -cpuprofile=cpu.prof -memprofile=mem.prof
```

### Stress Testing Scenarios

1. **Concurrent Adapter Creation**: Multiple simultaneous adapter instantiations
2. **Memory Leak Detection**: Long-running operation memory analysis
3. **Context Cancellation**: Proper cleanup under cancellation
4. **Resource Exhaustion**: Behavior under resource constraints

### Performance Metrics

- **Adapter Creation Time**: < 100ms per adapter
- **Memory Usage**: < 10MB increase per 1000 operations
- **Concurrent Operations**: Support 100+ simultaneous requests
- **Response Time**: < 1s for typical operations

## CI/CD Integration

### GitHub Actions Integration

```yaml
# Example workflow integration
- name: Run Tests
  run: |
    make test-unit
    make test-cover
    
- name: Integration Tests
  env:
    NERDCTL_INTEGRATION_TESTS: 1
  run: make test-integration
```

### Test Reports

Tests generate multiple report formats:
- **Console**: Real-time feedback
- **JUnit XML**: CI integration
- **HTML Reports**: Detailed analysis
- **JSON**: Machine processing

### Coverage Requirements

- **Minimum Coverage**: 80%
- **Critical Components**: 90%+
- **New Code**: 95%+

## Best Practices

### Writing Tests

1. **Test Naming**: Use descriptive test names
   ```go
   func TestV2ContainerManager_CreateContainer_WithResourceLimits(t *testing.T)
   ```

2. **Test Structure**: Follow Arrange-Act-Assert pattern
   ```go
   // Arrange
   mockExecutor := &mocks.MockCommandExecutor{}
   manager := &v2.V2ContainerManager{Executor: mockExecutor}
   
   // Act
   result, err := manager.CreateContainer(ctx, config)
   
   // Assert
   assert.NoError(t, err)
   assert.NotNil(t, result)
   ```

3. **Mock Usage**: Be specific with mock expectations
   ```go
   mockExecutor.On("ExecuteCommand", mock.Anything, 
       mock.MatchedBy(func(args []string) bool {
           return len(args) > 0 && args[0] == "run"
       })).Return(expectedOutput, nil)
   ```

### Test Maintenance

1. **Keep Tests Fast**: Unit tests should complete in milliseconds
2. **Isolate Dependencies**: Use mocks for external systems
3. **Clean Up Resources**: Ensure proper test cleanup
4. **Document Complex Scenarios**: Add comments for complex test logic

### Debugging Tests

```bash
# Run single test with verbose output
go test -v -run TestSpecificFunction ./test/adapters/v1/

# Run with race detection
go test -race ./...

# Generate test coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Performance Optimization

1. **Parallel Tests**: Use `t.Parallel()` for independent tests
2. **Test Caching**: Leverage Go test caching
3. **Mock Optimization**: Minimize mock setup complexity
4. **Resource Cleanup**: Prevent resource leaks

## Troubleshooting

### Common Issues

1. **Binary Not Found**: Ensure nerdctl is in PATH or NERDCTL_*_PATH is set
2. **Permission Denied**: Check user permissions for container operations
3. **Timeout Issues**: Increase timeout values for slow environments
4. **Flaky Tests**: Review test isolation and cleanup

### Debug Commands

```bash
# Check test environment
make test-env-check

# Validate test configuration
go run test/test_runner.go validate

# Run tests with debug output
go test -v -debug ./test/...
```

### Getting Help

1. Check the [test configuration](./config/test_config.yaml)
2. Review [test examples](./adapters/) 
3. Run environment validation: `make test-env-check`
4. Check mock setup in failing tests

---

For more information, see the [project documentation](../README.md) or [contributing guidelines](../CONTRIBUTING.md).