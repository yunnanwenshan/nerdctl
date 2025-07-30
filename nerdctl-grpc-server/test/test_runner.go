package test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/containerd/nerdctl-grpc-server/internal/config"
	"github.com/containerd/nerdctl-grpc-server/internal/compatibility"
)

// TestRunner orchestrates different types of tests based on environment and configuration
type TestRunner struct {
	config          *config.Config
	testEnvironment string
	availableBinaries map[string]string
	testTimeout     time.Duration
}

// NewTestRunner creates a new test runner with environment detection
func NewTestRunner() *TestRunner {
	return &TestRunner{
		config: &config.Config{
			LogLevel:        "info",
			AdapterStrategy: "auto",
		},
		testEnvironment:   detectTestEnvironment(),
		availableBinaries: detectAvailableBinaries(),
		testTimeout:       5 * time.Minute,
	}
}

// RunTestSuite executes the appropriate test suite based on environment
func (tr *TestRunner) RunTestSuite(t *testing.T, suiteType string) {
	switch suiteType {
	case "unit":
		tr.runUnitTests(t)
	case "integration":
		tr.runIntegrationTests(t)
	case "stress":
		tr.runStressTests(t)
	case "all":
		tr.runAllTests(t)
	default:
		t.Fatalf("Unknown test suite type: %s", suiteType)
	}
}

func (tr *TestRunner) runUnitTests(t *testing.T) {
	t.Log("Running unit tests...")
	
	// Unit tests don't require real binaries
	testPaths := []string{
		"./adapters/v1",
		"./adapters/v2", 
		"./adapters/factory",
		"./mocks",
	}
	
	for _, path := range testPaths {
		t.Run(fmt.Sprintf("unit_%s", strings.ReplaceAll(path, "/", "_")), func(t *testing.T) {
			tr.runGoTest(t, path, []string{"-v", "-race", "-cover"})
		})
	}
}

func (tr *TestRunner) runIntegrationTests(t *testing.T) {
	if tr.testEnvironment != "integration" && tr.testEnvironment != "ci" {
		t.Skip("Integration tests require NERDCTL_INTEGRATION_TESTS=1 or CI environment")
	}
	
	t.Log("Running integration tests...")
	
	// Check for available binaries
	if len(tr.availableBinaries) == 0 {
		t.Skip("No nerdctl binaries found for integration testing")
	}
	
	// Set integration test environment variable
	os.Setenv("NERDCTL_INTEGRATION_TESTS", "1")
	defer os.Unsetenv("NERDCTL_INTEGRATION_TESTS")
	
	testPaths := []string{
		"./integration",
	}
	
	for _, path := range testPaths {
		t.Run(fmt.Sprintf("integration_%s", strings.ReplaceAll(path, "/", "_")), func(t *testing.T) {
			tr.runGoTest(t, path, []string{"-v", "-timeout", "10m"})
		})
	}
}

func (tr *TestRunner) runStressTests(t *testing.T) {
	if tr.testEnvironment == "ci" && !strings.Contains(os.Getenv("CI_TEST_TYPES"), "stress") {
		t.Skip("Stress tests disabled in CI")
	}
	
	t.Log("Running stress tests...")
	
	testPaths := []string{
		"./stress",
	}
	
	for _, path := range testPaths {
		t.Run(fmt.Sprintf("stress_%s", strings.ReplaceAll(path, "/", "_")), func(t *testing.T) {
			tr.runGoTest(t, path, []string{"-v", "-timeout", "15m"})
		})
	}
}

func (tr *TestRunner) runAllTests(t *testing.T) {
	t.Log("Running all test suites...")
	
	t.Run("unit_tests", func(t *testing.T) {
		tr.runUnitTests(t)
	})
	
	t.Run("integration_tests", func(t *testing.T) {
		tr.runIntegrationTests(t)
	})
	
	t.Run("stress_tests", func(t *testing.T) {
		tr.runStressTests(t)
	})
}

func (tr *TestRunner) runGoTest(t *testing.T, path string, args []string) {
	ctx, cancel := context.WithTimeout(context.Background(), tr.testTimeout)
	defer cancel()
	
	fullPath := filepath.Join(".", path)
	cmd := exec.CommandContext(ctx, "go", append([]string{"test"}, append(args, fullPath)...)...)
	
	// Set environment variables
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("NERDCTL_TEST_PATH=%s", fullPath),
		fmt.Sprintf("TEST_ENVIRONMENT=%s", tr.testEnvironment),
	)
	
	// Add available binary paths to environment
	for version, binaryPath := range tr.availableBinaries {
		cmd.Env = append(cmd.Env, fmt.Sprintf("NERDCTL_%s_PATH=%s", strings.ToUpper(version), binaryPath))
	}
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("Test failed for %s: %v\nOutput:\n%s", path, err, string(output))
	} else {
		t.Logf("Test passed for %s:\n%s", path, string(output))
	}
}

// Environment and binary detection functions

func detectTestEnvironment() string {
	if os.Getenv("CI") != "" {
		return "ci"
	}
	
	if os.Getenv("NERDCTL_INTEGRATION_TESTS") != "" {
		return "integration"
	}
	
	return "development"
}

func detectAvailableBinaries() map[string]string {
	binaries := make(map[string]string)
	
	// Common installation paths
	searchPaths := []string{
		"/usr/local/bin/nerdctl",
		"/usr/bin/nerdctl", 
		"/opt/nerdctl/bin/nerdctl",
		"/home/runner/app/_output/nerdctl", // Our built binary
		"./bin/nerdctl", // Local binary
	}
	
	for _, path := range searchPaths {
		if isValidNerdctl(path) {
			version := detectBinaryVersion(path)
			if version != "" {
				key := getVersionKey(version)
				binaries[key] = path
			}
		}
	}
	
	return binaries
}

func isValidNerdctl(binaryPath string) bool {
	if _, err := os.Stat(binaryPath); err != nil {
		return false
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, binaryPath, "--version")
	return cmd.Run() == nil
}

func detectBinaryVersion(binaryPath string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, binaryPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	
	version := strings.TrimSpace(string(output))
	// Extract version from "nerdctl version X.Y.Z" format
	parts := strings.Fields(version)
	if len(parts) >= 3 && parts[0] == "nerdctl" && parts[1] == "version" {
		return parts[2]
	}
	
	return ""
}

func getVersionKey(version string) string {
	if version == "" {
		return "unknown"
	}
	
	if strings.HasPrefix(version, "1.") {
		return "v1"
	}
	
	if strings.HasPrefix(version, "2.") {
		return "v2"
	}
	
	return "other"
}

// TestConfig provides configuration for different test scenarios
type TestConfig struct {
	TestType        string
	Timeout         time.Duration
	Parallel        bool
	WithCoverage    bool
	WithRace        bool
	WithBenchmarks  bool
	SkipPatterns    []string
	RequiredBinaries []string
}

// PredefinedConfigs provides common test configurations
var PredefinedConfigs = map[string]*TestConfig{
	"quick": {
		TestType:     "unit",
		Timeout:      2 * time.Minute,
		Parallel:     true,
		WithCoverage: false,
		WithRace:     false,
		SkipPatterns: []string{"*_integration_test.go", "*_stress_test.go"},
	},
	"full": {
		TestType:         "all",
		Timeout:          15 * time.Minute,
		Parallel:         true,
		WithCoverage:     true,
		WithRace:         true,
		WithBenchmarks:   true,
		RequiredBinaries: []string{"v1", "v2"},
	},
	"ci": {
		TestType:     "all",
		Timeout:      10 * time.Minute,
		Parallel:     true,
		WithCoverage: true,
		WithRace:     false, // Disable race detection in CI for speed
		SkipPatterns: []string{"*_stress_test.go"}, // Skip stress tests in CI
	},
	"development": {
		TestType:     "unit",
		Timeout:      5 * time.Minute,
		Parallel:     false,
		WithCoverage: true,
		WithRace:     true,
	},
}

// TestSummary collects test execution results
type TestSummary struct {
	TotalTests    int
	PassedTests   int
	FailedTests   int
	SkippedTests  int
	Duration      time.Duration
	Coverage      float64
	FailedSuites  []string
}

// GenerateTestReport creates a detailed test report
func (tr *TestRunner) GenerateTestReport(summary *TestSummary) string {
	report := fmt.Sprintf(`
Test Execution Report
====================
Environment: %s
Duration: %v
Available Binaries: %v

Results:
- Total Tests: %d
- Passed: %d 
- Failed: %d
- Skipped: %d
- Coverage: %.2f%%

`, tr.testEnvironment, summary.Duration, tr.availableBinaries,
		summary.TotalTests, summary.PassedTests, summary.FailedTests, 
		summary.SkippedTests, summary.Coverage)
	
	if len(summary.FailedSuites) > 0 {
		report += "Failed Suites:\n"
		for _, suite := range summary.FailedSuites {
			report += fmt.Sprintf("- %s\n", suite)
		}
	}
	
	return report
}

// Validation functions for test environment

func (tr *TestRunner) ValidateTestEnvironment(t *testing.T) {
	t.Log("Validating test environment...")
	
	// Check Go version
	if !tr.validateGoVersion() {
		t.Error("Go version validation failed")
	}
	
	// Check required tools
	requiredTools := []string{"go", "git"}
	for _, tool := range requiredTools {
		if !tr.validateTool(tool) {
			t.Errorf("Required tool not found: %s", tool)
		}
	}
	
	// Check test directories
	testDirs := []string{"adapters", "integration", "stress", "mocks"}
	for _, dir := range testDirs {
		if !tr.validateTestDirectory(dir) {
			t.Errorf("Test directory not found or invalid: %s", dir)
		}
	}
	
	t.Log("Test environment validation completed")
}

func (tr *TestRunner) validateGoVersion() bool {
	cmd := exec.Command("go", "version")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	
	version := string(output)
	// Check for minimum Go version (1.21+)
	return strings.Contains(version, "go1.21") || 
		   strings.Contains(version, "go1.22") ||
		   strings.Contains(version, "go1.23")
}

func (tr *TestRunner) validateTool(tool string) bool {
	_, err := exec.LookPath(tool)
	return err == nil
}

func (tr *TestRunner) validateTestDirectory(dir string) bool {
	fullPath := filepath.Join(".", dir)
	info, err := os.Stat(fullPath)
	if err != nil {
		return false
	}
	return info.IsDir()
}