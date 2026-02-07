# go-restricted-runner

A Go library for executing commands in isolated/restricted environments.

📚 **[Full Documentation](docs/README.md)** - Detailed guides for each runner type.

## Overview

`go-restricted-runner` provides a unified interface for running shell commands with various levels of isolation and security restrictions. It supports multiple execution backends:

- **exec** - Direct command execution (no isolation)
- **sandbox-exec** - macOS sandbox-exec based isolation
- **firejail** - Linux firejail based isolation
- **docker** - Docker container based isolation

## Installation

```bash
go get github.com/inercia/go-restricted-runner
```

## Usage

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/inercia/go-restricted-runner/pkg/common"
    "github.com/inercia/go-restricted-runner/pkg/runner"
)

func main() {
    // Create a logger
    logger, _ := common.NewLogger("", "", common.LogLevelInfo, false)

    // Create a runner (exec, sandbox-exec, firejail, or docker)
    r, err := runner.New(runner.TypeExec, runner.Options{}, logger)
    if err != nil {
        log.Fatal(err)
    }

    // Execute a command
    ctx := context.Background()
    output, err := r.Run(ctx, "sh", "echo 'Hello, World!'", nil, nil, false)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(output)
}
```

## Runner Types

### Exec Runner

The basic runner that executes commands directly without any isolation.

```go
r, err := runner.New(runner.TypeExec, runner.Options{}, logger)
```

### Sandbox-Exec Runner (macOS)

Uses macOS `sandbox-exec` for process isolation.

```go
r, err := runner.New(runner.TypeSandboxExec, runner.Options{
    "allow_networking": false,
    "allow_read_folders": []string{"/tmp"},
}, logger)
```

### Firejail Runner (Linux)

Uses Linux `firejail` for process isolation.

```go
r, err := runner.New(runner.TypeFirejail, runner.Options{
    "allow_networking": false,
}, logger)
```

### Docker Runner

Executes commands inside Docker containers.

```go
r, err := runner.New(runner.TypeDocker, runner.Options{
    "image": "alpine:latest",
    "allow_networking": false,
}, logger)
```

## Interactive Process Communication

The library supports interactive process communication through the `RunWithPipes()` method, which provides access to stdin/stdout/stderr pipes for long-running or interactive processes.

### When to Use Run() vs RunWithPipes()

- **Use `Run()`** when you have a complete command to execute and want to get the output after it completes
- **Use `RunWithPipes()`** when you need to:
  - Send input to a running process interactively
  - Stream data to/from a long-running process
  - Work with REPLs or interactive shells
  - Process output while the command is still running

### Example: Interactive Process

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

    // Create a runner with restrictions
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

### Example: Python REPL

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

### Important Notes

1. **Always close stdin** when done writing to signal EOF to the process
2. **Always call wait()** to clean up resources, even if you don't care about the exit status
3. **Read from stdout/stderr** before or after calling wait() - both work
4. **Context cancellation** will kill the process
5. **All restrictions apply** - path restrictions, network isolation, etc. still work with RunWithPipes()

## API Reference

### Runner Interface

```go
type Runner interface {
    // Run executes a command and returns the output.
    Run(ctx context.Context, shell string, command string, env []string,
        params map[string]interface{}, tmpfile bool) (string, error)

    // RunWithPipes executes a command with access to stdin/stdout/stderr pipes.
    RunWithPipes(ctx context.Context, cmd string, args []string, env []string,
        params map[string]interface{}) (
        stdin io.WriteCloser,
        stdout io.ReadCloser,
        stderr io.ReadCloser,
        wait func() error,
        err error,
    )

    // CheckImplicitRequirements verifies that the runner's prerequisites are met.
    CheckImplicitRequirements() error
}
```

### Creating Runners

```go
func New(runnerType Type, options Options, logger *common.Logger) (Runner, error)
```

## Development

### Running Tests

```bash
make test
```

### Running Tests with Race Detection

```bash
make test-race
```

### Linting

```bash
make lint
```

### Formatting

```bash
make format
```

## License

MIT License - see [LICENSE](LICENSE) for details.

## Related Projects

- [MCPShell](https://github.com/inercia/MCPShell) - MCP server for shell command execution
- [Don](https://github.com/inercia/don) - AI agent using MCPShell tools

