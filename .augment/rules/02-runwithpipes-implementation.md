# RunWithPipes Implementation Patterns

**Trigger**: If the user prompt mentions "pipes", "stdin", "stdout", "stderr", "interactive", "streaming", or "RunWithPipes".

## RunWithPipes Method Overview

`RunWithPipes()` provides interactive I/O for processes. It was added to support long-running and interactive commands.

### Method Signature

```go
RunWithPipes(ctx context.Context, cmd string, args []string, env []string, params map[string]interface{}) (
    stdin io.WriteCloser,
    stdout io.ReadCloser,
    stderr io.ReadCloser,
    wait func() error,
    err error,
)
```

### Key Differences from Run()

| Aspect | Run() | RunWithPipes() |
|--------|-------|----------------|
| Execution | Shell-based | Direct command |
| Output | String after completion | Streaming via pipes |
| Input | None | Via stdin pipe |
| Lifecycle | Synchronous | Async (requires wait()) |
| Use case | One-shot commands | Interactive/long-running |

## Implementation Pattern for Each Runner

### ExecRunner (pkg/runner/exec.go)

**Simplest implementation** - baseline for others:

```go
func (r *Exec) RunWithPipes(ctx context.Context, cmd string, args []string, env []string, params map[string]interface{}) (...) {
    // 1. Check context
    select {
    case <-ctx.Done():
        return nil, nil, nil, nil, ctx.Err()
    default:
    }
    
    // 2. Create command
    execCmd := exec.CommandContext(ctx, cmd, args...)
    
    // 3. Set environment
    if len(env) > 0 {
        execCmd.Env = append(os.Environ(), env...)
    }
    
    // 4. Create pipes (order matters for cleanup)
    stdinPipe, err := execCmd.StdinPipe()
    if err != nil {
        return nil, nil, nil, nil, err
    }
    
    stdoutPipe, err := execCmd.StdoutPipe()
    if err != nil {
        stdinPipe.Close()  // Clean up already-created pipe
        return nil, nil, nil, nil, err
    }
    
    stderrPipe, err := execCmd.StderrPipe()
    if err != nil {
        stdinPipe.Close()
        stdoutPipe.Close()
        return nil, nil, nil, nil, err
    }
    
    // 5. Start command
    if err := execCmd.Start(); err != nil {
        stdinPipe.Close()
        stdoutPipe.Close()
        stderrPipe.Close()
        return nil, nil, nil, nil, err
    }
    
    // 6. Create wait function
    waitFunc := func() error {
        return execCmd.Wait()
    }
    
    return stdinPipe, stdoutPipe, stderrPipe, waitFunc, nil
}
```

### SandboxExec/Firejail Pattern

**Add isolation layer** around exec pattern:

```go
func (r *SandboxExec) RunWithPipes(...) (...) {
    // 1. Process template variables in restrictions
    r.options.AllowReadFolders = common.ProcessTemplateListFlexible(
        r.options.AllowReadFolders, params)
    // ... process other restriction lists
    
    // 2. Generate and write profile
    var profileBuf bytes.Buffer
    r.profileTpl.Execute(&profileBuf, r.options)
    
    profileFile, err := os.CreateTemp("", "sandbox-profile-*.sb")
    profileFile.Write(profileBuf.Bytes())
    profileFile.Close()
    
    // 3. Build sandboxed command
    // sandbox-exec: sandbox-exec -f <profile> <cmd> <args...>
    // firejail: firejail --profile=<profile> <cmd> <args...>
    sandboxArgs := []string{"-f", profileFile.Name(), cmd}
    sandboxArgs = append(sandboxArgs, args...)
    execCmd := exec.CommandContext(ctx, "sandbox-exec", sandboxArgs...)
    
    // 4. Create pipes (same as ExecRunner)
    // ... stdinPipe, stdoutPipe, stderrPipe
    
    // 5. Start command
    execCmd.Start()
    
    // 6. Create wait function WITH CLEANUP
    waitFunc := func() error {
        err := execCmd.Wait()
        
        // Clean up profile file
        os.Remove(profileFile.Name())
        
        return err
    }
    
    return stdinPipe, stdoutPipe, stderrPipe, waitFunc, nil
}
```

### Docker Pattern

**Container-based approach** - different from others:

```go
func (r *Docker) RunWithPipes(...) (...) {
    // 1. Create long-running background container
    containerName := fmt.Sprintf("go-restricted-runner-%d", time.Now().UnixNano())
    
    dockerRunArgs := []string{"run", "--name", containerName, "-d"}
    // Add restrictions: --network, --memory, --user, -v mounts, -e env
    dockerRunArgs = append(dockerRunArgs, r.opts.Image, "sleep", "infinity")
    
    createCmd := exec.CommandContext(ctx, "docker", dockerRunArgs...)
    createCmd.CombinedOutput()  // Create container
    
    // 2. Build docker exec command
    execArgs := []string{"exec", "-i", containerName, cmd}
    execArgs = append(execArgs, args...)
    execCmd := exec.CommandContext(ctx, "docker", execArgs...)
    
    // 3. Create pipes (same pattern)
    // ... but clean up container on any error
    
    // 4. Start docker exec
    execCmd.Start()
    
    // 5. Create wait function WITH CONTAINER CLEANUP
    waitFunc := func() error {
        execErr := execCmd.Wait()
        
        // Always clean up container
        cleanupCmd := exec.Command("docker", "rm", "-f", containerName)
        cleanupCmd.Run()
        
        return execErr
    }
    
    return stdinPipe, stdoutPipe, stderrPipe, waitFunc, nil
}
```

## Critical Implementation Details

### 1. Pipe Creation Order
Always create pipes in this order and clean up on errors:
```go
stdin, err := cmd.StdinPipe()
if err != nil { return err }

stdout, err := cmd.StdoutPipe()
if err != nil {
    stdin.Close()  // Clean up stdin
    return err
}

stderr, err := cmd.StderrPipe()
if err != nil {
    stdin.Close()   // Clean up both
    stdout.Close()
    return err
}
```

### 2. Resource Cleanup
The `wait()` function MUST clean up:
- Temporary files (profiles, scripts)
- Docker containers
- Any other resources created

### 3. Context Handling
- Check `ctx.Done()` before starting
- Use `exec.CommandContext()` for automatic cancellation
- Context cancellation kills the process

### 4. Error Messages
- Include context in errors: `"failed to create stdin pipe: " + err.Error()`
- Log at Debug level for internal operations
- Return descriptive errors to caller

## Testing RunWithPipes

Required test cases:
1. Basic echo (cat command)
2. Multiple writes to stdin
3. Stderr capture
4. Context cancellation
5. Command not found
6. Environment variables
7. Command exits early
8. Concurrent read/write

Example test structure:
```go
func TestExecRunner_RunWithPipes_BasicEcho(t *testing.T) {
    runner, _ := NewExec(Options{}, logger)
    stdin, stdout, stderr, wait, err := runner.RunWithPipes(ctx, "cat", nil, nil, nil)
    require.NoError(t, err)
    
    fmt.Fprintln(stdin, "test")
    stdin.Close()
    
    output, _ := io.ReadAll(stdout)
    io.ReadAll(stderr)
    
    err = wait()
    require.NoError(t, err)
    assert.Equal(t, "test\n", string(output))
}
```

