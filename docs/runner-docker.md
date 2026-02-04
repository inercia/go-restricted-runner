# Docker Runner

The Docker runner provides the highest level of isolation by executing commands inside Docker containers. It offers complete process, filesystem, and network isolation with extensive configuration options.

## How It Works

1. **Script Generation**: For complex commands, a temporary script file is created
2. **Docker Command Building**: Constructs a `docker run` command with all configured options
3. **Container Execution**: Runs the command inside a disposable container (`--rm`)
4. **Output Capture**: Captures stdout/stderr and returns the output
5. **Cleanup**: Container is automatically removed after execution

### Execution Flow

```
Command → Create Script → Mount in Container → Execute → Capture Output → Cleanup
```

For single executable commands, the runner optimizes by skipping the script file and executing directly.

## Pros and Cons

### Pros

- ✅ **Maximum isolation**: Complete process, filesystem, and network isolation
- ✅ **Cross-platform**: Works on Linux, macOS, and Windows (where Docker runs)
- ✅ **Resource limits**: Control memory, CPU, and other resources
- ✅ **Reproducible environments**: Consistent execution across different hosts
- ✅ **Custom images**: Use any Docker image for specific requirements
- ✅ **Network control**: Fine-grained network configuration
- ✅ **Capability control**: Add or drop Linux capabilities

### Cons

- ❌ **Docker required**: Requires Docker installation and running daemon
- ❌ **Higher overhead**: Container startup adds latency (~100-500ms)
- ❌ **Resource consumption**: Docker daemon uses system resources
- ❌ **Image management**: Need to manage/pull Docker images
- ❌ **Complexity**: Many configuration options to understand
- ❌ **Storage**: Docker images consume disk space

## Limitations

- Requires Docker to be installed and the daemon running
- Container startup adds latency to command execution
- Docker images must be pulled before first use
- Some host features may not be available inside containers
- Requires sufficient disk space for images and layers

## API Usage

### Basic Usage

```go
import (
    "context"
    "github.com/inercia/go-restricted-runner/pkg/common"
    "github.com/inercia/go-restricted-runner/pkg/runner"
)

logger, _ := common.NewLogger("", "", common.LogLevelInfo, false)

// Create a Docker runner (image is required)
r, err := runner.New(runner.TypeDocker, runner.Options{
    "image": "alpine:latest",
}, logger)
if err != nil {
    log.Fatal(err)
}

ctx := context.Background()
output, err := r.Run(ctx, "sh", "echo 'Hello from Docker!'", nil, nil, false)
```

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `image` | `string` | **required** | Docker image to use |
| `allow_networking` | `bool` | `true` | Allow network access |
| `network` | `string` | `""` | Specific network (e.g., "host", "bridge") |
| `docker_run_opts` | `string` | `""` | Additional docker run options |
| `mounts` | `[]string` | `[]` | Mount points ("host:container") |
| `user` | `string` | `""` | User to run as inside container |
| `workdir` | `string` | `""` | Working directory inside container |
| `prepare_command` | `string` | `""` | Command to run before main command |
| `memory` | `string` | `""` | Memory limit (e.g., "512m", "1g") |
| `memory_reservation` | `string` | `""` | Memory soft limit |
| `memory_swap` | `string` | `""` | Swap limit ("-1" for unlimited) |
| `memory_swappiness` | `int` | `-1` | Swappiness (0-100, -1 for default) |
| `cap_add` | `[]string` | `[]` | Linux capabilities to add |
| `cap_drop` | `[]string` | `[]` | Linux capabilities to drop |
| `dns` | `[]string` | `[]` | Custom DNS servers |
| `dns_search` | `[]string` | `[]` | Custom DNS search domains |
| `platform` | `string` | `""` | Platform (e.g., "linux/amd64") |

### Disable Network Access

```go
r, err := runner.New(runner.TypeDocker, runner.Options{
    "image":            "alpine:latest",
    "allow_networking": false,  // Adds --network none
}, logger)
```

### With Volume Mounts

```go
r, err := runner.New(runner.TypeDocker, runner.Options{
    "image":  "alpine:latest",
    "mounts": []string{
        "/host/data:/container/data:ro",  // Read-only mount
        "/host/output:/container/output", // Read-write mount
    },
}, logger)
```

### With Memory Limits

```go
r, err := runner.New(runner.TypeDocker, runner.Options{
    "image":              "alpine:latest",
    "memory":             "512m",
    "memory_reservation": "256m",
    "memory_swap":        "1g",
}, logger)
```

### With Capabilities

```go
r, err := runner.New(runner.TypeDocker, runner.Options{
    "image":    "alpine:latest",
    "cap_add":  []string{"NET_ADMIN"},
    "cap_drop": []string{"MKNOD", "AUDIT_WRITE"},
}, logger)
```

### With Custom User and Working Directory

```go
r, err := runner.New(runner.TypeDocker, runner.Options{
    "image":   "node:18-alpine",
    "user":    "1000:1000",
    "workdir": "/app",
    "mounts":  []string{"/host/project:/app"},
}, logger)
```

### With Prepare Command

```go
r, err := runner.New(runner.TypeDocker, runner.Options{
    "image":           "python:3.11-slim",
    "prepare_command": "pip install requests",
}, logger)

// The prepare_command runs before the main command in the script
output, err := r.Run(ctx, "sh", "python -c 'import requests; print(requests.__version__)'", nil, nil, false)
```

### Full Example

```go
r, err := runner.New(runner.TypeDocker, runner.Options{
    "image":            "ubuntu:22.04",
    "allow_networking": true,
    "network":          "bridge",
    "user":             "1000:1000",
    "workdir":          "/workspace",
    "memory":           "1g",
    "mounts":           []string{"/home/user/project:/workspace"},
    "cap_drop":         []string{"ALL"},
    "dns":              []string{"8.8.8.8", "8.8.4.4"},
    "platform":         "linux/amd64",
}, logger)
```

## Implicit Requirements

The Docker runner checks these requirements on creation:

1. **Executable**: `docker` must be available in PATH
2. **Daemon Running**: Docker daemon must be running

```go
r, err := runner.New(runner.TypeDocker, runner.Options{
    "image": "alpine:latest",
}, logger)
if err != nil {
    // Possible errors:
    // - "docker runner requires 'image' option"
    // - "docker executable not found in PATH"
    // - "docker daemon is not running"
}
```

## Generated Docker Command

The runner generates commands like:

```bash
docker run --rm \
    --network none \
    --user 1000:1000 \
    --workdir /app \
    --memory 512m \
    -v /host/script.sh:/tmp/script.sh \
    alpine:latest \
    sh /tmp/script.sh
```

## Security Considerations

- Use specific image tags, not `latest`, for reproducibility
- Drop unnecessary capabilities with `cap_drop`
- Use `--network none` when network access isn't needed
- Run as non-root user when possible
- Set memory limits to prevent resource exhaustion
- Use read-only mounts (`:ro`) when write access isn't needed
- Consider using `--read-only` via `docker_run_opts` for additional security

## See Also

- [Exec Runner](runner-exec.md) - No isolation
- [Sandbox-Exec Runner](runner-sandbox-exec.md) - macOS isolation
- [Firejail Runner](runner-firejail.md) - Linux isolation
- [Docker Documentation](https://docs.docker.com/) - Official docs
- [Docker Security](https://docs.docker.com/engine/security/) - Security best practices

