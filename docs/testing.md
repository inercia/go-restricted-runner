# Testing Guide

This document describes the testing strategy and infrastructure for go-restricted-runner.

## Test Organization

### Unit Tests

Unit tests verify individual components and functions in isolation:

- **Options parsing**: Testing configuration option parsing for each runner type
- **Rule building**: Testing the construction of restriction rules
- **Error handling**: Testing error conditions and edge cases
- **Helper functions**: Testing utility functions

**Location**: `pkg/runner/*_test.go`

**Example**:
```go
func TestNewLandrunOptions(t *testing.T) {
    // Tests option parsing for Landrun runner
}
```

### Integration Tests

Integration tests verify actual sandboxing behavior and runner functionality:

- **Filesystem restrictions**: Testing read/write/execute permissions
- **Network restrictions**: Testing TCP bind/connect controls
- **Process isolation**: Testing command execution in restricted environments
- **Template variables**: Testing dynamic path configuration
- **RunWithPipes**: Testing interactive process communication

**Location**: `pkg/runner/*_test.go` (prefixed with `TestXxx_Integration_`)

**Example**:
```go
func TestLandrun_Integration_FilesystemDenial(t *testing.T) {
    // Tests that Landlock actually denies access to restricted paths
}
```

### Benchmark Tests

Benchmark tests measure performance characteristics:

- **Execution overhead**: Measuring the cost of sandboxing
- **Comparison**: Comparing different runner types
- **Scalability**: Testing performance under load

**Location**: `pkg/runner/*_test.go` (prefixed with `Benchmark`)

**Example**:
```go
func BenchmarkLandrun_Run_WithRestrictions(b *testing.B) {
    // Measures performance of Landrun with restrictions
}
```

## Platform-Specific Testing

### Linux-Only Runners

Landrun and Firejail are Linux-only. Tests for these runners:

1. **Check platform**: Skip on non-Linux systems
2. **Check availability**: Skip if Landlock/firejail not available
3. **Run tests**: Execute comprehensive test suite

**Example**:
```go
func TestLandrun_Run(t *testing.T) {
    if !isLandlockAvailable() {
        t.Skip("Landlock not available on this system")
    }
    // Test code...
}
```

### macOS-Only Runners

Sandbox-exec is macOS-only:

```go
func TestSandboxExec_Run(t *testing.T) {
    if runtime.GOOS != "darwin" {
        t.Skip("Skipping sandbox-exec tests on non-macOS platform")
    }
    // Test code...
}
```

### Cross-Platform Runners

Exec and Docker runners work on all platforms but may have platform-specific behavior.

## Running Tests

### All Tests

```bash
go test ./...
```

### Specific Package

```bash
go test ./pkg/runner/
```

### Specific Test

```bash
go test -v -run TestLandrun_Run_BasicCommand ./pkg/runner/
```

### With Coverage

```bash
go test -coverprofile=coverage.txt -covermode=atomic ./...
```

### With Race Detection

```bash
go test -race ./...
```

### Benchmarks

```bash
go test -bench=. -benchtime=1s ./pkg/runner/
```

### Verbose Output

```bash
go test -v ./pkg/runner/
```

## GitHub Actions CI

The CI pipeline runs tests on multiple platforms and configurations:

### Standard Test Job

Runs on: `ubuntu-latest`, `macos-latest`, `windows-latest`

- Downloads dependencies
- Runs all tests with race detection
- Generates coverage report
- Uploads coverage to Codecov (Ubuntu only)

### Linux-Specific Runners Job

Runs on: `ubuntu-latest`

- Checks Landlock availability
- Installs firejail
- Runs Landrun tests
- Runs Firejail tests
- Runs integration tests
- Runs benchmarks

**Configuration**: `.github/workflows/ci.yml`

## Test Coverage Goals

- **Overall**: >80% code coverage
- **Critical paths**: 100% coverage (error handling, cleanup)
- **Platform-specific**: Comprehensive coverage on target platforms

## Writing New Tests

### Test Naming Convention

```go
// Unit test
func TestRunnerName_MethodName(t *testing.T)

// Integration test
func TestRunnerName_Integration_Feature(t *testing.T)

// Benchmark
func BenchmarkRunnerName_Operation(b *testing.B)
```

### Test Structure

```go
func TestLandrun_Feature(t *testing.T) {
    // 1. Check prerequisites
    if !isLandlockAvailable() {
        t.Skip("Landlock not available")
    }

    // 2. Setup
    logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)
    runner, err := NewLandrun(options, logger)
    if err != nil {
        t.Fatalf("Setup failed: %v", err)
    }

    // 3. Execute
    result, err := runner.Run(ctx, shell, command, env, params, tmpfile)

    // 4. Verify
    if err != nil {
        t.Errorf("Unexpected error: %v", err)
    }
    if result != expected {
        t.Errorf("Expected %v, got %v", expected, result)
    }

    // 5. Cleanup (if needed)
    defer cleanup()
}
```

### Table-Driven Tests

For testing multiple scenarios:

```go
func TestLandrun_Scenarios(t *testing.T) {
    tests := []struct {
        name        string
        options     Options
        command     string
        shouldError bool
    }{
        {
            name: "scenario 1",
            options: Options{...},
            command: "...",
            shouldError: false,
        },
        // More test cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test code using tt.options, tt.command, etc.
        })
    }
}
```

## Landrun-Specific Tests

### Test Categories

1. **Unit Tests** (11 tests)
   - CheckImplicitRequirements
   - Run_BasicCommand
   - Run_WithFilesystemRestrictions
   - Run_WithWriteRestrictions
   - Run_WithTemplateVariables
   - Run_WithEnvironmentVariables
   - Run_ContextCancellation
   - RunWithPipes_BasicEcho
   - RunWithPipes_MultipleWrites
   - RunWithPipes_ContextCancellation
   - BestEffortMode

2. **Integration Tests** (6 tests)
   - Integration_FilesystemDenial
   - Integration_WriteRestriction
   - Integration_ExecuteRestriction
   - Integration_MultipleRestrictions
   - Integration_RunWithPipes_Restrictions
   - Integration_ErrorHandling

3. **Benchmark Tests** (2 tests)
   - Benchmark_Run_Unrestricted
   - Benchmark_Run_WithRestrictions

### Running Landrun Tests on Linux

```bash
# All Landrun tests
go test -v -run TestLandrun ./pkg/runner/

# Integration tests only
go test -v -run TestLandrun_Integration ./pkg/runner/

# Benchmarks
go test -bench=BenchmarkLandrun ./pkg/runner/
```

## Troubleshooting Tests

### Tests Skipping on Linux

If Landrun tests skip on Linux, check:

1. **Kernel version**: `uname -r` (need 5.13+)
2. **Landlock config**: `grep CONFIG_SECURITY_LANDLOCK /boot/config-$(uname -r)`
3. **LSM list**: `cat /sys/kernel/security/lsm`

### Firejail Tests Failing

If Firejail tests fail:

1. **Install firejail**: `sudo apt-get install firejail`
2. **Check installation**: `which firejail`
3. **Test manually**: `firejail echo test`

### Docker Tests Skipping

If Docker tests skip:

1. **Install Docker**: Follow Docker installation guide
2. **Start daemon**: `sudo systemctl start docker`
3. **Check status**: `docker info`

## Best Practices

1. **Always skip gracefully**: Use `t.Skip()` for unavailable features
2. **Clean up resources**: Use `defer` for cleanup
3. **Test error paths**: Don't just test happy paths
4. **Use meaningful names**: Test names should describe what they test
5. **Keep tests focused**: One test should test one thing
6. **Avoid test interdependence**: Tests should be independent
7. **Use helpers**: Extract common setup into helper functions

