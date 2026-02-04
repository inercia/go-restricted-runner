# go-restricted-runner Documentation

This documentation provides detailed information about the different runner types available in `go-restricted-runner`.

## Table of Contents

### Runner Types

| Runner | Platform | Isolation Level | Description |
|--------|----------|-----------------|-------------|
| [Exec Runner](runner-exec.md) | All | None | Direct command execution without isolation |
| [Sandbox-Exec Runner](runner-sandbox-exec.md) | macOS | Medium | macOS sandbox-exec based isolation |
| [Firejail Runner](runner-firejail.md) | Linux | Medium | Linux firejail based isolation |
| [Docker Runner](runner-docker.md) | All* | High | Docker container based isolation |

*Requires Docker to be installed and running.

## Quick Start

```go
import (
    "context"
    "github.com/inercia/go-restricted-runner/pkg/common"
    "github.com/inercia/go-restricted-runner/pkg/runner"
)

// Create a logger
logger, _ := common.NewLogger("", "", common.LogLevelInfo, false)

// Create a runner
r, err := runner.New(runner.TypeExec, runner.Options{}, logger)
if err != nil {
    // Handle error
}

// Execute a command
ctx := context.Background()
output, err := r.Run(ctx, "sh", "echo 'Hello!'", nil, nil, false)
```

## Choosing a Runner

### When to use each runner:

- **Exec**: Development, testing, or when you trust the commands being executed
- **Sandbox-Exec**: macOS environments requiring process isolation
- **Firejail**: Linux environments requiring process isolation
- **Docker**: Maximum isolation or cross-platform consistent environments

### Comparison Matrix

| Feature | Exec | Sandbox-Exec | Firejail | Docker |
|---------|------|--------------|----------|--------|
| Network control | ❌ | ✅ | ✅ | ✅ |
| Filesystem isolation | ❌ | ✅ | ✅ | ✅ |
| Custom profiles | ❌ | ✅ | ✅ | ✅ |
| Memory limits | ❌ | ❌ | ❌ | ✅ |
| Cross-platform | ✅ | ❌ | ❌ | ✅* |
| No dependencies | ✅ | ✅** | ❌ | ❌ |
| Performance overhead | None | Low | Low | Medium |

\* Requires Docker installation
\** Built into macOS

## Common Interface

All runners implement the `Runner` interface:

```go
type Runner interface {
    // Run executes a command and returns the output.
    Run(ctx context.Context, shell string, command string, env []string, 
        params map[string]interface{}, tmpfile bool) (string, error)

    // CheckImplicitRequirements verifies that the runner's prerequisites are met.
    CheckImplicitRequirements() error
}
```

### Run Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `ctx` | `context.Context` | Context for cancellation and timeout |
| `shell` | `string` | Shell to use (empty for default) |
| `command` | `string` | The command to execute |
| `env` | `[]string` | Environment variables in `KEY=VALUE` format |
| `params` | `map[string]interface{}` | Template parameters for variable substitution |
| `tmpfile` | `bool` | Whether to use a temporary file for the command |

## Creating Runners

Use the factory function to create runners:

```go
func New(runnerType Type, options Options, logger *common.Logger) (Runner, error)
```

Runner types:
- `runner.TypeExec` - Direct execution
- `runner.TypeSandboxExec` - macOS sandbox-exec
- `runner.TypeFirejail` - Linux firejail
- `runner.TypeDocker` - Docker container

## Error Handling

Each runner performs implicit requirements checks when created:

```go
r, err := runner.New(runner.TypeFirejail, runner.Options{}, logger)
if err != nil {
    // Could be:
    // - "firejail runner requires Linux" (wrong OS)
    // - "firejail executable not found in PATH" (missing dependency)
}
```

