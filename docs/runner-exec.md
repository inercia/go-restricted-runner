# Exec Runner

The Exec runner provides direct command execution without any isolation or sandboxing. It's the simplest runner type and serves as the baseline for other runners.

## How It Works

The Exec runner executes commands directly using the system shell:

1. **Command Preparation**: The command is either executed directly (for simple executables) or wrapped in a shell script
2. **Shell Selection**: Uses the configured shell or detects the appropriate shell based on the OS
3. **Execution**: Creates an `exec.Cmd` and runs the command
4. **Output Capture**: Captures stdout and stderr, returning the combined output

### Execution Modes

The runner supports multiple execution modes:

- **Direct Execution**: For single executable commands (e.g., `ls`, `echo`)
- **Shell Execution**: For complex commands with pipes, redirects, etc.
- **Temporary Script**: When `tmpfile=true`, writes the command to a temporary script file

### Platform Support

| Platform | Supported | Default Shell |
|----------|-----------|---------------|
| Linux | ✅ | `/bin/sh` |
| macOS | ✅ | `/bin/sh` |
| Windows | ✅ | `cmd.exe` or `powershell` |

## Pros and Cons

### Pros

- ✅ **No dependencies**: Works on any system with a shell
- ✅ **Maximum performance**: No overhead from isolation
- ✅ **Full system access**: Commands can access all system resources
- ✅ **Cross-platform**: Works on Linux, macOS, and Windows
- ✅ **Simple debugging**: Commands run exactly as they would in a terminal

### Cons

- ❌ **No isolation**: Commands have full access to the system
- ❌ **Security risk**: Untrusted commands can damage the system
- ❌ **No resource limits**: Cannot limit CPU, memory, or network usage
- ❌ **No filesystem restrictions**: Can read/write anywhere the user can

## Limitations

- No network isolation
- No filesystem sandboxing
- No resource limiting (CPU, memory)
- No security boundaries
- Commands run with the full privileges of the parent process

## API Usage

### Basic Usage

```go
import (
    "context"
    "github.com/inercia/go-restricted-runner/pkg/common"
    "github.com/inercia/go-restricted-runner/pkg/runner"
)

// Create logger
logger, _ := common.NewLogger("", "", common.LogLevelInfo, false)

// Create an Exec runner
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
fmt.Println(output) // Output: Hello, World!
```

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `shell` | `string` | System default | Shell to use for command execution |

```go
// Create runner with custom shell
r, err := runner.New(runner.TypeExec, runner.Options{
    "shell": "/bin/bash",
}, logger)
```

### With Environment Variables

```go
env := []string{
    "MY_VAR=value1",
    "ANOTHER_VAR=value2",
}

output, err := r.Run(ctx, "sh", "echo $MY_VAR", env, nil, false)
```

### With Template Parameters

```go
params := map[string]interface{}{
    "filename": "test.txt",
}

// Use params for template variable substitution in other runners
output, err := r.Run(ctx, "sh", "cat {{.filename}}", nil, params, false)
```

### Using Temporary Script File

```go
// Write command to temp file before execution
output, err := r.Run(ctx, "sh", `
    echo "Line 1"
    echo "Line 2"
    echo "Line 3"
`, nil, nil, true)  // tmpfile=true
```

### With Context Timeout

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

output, err := r.Run(ctx, "sh", "sleep 10 && echo done", nil, nil, false)
if err != nil {
    // Will timeout after 5 seconds
    fmt.Println("Command timed out:", err)
}
```

## When to Use

Use the Exec runner when:

- You trust the commands being executed
- You need maximum performance
- You're in a development or testing environment
- You need full system access for legitimate purposes
- The commands are well-understood and controlled

## Security Considerations

⚠️ **Warning**: The Exec runner provides no security boundaries. Only use it with trusted commands.

- Never use with user-provided input without thorough validation
- Consider using sandboxed runners (Sandbox-Exec, Firejail, Docker) for untrusted commands
- Log all executed commands for auditing purposes

## See Also

- [Sandbox-Exec Runner](runner-sandbox-exec.md) - macOS isolation
- [Firejail Runner](runner-firejail.md) - Linux isolation
- [Docker Runner](runner-docker.md) - Container-based isolation

