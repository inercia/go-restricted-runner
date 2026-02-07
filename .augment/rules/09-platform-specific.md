# Platform-Specific Code Patterns

**Trigger**: If the user prompt mentions "platform", "Windows", "macOS", "Linux", "darwin", "cross-platform", or "build tags".

## Platform Detection

### Runtime Checks
Use `runtime.GOOS` for platform detection:

```go
import "runtime"

if runtime.GOOS == "darwin" {
    // macOS-specific code
} else if runtime.GOOS == "linux" {
    // Linux-specific code
} else if runtime.GOOS == "windows" {
    // Windows-specific code
}
```

### Platform Constants
```go
const (
    platformDarwin  = "darwin"
    platformLinux   = "linux"
    platformWindows = "windows"
)
```

## File Organization

### Platform-Specific Files
Use build tags for platform-specific implementations:

```
pkg/runner/
  shell.go           # Common interface/types
  shell_unix.go      # Unix implementation (Linux, macOS)
  shell_windows.go   # Windows implementation
```

### Build Tags
```go
// shell_unix.go
//go:build !windows
// +build !windows

package runner

// Unix-specific implementation
```

```go
// shell_windows.go
//go:build windows
// +build windows

package runner

// Windows-specific implementation
```

## Shell Handling

### Default Shells by Platform
```go
func getDefaultShell() string {
    switch runtime.GOOS {
    case "windows":
        return "cmd"
    case "darwin", "linux":
        return "sh"
    default:
        return "sh"
    }
}
```

### Shell Command Arguments
Different shells have different argument patterns:

```go
func getShellCommandArgs(shell, command string) (string, []string) {
    shellLower := strings.ToLower(shell)
    
    // Windows shells
    if isWindowsShell(shellLower) {
        if shellLower == "cmd" {
            return shell, []string{"/c", command}
        }
        if shellLower == "powershell" {
            return shell, []string{"-Command", command}
        }
    }
    
    // Unix shells
    return shell, []string{"-c", command}
}
```

### Windows Shell Detection
```go
func isWindowsShell(shell string) bool {
    shell = strings.ToLower(shell)
    return shell == "cmd" || 
           shell == "powershell" || 
           shell == "pwsh" ||
           strings.HasSuffix(shell, "cmd.exe") ||
           strings.HasSuffix(shell, "powershell.exe")
}
```

## Platform-Specific Runners

### SandboxExec (macOS Only)
```go
func (r *SandboxExec) CheckImplicitRequirements() error {
    if runtime.GOOS != "darwin" {
        return fmt.Errorf("sandbox-exec runner requires macOS")
    }
    
    if !common.CheckExecutableExists("sandbox-exec") {
        return fmt.Errorf("sandbox-exec executable not found in PATH")
    }
    
    return nil
}
```

### Firejail (Linux Only)
```go
func (r *Firejail) CheckImplicitRequirements() error {
    if runtime.GOOS != "linux" {
        return fmt.Errorf("firejail runner requires Linux")
    }
    
    if !common.CheckExecutableExists("firejail") {
        return fmt.Errorf("firejail executable not found in PATH")
    }
    
    return nil
}
```

## Testing Platform-Specific Code

### Skip Tests on Wrong Platform
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

### Platform-Specific Test Commands
```go
func TestExecRunner_RunWithPipes_StderrCapture(t *testing.T) {
    var cmd string
    var args []string
    
    if runtime.GOOS == "windows" {
        cmd = "cmd"
        args = []string{"/c", "echo error message 1>&2"}
    } else {
        cmd = "sh"
        args = []string{"-c", "echo 'error message' >&2"}
    }
    
    stdin, stdout, stderr, wait, err := runner.RunWithPipes(ctx, cmd, args, nil, nil)
    // ... test code
}
```

## Path Handling

### Path Separators
```go
import "path/filepath"

// Use filepath.Join for cross-platform paths
path := filepath.Join("dir", "subdir", "file.txt")
// Windows: dir\subdir\file.txt
// Unix: dir/subdir/file.txt

// Use filepath.Separator for the platform separator
sep := string(filepath.Separator)
```

### Executable Extensions
```go
func getExecutableName(name string) string {
    if runtime.GOOS == "windows" {
        if !strings.HasSuffix(name, ".exe") {
            return name + ".exe"
        }
    }
    return name
}
```

## Environment Variables

### PATH Separator
```go
import "os"

// Get PATH separator
pathSep := string(os.PathListSeparator)
// Windows: ;
// Unix: :

// Split PATH
paths := strings.Split(os.Getenv("PATH"), pathSep)
```

### Environment Variable Names
```go
// Windows is case-insensitive, Unix is case-sensitive
// Use uppercase for consistency
env := []string{
    "PATH=/usr/bin",
    "HOME=/home/user",
}
```

## Command Execution Differences

### Windows Considerations

1. **No tmpfile for Windows shells**:
```go
if runtime.GOOS == "windows" && isWindowsShell(shell) {
    // Use direct execution for Windows shells
    execCmd = exec.CommandContext(ctx, shell, "/c", command)
} else {
    // Use temp file for Unix
    tmpFile, _ := os.CreateTemp("", "script-*.sh")
    // ... write script
    execCmd = exec.CommandContext(ctx, shell, tmpFile.Name())
}
```

2. **Different command syntax**:
```go
// Unix
command := "ls -la"

// Windows
command := "dir"
```

3. **Different line endings**:
```go
// Unix: \n
// Windows: \r\n
// Use bufio.Scanner which handles both
```

## Docker Platform Considerations

### Platform Specification
```go
type DockerOptions struct {
    // Set platform if server is multi-platform capable
    Platform string `json:"platform"`  // e.g., "linux/amd64", "linux/arm64"
}

// Usage
dockerRunArgs = append(dockerRunArgs, "--platform", r.opts.Platform)
```

### Volume Mounts
```go
// Windows paths need special handling
if runtime.GOOS == "windows" {
    // Convert C:\path to /c/path for Docker
    hostPath = convertWindowsPath(hostPath)
}

mount := fmt.Sprintf("%s:%s", hostPath, containerPath)
```

## Best Practices

### 1. Use Runtime Checks for Behavior
```go
// GOOD - runtime check
if runtime.GOOS == "windows" {
    // Windows-specific behavior
}

// AVOID - build tags for simple checks
// Use build tags only for separate implementations
```

### 2. Provide Cross-Platform Defaults
```go
func NewExec(options Options, logger *common.Logger) (*Exec, error) {
    execOptions, _ := NewExecOptions(options)
    
    // Use platform-appropriate default shell
    if execOptions.Shell == "" {
        execOptions.Shell = getDefaultShell()
    }
    
    return &Exec{options: execOptions, logger: logger}, nil
}
```

### 3. Test on Multiple Platforms
- Test on macOS, Linux, and Windows if possible
- Use CI/CD with multiple platforms
- Document platform-specific behavior

### 4. Document Platform Requirements
```go
// SandboxExec implements the Runner interface using macOS sandbox-exec
//
// Platform Requirements:
//   - macOS (darwin)
//   - sandbox-exec executable in PATH
type SandboxExec struct {
    // ...
}
```

### 5. Graceful Degradation
```go
// Try platform-specific optimization, fall back to generic
if runtime.GOOS == "linux" && common.CheckExecutableExists("firejail") {
    // Use firejail
} else {
    // Fall back to exec
}
```

## Common Pitfalls

### ❌ Hardcoded Paths
```go
// BAD
scriptPath := "/tmp/script.sh"

// GOOD
scriptPath := filepath.Join(os.TempDir(), "script.sh")
```

### ❌ Assuming Unix Commands
```go
// BAD
cmd := "ls -la"

// GOOD
var cmd string
if runtime.GOOS == "windows" {
    cmd = "dir"
} else {
    cmd = "ls -la"
}
```

### ❌ Ignoring Platform in Tests
```go
// BAD
func TestSomething(t *testing.T) {
    // Assumes Unix
    cmd := "cat /etc/hosts"
}

// GOOD
func TestSomething(t *testing.T) {
    if runtime.GOOS == "windows" {
        t.Skip("Test requires Unix")
    }
    cmd := "cat /etc/hosts"
}
```

