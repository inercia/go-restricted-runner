# Sandbox-Exec and Firejail Runners

**Trigger**: If the user prompt mentions "sandbox", "firejail", "sandbox-exec", "macOS", "Linux", "isolation", or "profile".

## Overview

Both SandboxExec (macOS) and Firejail (Linux) provide medium-level isolation using OS-specific sandboxing tools.

### Common Pattern

Both runners follow the same implementation pattern:
1. Process template variables in restriction options
2. Generate sandbox/firejail profile from template
3. Write profile to temporary file
4. Execute command with sandbox tool and profile
5. Clean up profile file after execution

## SandboxExec (macOS)

### Platform Check
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

### SandboxExecOptions
```go
type SandboxExecOptions struct {
    Shell             string   // Shell to use
    AllowNetworking   bool     // Allow network access
    AllowUserFolders  bool     // Allow access to user folders
    AllowReadFolders  []string // Folders allowed for reading
    AllowWriteFolders []string // Folders allowed for writing
    AllowReadFiles    []string // Files allowed for reading
    AllowWriteFiles   []string // Files allowed for writing
    CustomProfile     string   // Custom sandbox profile
}
```

### Profile Template
Located in `pkg/runner/sandbox_profile.tpl`:
```scheme
(version 1)
(deny default)

{{if .AllowNetworking}}
(allow network*)
{{else}}
(deny network*)
{{end}}

{{range .AllowReadFolders}}
(allow file-read* (subpath "{{.}}"))
{{end}}

{{range .AllowWriteFolders}}
(allow file-write* (subpath "{{.}}"))
{{end}}
```

### Command Execution
```go
// Single executable
execCmd = exec.CommandContext(ctx, "sandbox-exec", "-f", profileFile.Name(), cmd)

// Script-based
execCmd = exec.CommandContext(ctx, "sandbox-exec", "-f", profileFile.Name(), scriptFile)
```

## Firejail (Linux)

### Platform Check
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

### FirejailOptions
```go
type FirejailOptions struct {
    Shell             string   // Shell to use
    AllowNetworking   bool     // Allow network access
    AllowUserFolders  bool     // Allow access to user folders
    AllowReadFolders  []string // Folders allowed for reading
    AllowWriteFolders []string // Folders allowed for writing
    AllowReadFiles    []string // Files allowed for reading
    AllowWriteFiles   []string // Files allowed for writing
    CustomProfile     string   // Custom firejail profile
}
```

### Profile Template
Located in `pkg/runner/firejail_profile.tpl`:
```
{{if not .AllowNetworking}}
net none
{{end}}

{{if not .AllowUserFolders}}
private
{{end}}

{{range .AllowReadFolders}}
read-only {{.}}
{{end}}

{{range .AllowWriteFolders}}
whitelist {{.}}
{{end}}
```

### Command Execution
```go
// Single executable
execCmd = exec.CommandContext(ctx, "firejail", "--profile="+profileFile.Name(), cmd)

// Script-based
execCmd = exec.CommandContext(ctx, "firejail", "--profile="+profileFile.Name(), scriptFile)
```

## Common Implementation Pattern

### Run() Method
```go
func (r *SandboxExec) Run(ctx context.Context, shell, command string, env []string, params map[string]interface{}, tmpfile bool) (string, error) {
    // 1. Check context
    select {
    case <-ctx.Done():
        return "", ctx.Err()
    default:
    }
    
    // 2. Process template variables in restrictions
    if len(r.options.AllowReadFolders) > 0 {
        r.options.AllowReadFolders = common.ProcessTemplateListFlexible(
            r.options.AllowReadFolders, params)
    }
    // ... process other lists
    
    // 3. Generate profile
    var profileBuf bytes.Buffer
    if err := r.profileTpl.Execute(&profileBuf, r.options); err != nil {
        return "", fmt.Errorf("failed to render profile: %w", err)
    }
    
    // 4. Create temporary profile file
    profileFile, err := os.CreateTemp("", "sandbox-profile-*.sb")
    if err != nil {
        return "", fmt.Errorf("failed to create profile file: %w", err)
    }
    defer os.Remove(profileFile.Name())
    
    // 5. Write profile
    if _, err := profileFile.Write(profileBuf.Bytes()); err != nil {
        return "", fmt.Errorf("failed to write profile: %w", err)
    }
    profileFile.Close()
    
    // 6. Execute command
    var execCmd *exec.Cmd
    if isSingleExecutableCommand(command) {
        execCmd = exec.CommandContext(ctx, "sandbox-exec", "-f", profileFile.Name(), command)
    } else {
        // Create script file
        tmpScript, _ := os.CreateTemp("", "sandbox-script-*.sh")
        defer os.Remove(tmpScript.Name())
        // ... write script
        execCmd = exec.CommandContext(ctx, "sandbox-exec", "-f", profileFile.Name(), tmpScript.Name())
    }
    
    // 7. Set environment and capture output
    if len(env) > 0 {
        execCmd.Env = append(os.Environ(), env...)
    }
    
    var stdout, stderr bytes.Buffer
    execCmd.Stdout = &stdout
    execCmd.Stderr = &stderr
    
    // 8. Run and return
    if err := execCmd.Run(); err != nil {
        if stderr.Len() > 0 {
            return "", errors.New(stderr.String())
        }
        return "", err
    }
    
    return stdout.String(), nil
}
```

### RunWithPipes() Method
```go
func (r *SandboxExec) RunWithPipes(ctx context.Context, cmd string, args []string, env []string, params map[string]interface{}) (...) {
    // 1-3. Same as Run(): check context, process templates, generate profile
    
    // 4. Build sandboxed command
    sandboxArgs := []string{"-f", profileFile.Name(), cmd}
    sandboxArgs = append(sandboxArgs, args...)
    execCmd := exec.CommandContext(ctx, "sandbox-exec", sandboxArgs...)
    
    // 5. Create pipes
    stdinPipe, _ := execCmd.StdinPipe()
    stdoutPipe, _ := execCmd.StdoutPipe()
    stderrPipe, _ := execCmd.StderrPipe()
    
    // 6. Start command
    execCmd.Start()
    
    // 7. Create wait function WITH CLEANUP
    waitFunc := func() error {
        err := execCmd.Wait()
        
        // Clean up profile file
        if removeErr := os.Remove(profileFile.Name()); removeErr != nil {
            r.logger.Debug("Warning: failed to remove profile file: %v", removeErr)
        }
        
        return err
    }
    
    return stdinPipe, stdoutPipe, stderrPipe, waitFunc, nil
}
```

## Template Variable Processing

Both runners support template variables in restriction paths:

```go
// User provides
Options{
    "allow_read_folders": []string{"{{.workdir}}", "/tmp"},
}

// With params
params := map[string]interface{}{
    "workdir": "/home/user/project",
}

// Results in
AllowReadFolders: []string{"/home/user/project", "/tmp"}
```

### Processing Pattern
```go
if len(r.options.AllowReadFolders) > 0 {
    r.options.AllowReadFolders = common.ProcessTemplateListFlexible(
        r.options.AllowReadFolders, params)
}
```

## Profile File Management

### Temporary File Creation
```go
// SandboxExec
profileFile, err := os.CreateTemp("", "sandbox-profile-*.sb")

// Firejail
profileFile, err := os.CreateTemp("", "firejail-profile-*.profile")
```

### Cleanup Strategies

**In Run()**: Use defer
```go
defer os.Remove(profileFile.Name())
```

**In RunWithPipes()**: Clean up in wait()
```go
waitFunc := func() error {
    err := execCmd.Wait()
    os.Remove(profileFile.Name())  // Always clean up
    return err
}
```

## Testing Sandbox Runners

### Platform-Specific Skipping
```go
func TestSandboxExec_Run(t *testing.T) {
    if runtime.GOOS != "darwin" {
        t.Skip("Skipping sandbox-exec tests on non-macOS platform")
    }
    // ... test code
}
```

### Test Restrictions
```go
func TestSandboxExec_NetworkRestriction(t *testing.T) {
    runner, _ := NewSandboxExec(Options{
        "allow_networking": false,
    }, logger)
    
    // This should fail due to network restriction
    _, err := runner.Run(ctx, "sh", "curl https://example.com", nil, nil, false)
    if err == nil {
        t.Error("Expected network access to be blocked")
    }
}
```

## Common Issues

### Issue: Profile File Not Cleaned Up
**Solution**: Always clean up in defer (Run) or wait() (RunWithPipes)

### Issue: Template Variables Not Processed
**Solution**: Call `ProcessTemplateListFlexible()` for all restriction lists

### Issue: Custom Profile Ignored
**Solution**: Check if `CustomProfile` is set before generating from template

