# Landrun Runner Integration - Implementation Summary

This document summarizes the implementation of the Landrun runner integration into go-restricted-runner.

## Overview

The Landrun runner provides kernel-native sandboxing for Linux using the Landlock LSM (Linux Security Module). It offers fine-grained filesystem and network access control without requiring root privileges or external binaries.

## What Was Implemented

### 1. Core Implementation Files

#### `pkg/runner/landrun.go`
- **Landrun struct**: Main runner implementation
- **LandrunOptions struct**: Configuration options for filesystem and network restrictions
- **NewLandrun()**: Factory function for creating Landrun instances
- **CheckImplicitRequirements()**: Validates Landlock availability on the system
- **buildLandlockRules()**: Constructs Landlock rules from options and template parameters
- **Run()**: Executes commands with Landlock restrictions
- **RunWithPipes()**: Executes interactive commands with stdin/stdout/stderr pipes

**Key Features:**
- Filesystem access control (read, write, execute permissions)
- Network access control (TCP bind/connect on kernel 6.7+)
- Template variable support in paths
- Best-effort mode for graceful degradation on older kernels
- Unrestricted modes for filesystem and networking

#### `pkg/runner/landrun_test.go`
Comprehensive test suite covering:
- Basic command execution
- Filesystem restrictions (read/write/execute)
- Template variable processing
- Environment variables
- Context cancellation
- RunWithPipes functionality
- Best-effort mode
- Multiple writes and concurrent I/O

### 2. Integration Updates

#### `pkg/runner/runner.go`
- Added `TypeLandrun` constant to runner types
- Updated factory function `New()` to support Landrun creation
- Added documentation for Landrun requirements

#### `go.mod`
- Added `github.com/landlock-lsm/go-landlock v0.6.0` as a direct dependency
- Includes transitive dependencies: `golang.org/x/sys` and `kernel.org/pub/linux/libs/security/libcap/psx`

### 3. Documentation

#### `docs/runner-landrun.md`
Comprehensive documentation including:
- Overview and requirements
- Configuration options
- Usage examples (basic, filesystem restrictions, network restrictions, templates, RunWithPipes)
- How it works
- Landlock access rights explanation
- Comparison with other runners
- Limitations and troubleshooting
- References

#### `README.md`
- Added Landrun to the list of supported runners
- Added usage example for Landrun

#### `docs/README.md`
- Added Landrun to runner types table
- Updated comparison matrix
- Added Landrun to runner types list

### 4. Examples

#### `examples/landrun_example.go`
Practical examples demonstrating:
- Basic command execution with unrestricted access
- Filesystem restrictions
- Interactive process with RunWithPipes
- Template variables in paths

## Key Features

### Filesystem Access Control
```go
runner.Options{
    "allow_read_folders": []string{"/usr", "/lib", "/etc"},
    "allow_read_exec_folders": []string{"/usr/bin", "/bin"},
    "allow_write_folders": []string{"/tmp"},
}
```

### Network Access Control (Kernel 6.7+)
```go
runner.Options{
    "allow_bind_tcp": []uint16{8080},
    "allow_connect_tcp": []uint16{80, 443},
}
```

### Template Variables
```go
runner.Options{
    "allow_read_folders": []string{"{{.workdir}}", "/usr"},
}
// Use with params
params := map[string]interface{}{"workdir": "/home/user/project"}
```

### Best Effort Mode
```go
runner.Options{
    "best_effort": true,  // Gracefully degrade on older kernels
}
```

## Technical Details

### How It Works
1. When `Run()` or `RunWithPipes()` is called, Landlock restrictions are applied to the current process
2. All child processes inherit the same restrictions
3. The Linux kernel enforces restrictions at the system call level
4. Restrictions cannot be relaxed once applied (only made stricter)

### Platform Requirements
- **Operating System**: Linux only
- **Kernel Version**: 
  - Linux 5.13+ for basic filesystem sandboxing
  - Linux 6.7+ for network restrictions
- **Landlock Support**: Kernel must be compiled with `CONFIG_SECURITY_LANDLOCK=y`

### Comparison with Other Runners

| Feature | Landrun | Firejail | Sandbox-exec | Docker |
|---------|---------|----------|--------------|--------|
| Platform | Linux | Linux | macOS | All |
| Kernel Requirement | 5.13+ | Any | Any | Any |
| External Binary | No | Yes | Yes | Yes |
| Root Required | No | No | No | Sometimes |
| Isolation Level | Medium-High | Medium | Medium | High |
| Overhead | Minimal | Low | Low | High |
| Network Control | Yes (6.7+) | Yes | Yes | Yes |

## Testing

All tests pass successfully:
- Tests properly skip on non-Linux platforms
- Tests skip gracefully when Landlock is not available
- Comprehensive coverage of all features
- Integration with existing test suite

## Files Modified/Created

### Created:
- `pkg/runner/landrun.go` (344 lines)
- `pkg/runner/landrun_test.go` (423 lines)
- `docs/runner-landrun.md` (comprehensive documentation)
- `examples/landrun_example.go` (example code)
- `LANDRUN_INTEGRATION.md` (this file)

### Modified:
- `pkg/runner/runner.go` (added TypeLandrun constant and factory case)
- `go.mod` (added go-landlock dependency)
- `go.sum` (updated with new dependencies)
- `README.md` (added Landrun to overview and examples)
- `docs/README.md` (added Landrun to documentation index and comparison)

## Usage Example

```go
package main

import (
    "context"
    "log"
    "github.com/inercia/go-restricted-runner/pkg/common"
    "github.com/inercia/go-restricted-runner/pkg/runner"
)

func main() {
    logger, _ := common.NewLogger("", "", common.LogLevelInfo, false)
    
    r, err := runner.New(runner.TypeLandrun, runner.Options{
        "allow_read_folders": []string{"/usr", "/lib", "/etc"},
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
    
    println(output)
}
```

## Conclusion

The Landrun runner integration is complete and fully functional. It provides a modern, kernel-native sandboxing solution for Linux that fills the gap between firejail and Docker, offering fine-grained control with minimal overhead and no external dependencies.

