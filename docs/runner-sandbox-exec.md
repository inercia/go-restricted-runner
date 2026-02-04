# Sandbox-Exec Runner

The Sandbox-Exec runner provides process isolation on macOS using the built-in `sandbox-exec` tool. It uses Apple's Seatbelt sandbox technology to restrict what commands can do.

## How It Works

1. **Profile Generation**: A sandbox profile (`.sb` file) is generated from a template based on the configured options
2. **Temporary Files**: The profile and command script are written to temporary files
3. **Sandboxed Execution**: The command runs inside `sandbox-exec -f <profile> <command>`
4. **Cleanup**: Temporary files are removed after execution

### Sandbox Profile

The default profile:
- Allows most operations by default (`(allow default)`)
- Denies writes to system directories (`/bin`, `/sbin`, `/usr`, `/etc`, `/System`, `/Library`)
- Optionally restricts network access
- Optionally restricts access to user folders (Documents, Desktop, etc.)
- Allows explicit read/write access to specified paths

## Pros and Cons

### Pros

- ✅ **Built into macOS**: No additional software installation required
- ✅ **Low overhead**: Native kernel-level sandboxing
- ✅ **Fine-grained control**: Control network, filesystem access per-path
- ✅ **Custom profiles**: Support for fully custom sandbox profiles
- ✅ **Automatic cleanup**: Temporary files managed automatically

### Cons

- ❌ **macOS only**: Not available on Linux or Windows
- ❌ **Deprecated API**: Apple has deprecated `sandbox-exec` (but it still works)
- ❌ **Complex profiles**: Sandbox profile language has a learning curve
- ❌ **Limited documentation**: Apple doesn't officially document the profile format
- ❌ **No resource limits**: Cannot limit CPU or memory usage

## Limitations

- Only works on macOS (darwin)
- `tmpfile` parameter is ignored (always uses temporary scripts)
- Sandbox profile errors can be cryptic
- Some system operations may fail unexpectedly due to sandbox restrictions
- Cannot restrict CPU or memory usage
- Apple may remove `sandbox-exec` in future macOS versions

## API Usage

### Basic Usage

```go
import (
    "context"
    "github.com/inercia/go-restricted-runner/pkg/common"
    "github.com/inercia/go-restricted-runner/pkg/runner"
)

logger, _ := common.NewLogger("", "", common.LogLevelInfo, false)

// Create a Sandbox-Exec runner
r, err := runner.New(runner.TypeSandboxExec, runner.Options{}, logger)
if err != nil {
    log.Fatal(err) // Will fail on non-macOS systems
}

ctx := context.Background()
output, err := r.Run(ctx, "sh", "echo 'Hello from sandbox!'", nil, nil, false)
```

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `shell` | `string` | System default | Shell to use for execution |
| `allow_networking` | `bool` | `false` | Allow network access |
| `allow_user_folders` | `bool` | `false` | Allow access to user folders |
| `allow_read_folders` | `[]string` | `[]` | Folders to allow read access |
| `allow_write_folders` | `[]string` | `[]` | Folders to allow write access |
| `allow_read_files` | `[]string` | `[]` | Specific files to allow read access |
| `allow_write_files` | `[]string` | `[]` | Specific files to allow write access |
| `custom_profile` | `string` | `""` | Complete custom sandbox profile |

### Disable Network Access

```go
r, err := runner.New(runner.TypeSandboxExec, runner.Options{
    "allow_networking": false,
}, logger)

// This will fail - network is blocked
output, err := r.Run(ctx, "sh", "curl https://example.com", nil, nil, false)
```

### Allow Specific Folders

```go
r, err := runner.New(runner.TypeSandboxExec, runner.Options{
    "allow_networking":    false,
    "allow_read_folders":  []string{"/tmp", "/Users/me/project"},
    "allow_write_folders": []string{"/tmp/output"},
}, logger)
```

### With Template Variables

```go
r, err := runner.New(runner.TypeSandboxExec, runner.Options{
    "allow_read_folders": []string{"{{.workdir}}"},
}, logger)

params := map[string]interface{}{
    "workdir": "/Users/me/myproject",
}

output, err := r.Run(ctx, "sh", "ls -la", nil, params, false)
```

### Custom Sandbox Profile

```go
customProfile := `
(version 1)
(deny default)
(allow process-exec)
(allow file-read* (subpath "/usr"))
(allow file-read* (subpath "/bin"))
(allow file-read* (subpath "/tmp"))
`

r, err := runner.New(runner.TypeSandboxExec, runner.Options{
    "custom_profile": customProfile,
}, logger)
```

## Default Profile Details

The default sandbox profile template:

```scheme
(version 1)
(allow default)

;; Protect system directories from writes
(deny file-write* (subpath "/bin"))
(deny file-write* (subpath "/sbin"))
(deny file-write* (subpath "/usr/bin"))
(deny file-write* (subpath "/usr/sbin"))
(deny file-write* (subpath "/etc"))
(deny file-write* (subpath "/System"))
(deny file-write* (subpath "/Library"))
;; ... more system paths

;; Network control (based on allow_networking option)
(deny network*)  ;; or (allow network*)

;; User folders control (based on allow_user_folders option)
(deny file-read-data (regex "^/Users/.*/(Documents|Desktop|Downloads|...)"))
```

## Implicit Requirements

The Sandbox-Exec runner checks these requirements on creation:

1. **Operating System**: Must be macOS (`runtime.GOOS == "darwin"`)
2. **Executable**: `sandbox-exec` must be available in PATH

```go
r, err := runner.New(runner.TypeSandboxExec, runner.Options{}, logger)
if err != nil {
    // Possible errors:
    // - "sandbox-exec runner requires macOS"
    // - "sandbox-exec executable not found in PATH"
}
```

## Security Considerations

- The default profile uses `(allow default)` - it's permissive by design
- For maximum security, use a custom profile starting with `(deny default)`
- Always test your sandbox profile thoroughly before production use
- Some commands may fail silently when sandboxed - check for unexpected behavior

## See Also

- [Exec Runner](runner-exec.md) - No isolation
- [Firejail Runner](runner-firejail.md) - Linux alternative
- [Docker Runner](runner-docker.md) - Container-based isolation
- [Apple Sandbox Guide](https://reverse.put.as/wp-content/uploads/2011/09/Apple-Sandbox-Guide-v1.0.pdf) - Unofficial documentation
