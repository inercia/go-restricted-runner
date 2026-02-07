# Testing Patterns and Best Practices

**Trigger**: If the user prompt mentions "test", "testing", "unit test", "integration test", or discusses test coverage.

## Test File Organization

- Place tests in `*_test.go` files in the same package
- Group related tests in the same file:
  - `exec_test.go` - Tests for ExecRunner
  - `docker_test.go` - Tests for Docker runner
  - `runner_pipes_test.go` - Tests for RunWithPipes across runners
  - `runner_test.go` - Tests for factory and interface

## Test Naming Conventions

```go
// Format: Test<Type>_<Method>_<Scenario>
func TestExecRunner_Run_BasicCommand(t *testing.T) { }
func TestExecRunner_Run_WithEnvironment(t *testing.T) { }
func TestExecRunner_RunWithPipes_ContextCancellation(t *testing.T) { }
```

## Platform-Specific Test Skipping

```go
func TestSandboxExec_Run(t *testing.T) {
    if runtime.GOOS != "darwin" {
        t.Skip("Skipping sandbox-exec tests on non-macOS platform")
    }
    // ... test code
}

func TestFirejail_Run(t *testing.T) {
    if runtime.GOOS != "linux" {
        t.Skip("Skipping firejail tests on non-Linux platform")
    }
    // ... test code
}
```

## Docker Test Skipping

```go
func TestDocker_Run(t *testing.T) {
    // Check if Docker is available
    if !isDockerAvailable() {
        t.Skip("Docker not installed or not running, skipping test")
    }
    // ... test code
}

func isDockerAvailable() bool {
    cmd := exec.Command("docker", "info")
    return cmd.Run() == nil
}
```

## Table-Driven Tests

Use table-driven tests for multiple scenarios:

```go
func TestNewExecOptions(t *testing.T) {
    tests := []struct {
        name    string
        options Options
        want    ExecOptions
        wantErr bool
    }{
        {
            name: "valid options with shell",
            options: Options{"shell": "/bin/bash"},
            want: ExecOptions{Shell: "/bin/bash"},
            wantErr: false,
        },
        {
            name: "empty options",
            options: Options{},
            want: ExecOptions{},
            wantErr: false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := NewExecOptions(tt.options)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("got %v, want %v", got, tt.want)
            }
        })
    }
}
```

## Logger Setup in Tests

Always create a logger for tests:

```go
func TestSomething(t *testing.T) {
    logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)
    runner, err := NewExec(Options{}, logger)
    // ... test code
}
```

## Testing RunWithPipes

Essential test cases for RunWithPipes:

```go
// 1. Basic functionality
func TestExecRunner_RunWithPipes_BasicEcho(t *testing.T) {
    // Test stdin -> stdout echo
}

// 2. Multiple writes
func TestExecRunner_RunWithPipes_MultipleWrites(t *testing.T) {
    // Test multiple writes to stdin
}

// 3. Stderr capture
func TestExecRunner_RunWithPipes_StderrCapture(t *testing.T) {
    // Test stderr is captured separately
}

// 4. Context cancellation
func TestExecRunner_RunWithPipes_ContextCancellation(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    // Start long-running command
    // Cancel context
    // Verify process is killed
}

// 5. Error handling
func TestExecRunner_RunWithPipes_CommandNotFound(t *testing.T) {
    // Test non-existent command returns error
}

// 6. Environment variables
func TestExecRunner_RunWithPipes_WithEnvironment(t *testing.T) {
    // Test env vars are passed correctly
}

// 7. Early exit
func TestExecRunner_RunWithPipes_CommandExitsEarly(t *testing.T) {
    // Test when command exits before stdin closed
}

// 8. Concurrent I/O
func TestExecRunner_RunWithPipes_ConcurrentReadWrite(t *testing.T) {
    // Test reading and writing concurrently
}
```

## Testing Cleanup

Always clean up resources in tests:

```go
func TestSomething(t *testing.T) {
    stdin, stdout, stderr, wait, err := runner.RunWithPipes(...)
    require.NoError(t, err)
    
    // Always close stdin
    defer stdin.Close()
    
    // Always read from pipes (prevents deadlock)
    defer io.ReadAll(stdout)
    defer io.ReadAll(stderr)
    
    // Always call wait
    defer wait()
    
    // ... test code
}
```

## Assertion Libraries

The codebase uses standard testing library. Common patterns:

```go
// Error checking
if err != nil {
    t.Fatalf("Unexpected error: %v", err)
}

// Value comparison
if got != want {
    t.Errorf("got %v, want %v", got, want)
}

// String contains
if !strings.Contains(output, expected) {
    t.Errorf("output %q does not contain %q", output, expected)
}
```

## Running Tests

```bash
# All tests
make test

# With race detection
make test-race

# With coverage
make test-coverage

# Specific package
go test -v ./pkg/runner/...

# Specific test
go test -v ./pkg/runner/... -run TestExecRunner_RunWithPipes

# Verbose output
go test -v ./...
```

## Test Coverage Goals

- Aim for >80% coverage for new code
- 100% coverage for critical paths (error handling, cleanup)
- Test both success and failure cases
- Include edge cases (empty input, nil values, etc.)

## Common Test Pitfalls

### 1. Forgetting to Close stdin
```go
// BAD - process may hang
stdin, stdout, stderr, wait, _ := runner.RunWithPipes(...)
io.ReadAll(stdout)
wait()

// GOOD
stdin, stdout, stderr, wait, _ := runner.RunWithPipes(...)
stdin.Close()  // Signal EOF
io.ReadAll(stdout)
wait()
```

### 2. Not Reading from Pipes
```go
// BAD - may deadlock if output buffer fills
stdin, stdout, stderr, wait, _ := runner.RunWithPipes(...)
stdin.Close()
wait()  // May hang!

// GOOD
stdin, stdout, stderr, wait, _ := runner.RunWithPipes(...)
stdin.Close()
io.ReadAll(stdout)  // Drain pipes
io.ReadAll(stderr)
wait()
```

### 3. Platform-Specific Commands
```go
// BAD - assumes Unix
cmd := "ls"

// GOOD - check platform
var cmd string
if runtime.GOOS == "windows" {
    cmd = "dir"
} else {
    cmd = "ls"
}
```

