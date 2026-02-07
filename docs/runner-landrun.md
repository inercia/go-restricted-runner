# Landrun Runner

The Landrun runner provides kernel-native sandboxing for Linux using the Landlock LSM (Linux Security Module). It offers fine-grained filesystem and network access control without requiring root privileges or external binaries.

## Overview

Landlock is a Linux kernel security feature (available since kernel 5.13) that allows unprivileged processes to sandbox themselves. The Landrun runner leverages this capability through the `go-landlock` library to provide:

- **Kernel-native security** - Uses Linux Landlock LSM built into the kernel
- **No root required** - Unprivileged sandboxing
- **Lightweight** - No container overhead, minimal performance impact
- **Fine-grained control** - Precise filesystem and network restrictions
- **Best-effort mode** - Graceful degradation on older kernels

## Important Limitations

**⚠️ Landlock restrictions are irreversible and process-wide**

Landlock restrictions are applied to the current process and cannot be removed or relaxed once applied. This has important implications:

1. **Multiple commands in the same process**: If you call `Run()` or `RunWithPipes()` multiple times with different restrictions in the same process, the restrictions will accumulate. Each call adds more restrictions that cannot be undone.

2. **Process-wide effect**: Once Landlock is applied, it affects the current process and ALL its children, including any subsequent operations in your program.

3. **Best practices**:
   - Use one Landrun instance per process for best results
   - For multiple commands with different restrictions, consider:
     - Running each command in a separate process
     - Using the Docker runner which provides process-level isolation
     - Using Firejail which spawns separate sandboxed processes

4. **Testing**: When writing tests, be aware that Landlock restrictions applied in one test may affect subsequent tests in the same test process. Use `t.Parallel()` or run tests in separate processes if needed.

This is a fundamental limitation of how Landlock works in the Linux kernel, not a limitation of this library.

## Requirements

- **Operating System**: Linux only
- **Kernel Version**: 
  - Linux 5.13+ for basic filesystem sandboxing
  - Linux 6.7+ for network restrictions (TCP bind/connect)
- **Landlock Support**: Kernel must be compiled with `CONFIG_SECURITY_LANDLOCK=y`

To check if Landlock is available on your system:

```bash
# Check kernel config
grep CONFIG_SECURITY_LANDLOCK /boot/config-$(uname -r)
# Should show: CONFIG_SECURITY_LANDLOCK=y

# Check if Landlock is in the LSM list
cat /sys/kernel/security/lsm
# Should include "landlock" in the list
```

## Configuration Options

### Filesystem Access

- `allow_read_folders` ([]string): Directories with read-only access
- `allow_read_exec_folders` ([]string): Directories with read and execute access
- `allow_write_folders` ([]string): Directories with read-write access
- `allow_write_exec_folders` ([]string): Directories with read-write-execute access

### Network Access (Kernel 6.7+)

- `allow_bind_tcp` ([]uint16): TCP ports allowed for binding
- `allow_connect_tcp` ([]uint16): TCP ports allowed for connecting
- `allow_networking` (bool): Allow unrestricted network access (default: false)

### Other Options

- `unrestricted_filesystem` (bool): Allow unrestricted filesystem access (default: false)
- `best_effort` (bool): Gracefully degrade on older kernels (default: false)

## Usage Examples

### Basic Usage with Filesystem Restrictions

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
    logger, _ := common.NewLogger("", "", common.LogLevelInfo, false)

    // Create Landrun runner with filesystem restrictions
    r, err := runner.New(runner.TypeLandrun, runner.Options{
        "allow_read_folders": []string{"/usr", "/lib", "/lib64", "/etc"},
        "allow_read_exec_folders": []string{"/usr/bin", "/bin"},
        "allow_write_folders": []string{"/tmp"},
        "best_effort": true,
    }, logger)
    if err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()
    output, err := r.Run(ctx, "sh", "ls /usr/bin | head -5", nil, nil, false)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(output)
}
```

### Network Restrictions (Kernel 6.7+)

```go
// Create runner with network restrictions
r, err := runner.New(runner.TypeLandrun, runner.Options{
    "allow_read_exec_folders": []string{"/usr", "/bin", "/lib", "/lib64"},
    "allow_bind_tcp": []uint16{8080},           // Allow binding to port 8080
    "allow_connect_tcp": []uint16{80, 443},     // Allow connecting to ports 80 and 443
    "best_effort": true,
}, logger)
```

### Template Variables in Paths

```go
// Use template variables for dynamic paths
r, err := runner.New(runner.TypeLandrun, runner.Options{
    "allow_read_folders": []string{"{{.workdir}}", "/usr", "/lib"},
    "allow_write_folders": []string{"{{.tmpdir}}"},
    "allow_read_exec_folders": []string{"/usr/bin", "/bin"},
    "best_effort": true,
}, logger)

params := map[string]interface{}{
    "workdir": "/home/user/project",
    "tmpdir":  "/tmp/myapp",
}

output, err := r.Run(ctx, "sh", "ls {{.workdir}}", nil, params, false)
```

### Interactive Process with RunWithPipes

```go
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

// Write Python commands
fmt.Fprintln(stdin, "print('Hello from Python')")
fmt.Fprintln(stdin, "exit()")
stdin.Close()

// Read output
output, _ := io.ReadAll(stdout)
io.ReadAll(stderr)

// Wait for completion
if err := wait(); err != nil {
    log.Printf("Command failed: %v", err)
}

fmt.Println(string(output))
```

### Best Effort Mode

Best effort mode allows the runner to gracefully degrade on older kernels:

```go
r, err := runner.New(runner.TypeLandrun, runner.Options{
    "allow_read_exec_folders": []string{"/usr", "/bin"},
    "allow_bind_tcp": []uint16{8080},  // Will be ignored on kernels < 6.7
    "best_effort": true,                // Enable graceful degradation
}, logger)
```

With best effort mode:
- On Linux 6.7+: Full filesystem and network restrictions
- On Linux 6.2-6.6: Filesystem restrictions only (no network)
- On Linux 5.13-6.1: Basic filesystem restrictions
- On older Linux: No restrictions (sandbox disabled)

## How It Works

1. **Restriction Application**: When `Run()` or `RunWithPipes()` is called, Landlock restrictions are applied to the current process
2. **Process Inheritance**: All child processes inherit the same restrictions
3. **Kernel Enforcement**: The Linux kernel enforces the restrictions at the system call level
4. **No Relaxation**: Once applied, Landlock restrictions cannot be relaxed (only made stricter)

## Landlock Access Rights

Landlock provides fine-grained control over filesystem operations:

**File-specific rights:**
- Execute files
- Read files
- Write to files
- Truncate files (Landlock ABI v3+)

**Directory-specific rights:**
- Read directory contents
- Remove directories/files
- Create files, directories, devices, sockets, etc.
- Rename/link files between directories (Landlock ABI v2+)

**Network-specific rights (Landlock ABI v4+):**
- Bind to specific TCP ports
- Connect to specific TCP ports

## Comparison with Other Runners

| Feature | Landrun | Firejail | Sandbox-exec | Docker |
|---------|---------|----------|--------------|--------|
| Platform | Linux | Linux | macOS | All |
| Kernel Requirement | 5.13+ | Any | Any | Any |
| External Binary | No | Yes | Yes | Yes |
| Root Required | No | No | No | Sometimes |
| Isolation Level | Medium-High | Medium | Medium | High |
| Overhead | Minimal | Low | Low | High |
| Network Control | Yes (6.7+) | Yes | Yes | Yes |

## Limitations

1. **Linux-only**: Only works on Linux systems
2. **Kernel version dependency**: Full features require newer kernels
3. **Process-wide restrictions**: Landlock applies to the entire process tree
4. **One-time application**: Restrictions can't be relaxed once applied
5. **Some operations not restrictable**: See [Kernel Documentation](https://docs.kernel.org/userspace-api/landlock.html) for details

## Troubleshooting

### Landlock Not Available

If you get an error about Landlock not being available:

1. Check your kernel version: `uname -r` (need 5.13+)
2. Verify Landlock is enabled: `grep CONFIG_SECURITY_LANDLOCK /boot/config-$(uname -r)`
3. Check LSM list: `cat /sys/kernel/security/lsm`

### Permission Denied Errors

If commands fail with permission denied:

1. Enable debug logging to see which paths are being restricted
2. Add necessary paths to `allow_read_folders` or `allow_read_exec_folders`
3. Remember to include system directories like `/usr`, `/lib`, `/lib64`
4. For executables, use `allow_read_exec_folders` not just `allow_read_folders`

### Network Restrictions Not Working

Network restrictions require kernel 6.7+:

1. Check kernel version: `uname -r`
2. Use `best_effort: true` to gracefully handle older kernels
3. Network rules will be silently ignored on kernels < 6.7 in best effort mode

## References

- [Landlock Documentation](https://landlock.io/)
- [Linux Kernel Landlock Documentation](https://docs.kernel.org/userspace-api/landlock.html)
- [go-landlock Library](https://github.com/landlock-lsm/go-landlock)

