# Firejail Runner

The Firejail runner provides process isolation on Linux using [Firejail](https://firejail.wordpress.com/), a SUID sandbox program that reduces the risk of security breaches by restricting the running environment of untrusted applications.

## How It Works

1. **Profile Generation**: A firejail profile is generated from a template based on configured options
2. **Temporary Files**: The profile and command script are written to temporary files
3. **Sandboxed Execution**: The command runs inside `firejail --profile=<profile> <command>`
4. **Security Features**: Applies seccomp filters, capability dropping, and namespace isolation
5. **Cleanup**: Temporary files are removed after execution

### Default Security Features

The default profile automatically applies:
- **seccomp**: System call filtering
- **caps.drop all**: Drop all Linux capabilities
- **noroot**: Prevent running as root inside the sandbox

## Pros and Cons

### Pros

- ✅ **Powerful isolation**: Uses Linux namespaces, seccomp, and capabilities
- ✅ **Active development**: Well-maintained open-source project
- ✅ **Extensive documentation**: Well-documented profile format
- ✅ **Fine-grained control**: Control network, filesystem, and more
- ✅ **Custom profiles**: Support for fully custom firejail profiles
- ✅ **Low overhead**: Minimal performance impact

### Cons

- ❌ **Linux only**: Not available on macOS or Windows
- ❌ **Requires installation**: Firejail must be installed separately
- ❌ **SUID binary**: Firejail requires setuid permissions
- ❌ **Complexity**: Many options can be overwhelming
- ❌ **No resource limits**: Memory/CPU limits require cgroups (not exposed)

## Limitations

- Only works on Linux
- `tmpfile` parameter is ignored (always uses temporary scripts)
- Firejail must be installed and properly configured
- Some applications may not work correctly when sandboxed
- Cannot directly limit CPU or memory (would need cgroups integration)

## API Usage

### Basic Usage

```go
import (
    "context"
    "github.com/inercia/go-restricted-runner/pkg/common"
    "github.com/inercia/go-restricted-runner/pkg/runner"
)

logger, _ := common.NewLogger("", "", common.LogLevelInfo, false)

// Create a Firejail runner
r, err := runner.New(runner.TypeFirejail, runner.Options{}, logger)
if err != nil {
    log.Fatal(err) // Will fail on non-Linux systems or if firejail not installed
}

ctx := context.Background()
output, err := r.Run(ctx, "sh", "echo 'Hello from firejail!'", nil, nil, false)
```

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `shell` | `string` | System default | Shell to use for execution |
| `allow_networking` | `bool` | `false` | Allow network access |
| `allow_user_folders` | `bool` | `false` | Allow access to user folders |
| `allow_read_folders` | `[]string` | `[]` | Folders to allow read access (whitelist + read-only) |
| `allow_write_folders` | `[]string` | `[]` | Folders to allow write access (whitelist) |
| `allow_read_files` | `[]string` | `[]` | Specific files to allow read access |
| `allow_write_files` | `[]string` | `[]` | Specific files to allow write access |
| `custom_profile` | `string` | `""` | Complete custom firejail profile |

### Disable Network Access

```go
r, err := runner.New(runner.TypeFirejail, runner.Options{
    "allow_networking": false,
}, logger)

// This will fail - network is blocked with "net none"
output, err := r.Run(ctx, "sh", "curl https://example.com", nil, nil, false)
```

### Allow Specific Folders

```go
r, err := runner.New(runner.TypeFirejail, runner.Options{
    "allow_networking":    false,
    "allow_read_folders":  []string{"/home/user/project", "/tmp"},
    "allow_write_folders": []string{"/tmp/output"},
}, logger)
```

### With Template Variables

```go
r, err := runner.New(runner.TypeFirejail, runner.Options{
    "allow_read_folders": []string{"{{.workdir}}"},
}, logger)

params := map[string]interface{}{
    "workdir": "/home/user/myproject",
}

output, err := r.Run(ctx, "sh", "ls -la", nil, params, false)
```

### Custom Firejail Profile

```go
customProfile := `
# Minimal custom profile
net none
seccomp
caps.drop all
noroot
whitelist /tmp
whitelist /usr/bin
`

r, err := runner.New(runner.TypeFirejail, runner.Options{
    "custom_profile": customProfile,
}, logger)
```

## Default Profile Details

The default firejail profile template:

```ini
# Network restrictions
net none  # if allow_networking is false

# File system restrictions (if allow_user_folders is false)
blacklist ${HOME}/Documents
blacklist ${HOME}/Desktop
blacklist ${HOME}/Downloads
blacklist ${HOME}/Pictures
blacklist ${HOME}/Videos
blacklist ${HOME}/Music

# Whitelisted paths
whitelist /path/to/allowed
read-only /path/to/readonly

# Security features (always applied)
seccomp
caps.drop all
noroot
```

## Implicit Requirements

The Firejail runner checks these requirements on creation:

1. **Operating System**: Must be Linux (`runtime.GOOS == "linux"`)
2. **Executable**: `firejail` must be available in PATH

```go
r, err := runner.New(runner.TypeFirejail, runner.Options{}, logger)
if err != nil {
    // Possible errors:
    // - "firejail runner requires Linux"
    // - "firejail executable not found in PATH"
}
```

## Installing Firejail

### Debian/Ubuntu

```bash
sudo apt install firejail
```

### Fedora

```bash
sudo dnf install firejail
```

### Arch Linux

```bash
sudo pacman -S firejail
```

### From Source

```bash
git clone https://github.com/netblue30/firejail.git
cd firejail
./configure && make && sudo make install-strip
```

## Security Considerations

- Firejail is a SUID binary - ensure it's from a trusted source
- The default profile is moderately restrictive but not maximum security
- Test your profile thoroughly before production use
- Consider using `--private` for full home directory isolation
- Firejail can be combined with AppArmor or SELinux for additional security

## See Also

- [Exec Runner](runner-exec.md) - No isolation
- [Sandbox-Exec Runner](runner-sandbox-exec.md) - macOS alternative
- [Docker Runner](runner-docker.md) - Container-based isolation
- [Firejail Documentation](https://firejail.wordpress.com/documentation-2/) - Official docs
- [Firejail GitHub](https://github.com/netblue30/firejail) - Source code

