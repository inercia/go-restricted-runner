# Interactive Process Communication with RunWithPipes

The `RunWithPipes()` method enables interactive communication with processes through stdin/stdout/stderr pipes. This is essential for long-running processes, REPLs, interactive shells, and streaming data scenarios.

## Table of Contents

- [When to Use RunWithPipes](#when-to-use-runwithpipes)
- [Basic Usage](#basic-usage)
- [Examples](#examples)
- [Important Notes](#important-notes)
- [API Reference](#api-reference)

## When to Use RunWithPipes

### Use `Run()` when:
- You have a complete command to execute
- You want to get the output after it completes
- The command is short-lived
- You don't need to send input during execution

### Use `RunWithPipes()` when:
- You need to send input to a running process interactively
- You're working with streaming data to/from a long-running process
- You're using REPLs or interactive shells (Python, Node.js, etc.)
- You need to process output while the command is still running
- You want separate access to stdout and stderr

## Basic Usage

### Method Signature

```go
RunWithPipes(ctx context.Context, cmd string, args []string, env []string, 
    params map[string]interface{}) (
    stdin io.WriteCloser,
    stdout io.ReadCloser,
    stderr io.ReadCloser,
    wait func() error,
    err error,
)
```

### Lifecycle

1. Call `RunWithPipes()` to start the process and get pipes
2. Write data to stdin as needed
3. Read from stdout/stderr (can be done concurrently)
4. Close stdin when done writing to signal EOF
5. Read any remaining output from stdout/stderr
6. Call `wait()` to wait for process completion and clean up resources

### Simple Example

```go
package main

import (
    "context"
    "fmt"
    "io"
    "log"

    "github.com/inercia/go-restricted-runner/pkg/common"
    "github.com/inercia/go-restricted-runner/pkg/runner"
)

func main() {
    logger, _ := common.NewLogger("", "", common.LogLevelInfo, false)
    r, err := runner.New(runner.TypeExec, runner.Options{}, logger)
    if err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()

    // Start an interactive process
    stdin, stdout, stderr, wait, err := r.RunWithPipes(
        ctx,
        "cat",      // command
        nil,        // args
        nil,        // env
        nil,        // params
    )
    if err != nil {
        log.Fatal(err)
    }

    // Send input to the process
    fmt.Fprintln(stdin, "Hello from restricted runner!")
    fmt.Fprintln(stdin, "This is interactive communication.")
    
    // Close stdin to signal EOF
    stdin.Close()

    // Read output
    output, _ := io.ReadAll(stdout)
    errOutput, _ := io.ReadAll(stderr)

    // Wait for process to complete
    if err := wait(); err != nil {
        log.Printf("Process error: %v", err)
    }

    fmt.Println("Output:", string(output))
    if len(errOutput) > 0 {
        fmt.Println("Errors:", string(errOutput))
    }
}
```

## Examples

### Example 1: Interactive Python REPL

```go
// Start an interactive Python session
stdin, stdout, stderr, wait, err := r.RunWithPipes(
    ctx,
    "python3",
    []string{"-i"},  // Interactive mode
    nil,
    nil,
)
if err != nil {
    log.Fatal(err)
}

// Send Python commands
fmt.Fprintln(stdin, "x = 10")
fmt.Fprintln(stdin, "y = 20")
fmt.Fprintln(stdin, "print(x + y)")
fmt.Fprintln(stdin, "exit()")
stdin.Close()

// Read output
output, _ := io.ReadAll(stdout)
io.ReadAll(stderr)

wait()
fmt.Println(string(output))
```

### Example 2: Streaming Data Processing

```go
stdin, stdout, stderr, wait, err := r.RunWithPipes(
    ctx,
    "grep",
    []string{"pattern"},
    nil,
    nil,
)
if err != nil {
    log.Fatal(err)
}

// Write data in a goroutine
go func() {
    for _, line := range dataLines {
        fmt.Fprintln(stdin, line)
    }
    stdin.Close()
}()

// Read results as they come
scanner := bufio.NewScanner(stdout)
for scanner.Scan() {
    fmt.Println("Match:", scanner.Text())
}

io.ReadAll(stderr)
wait()
```

### Example 3: Environment Variables

```go
// Use environment variables with RunWithPipes
stdin, stdout, stderr, wait, err := r.RunWithPipes(
    ctx,
    "sh",
    []string{"-c", "echo \"TEST_VAR is: $TEST_VAR\""},
    []string{"TEST_VAR=HelloWorld"},  // Environment variables
    nil,
)
if err != nil {
    log.Fatal(err)
}

stdin.Close()
output, _ := io.ReadAll(stdout)
io.ReadAll(stderr)
wait()

fmt.Print(string(output))
```

### Example 4: Concurrent Read/Write

```go
stdin, stdout, stderr, wait, err := r.RunWithPipes(ctx, "cat", nil, nil, nil)
if err != nil {
    log.Fatal(err)
}

done := make(chan bool)
var readOutput string

// Read in a goroutine
go func() {
    output, _ := io.ReadAll(stdout)
    readOutput = string(output)
    done <- true
}()

// Write some data
testData := "concurrent test\n"
stdin.Write([]byte(testData))
stdin.Close()

// Wait for read to complete
<-done
io.ReadAll(stderr)

err = wait()
if err != nil {
    log.Fatal(err)
}

fmt.Println(readOutput)
```

## Important Notes

### 1. Always Close stdin
You **must** close stdin when done writing to signal EOF to the process:

```go
stdin, stdout, stderr, wait, _ := r.RunWithPipes(...)
fmt.Fprintln(stdin, "data")
stdin.Close()  // ← Required!
```

### 2. Always Call wait()
The `wait()` function **must** be called to clean up resources, even if you don't care about the exit status:

```go
stdin, stdout, stderr, wait, _ := r.RunWithPipes(...)
// ... use pipes ...
wait()  // ← Required for cleanup!
```

### 3. Read from Pipes
Always read from stdout/stderr to prevent deadlocks if the output buffer fills:

```go
// Good - read from pipes
output, _ := io.ReadAll(stdout)
errOutput, _ := io.ReadAll(stderr)

// Bad - may deadlock if output is large
wait()  // Don't call wait() before reading!
```

### 4. Context Cancellation
Cancelling the context will kill the process:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

stdin, stdout, stderr, wait, _ := r.RunWithPipes(ctx, "sleep", []string{"60"}, nil, nil)
// Process will be killed after 5 seconds
```

### 5. All Restrictions Apply
Path restrictions, network isolation, and other security settings configured for the runner still apply with `RunWithPipes()`:

```go
// Create runner with restrictions
r, _ := runner.New(runner.TypeSandboxExec, runner.Options{
    "allow_networking": false,
    "allow_read_folders": []string{"/tmp"},
}, logger)

// Restrictions apply to RunWithPipes too
stdin, stdout, stderr, wait, _ := r.RunWithPipes(ctx, "curl", []string{"https://example.com"}, nil, nil)
// This will fail due to network restriction
```

## API Reference

### Parameters

- **ctx** (`context.Context`): Context for cancellation and timeout. Cancelling kills the process.
- **cmd** (`string`): The command/executable to run (e.g., "python3", "cat", "sh")
- **args** (`[]string`): Command-line arguments for the command
- **env** (`[]string`): Environment variables in KEY=VALUE format
- **params** (`map[string]interface{}`): Template parameters for variable substitution in paths/options

### Return Values

- **stdin** (`io.WriteCloser`): WriteCloser for sending input to the process. Must be closed when done.
- **stdout** (`io.ReadCloser`): ReadCloser for reading standard output from the process.
- **stderr** (`io.ReadCloser`): ReadCloser for reading standard error from the process.
- **wait** (`func() error`): Function to call after reading output. Returns process exit error if any. Must be called to clean up resources.
- **err** (`error`): Error if the process failed to start.

### Runner-Specific Behavior

#### ExecRunner
- Direct execution using `exec.CommandContext`
- No isolation or restrictions
- Simplest and fastest implementation

#### SandboxExec (macOS)
- Executes within macOS sandbox with configured restrictions
- Creates temporary sandbox profile file
- Profile is cleaned up in `wait()` function

#### Firejail (Linux)
- Executes within firejail sandbox with configured restrictions
- Creates temporary firejail profile file
- Profile is cleaned up in `wait()` function

#### Docker
- Creates a long-running background container
- Uses `docker exec -i` for interactive execution
- Container is automatically removed in `wait()` function
- All Docker restrictions (network, mounts, resources) apply

## Common Patterns

### Pattern 1: Simple Echo
```go
stdin, stdout, stderr, wait, _ := r.RunWithPipes(ctx, "cat", nil, nil, nil)
fmt.Fprintln(stdin, "test")
stdin.Close()
output, _ := io.ReadAll(stdout)
io.ReadAll(stderr)
wait()
```

### Pattern 2: REPL Session
```go
stdin, stdout, stderr, wait, _ := r.RunWithPipes(ctx, "python3", []string{"-i"}, nil, nil)
fmt.Fprintln(stdin, "print('Hello')")
fmt.Fprintln(stdin, "exit()")
stdin.Close()
output, _ := io.ReadAll(stdout)
io.ReadAll(stderr)
wait()
```

### Pattern 3: Streaming with Goroutines
```go
stdin, stdout, stderr, wait, _ := r.RunWithPipes(ctx, "command", args, nil, nil)

// Write in background
go func() {
    for _, item := range items {
        fmt.Fprintln(stdin, item)
    }
    stdin.Close()
}()

// Read as data arrives
scanner := bufio.NewScanner(stdout)
for scanner.Scan() {
    processLine(scanner.Text())
}

io.ReadAll(stderr)
wait()
```

## Troubleshooting

### Process Hangs
**Problem**: Process doesn't complete
**Solution**: Make sure you close stdin to signal EOF

### Deadlock
**Problem**: Program hangs indefinitely
**Solution**: Read from stdout/stderr before calling wait()

### Context Cancelled Error
**Problem**: Process exits with context.Canceled
**Solution**: Check your context timeout/cancellation logic

### Restrictions Not Working
**Problem**: Process can access restricted resources
**Solution**: Verify runner type and options are configured correctly

## See Also

- [Exec Runner Documentation](runner-exec.md)
- [SandboxExec Runner Documentation](runner-sandbox-exec.md)
- [Firejail Runner Documentation](runner-firejail.md)
- [Docker Runner Documentation](runner-docker.md)
- [Main README](../README.md)

