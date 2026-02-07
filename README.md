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

For interactive processes, REPLs, or streaming data scenarios, use the `RunWithPipes()` method:

```go
stdin, stdout, stderr, wait, err := r.RunWithPipes(ctx, "python3", []string{"-i"}, nil, nil)
if err != nil {
    log.Fatal(err)
}

fmt.Fprintln(stdin, "print('Hello')")
stdin.Close()

output, _ := io.ReadAll(stdout)
wait()
```

**📖 For detailed documentation, examples, and best practices, see [Interactive Process Communication Guide](docs/run-with-pipes.md)**

## API Reference

### Runner Interface

```go
type Runner interface {
    // Run executes a command and returns the output
    Run(ctx context.Context, shell string, command string, env []string,
        params map[string]interface{}, tmpfile bool) (string, error)

    // RunWithPipes executes a command with interactive stdin/stdout/stderr
    RunWithPipes(ctx context.Context, cmd string, args []string, env []string,
        params map[string]interface{}) (stdin, stdout, stderr, wait, error)

    // CheckImplicitRequirements verifies prerequisites are met
    CheckImplicitRequirements() error
}
```

### Factory Function

```go
func New(runnerType Type, options Options, logger *common.Logger) (Runner, error)
```

**📖 For detailed API documentation, see the [docs](docs/) directory**

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

