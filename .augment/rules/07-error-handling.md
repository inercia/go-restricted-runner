# Error Handling Patterns

**Trigger**: If the user prompt mentions "error", "error handling", "cleanup", "defer", or discusses failure cases.

## Error Handling Principles

### 1. Descriptive Errors
Always provide context in error messages:

```go
// BAD
return err

// GOOD
return fmt.Errorf("failed to create stdin pipe: %w", err)
```

### 2. Error Wrapping
Use `%w` to wrap errors for error chain inspection:

```go
if err := doSomething(); err != nil {
    return fmt.Errorf("operation failed: %w", err)
}

// Allows callers to use errors.Is() and errors.As()
```

### 3. Error Logging
Log errors at appropriate levels:

```go
// Internal errors (Debug)
r.logger.Debug("Failed to optimize command: %v", err)

// User-facing errors (Error)
r.logger.Error("Command execution failed: %v", err)
```

## Common Error Patterns

### Context Cancellation
Always check context before starting operations:

```go
func (r *Exec) Run(ctx context.Context, ...) (string, error) {
    // Check if context is already done
    select {
    case <-ctx.Done():
        return "", ctx.Err()
    default:
        // Continue execution
    }
    
    // Use CommandContext for automatic cancellation
    cmd := exec.CommandContext(ctx, ...)
}
```

### Pipe Creation Errors
Clean up already-created resources on error **and check cleanup errors**:

```go
stdinPipe, err := cmd.StdinPipe()
if err != nil {
    return nil, nil, nil, nil, fmt.Errorf("failed to create stdin pipe: %w", err)
}

stdoutPipe, err := cmd.StdoutPipe()
if err != nil {
    // IMPORTANT: Check error from Close()
    if closeErr := stdinPipe.Close(); closeErr != nil {
        r.logger.Debug("Warning: failed to close stdin pipe: %v", closeErr)
    }
    return nil, nil, nil, nil, fmt.Errorf("failed to create stdout pipe: %w", err)
}

stderrPipe, err := cmd.StderrPipe()
if err != nil {
    // Clean up both pipes, checking errors
    if closeErr := stdinPipe.Close(); closeErr != nil {
        r.logger.Debug("Warning: failed to close stdin pipe: %v", closeErr)
    }
    if closeErr := stdoutPipe.Close(); closeErr != nil {
        r.logger.Debug("Warning: failed to close stdout pipe: %v", closeErr)
    }
    return nil, nil, nil, nil, fmt.Errorf("failed to create stderr pipe: %w", err)
}
```

**Why check Close() errors?**
- Linters (errcheck) require all error returns to be checked
- Close() can fail (e.g., broken pipe, resource exhaustion)
- Logging warnings helps with debugging
- Don't fail the operation on cleanup errors - just log them

### Command Start Errors
Clean up all resources if start fails **with proper error checking**:

```go
if err := cmd.Start(); err != nil {
    // Clean up all pipes, checking errors
    if closeErr := stdinPipe.Close(); closeErr != nil {
        r.logger.Debug("Warning: failed to close stdin pipe: %v", closeErr)
    }
    if closeErr := stdoutPipe.Close(); closeErr != nil {
        r.logger.Debug("Warning: failed to close stdout pipe: %v", closeErr)
    }
    if closeErr := stderrPipe.Close(); closeErr != nil {
        r.logger.Debug("Warning: failed to close stderr pipe: %v", closeErr)
    }

    // Also clean up temp files, containers, etc.
    if removeErr := os.Remove(tempFile); removeErr != nil {
        r.logger.Debug("Warning: failed to remove temp file: %v", removeErr)
    }

    return nil, nil, nil, nil, fmt.Errorf("failed to start command: %w", err)
}
```

### Stderr Handling
Include stderr in error messages when available:

```go
var stdout, stderr bytes.Buffer
cmd.Stdout = &stdout
cmd.Stderr = &stderr

if err := cmd.Run(); err != nil {
    if stderr.Len() > 0 {
        errMsg := strings.TrimSpace(stderr.String())
        return "", errors.New(errMsg)
    }
    return "", err
}
```

## Resource Cleanup Patterns

### Using defer
For synchronous operations (Run method):

```go
func (r *Exec) Run(...) (string, error) {
    // Create temporary file
    tmpFile, err := os.CreateTemp("", "script-*.sh")
    if err != nil {
        return "", err
    }
    defer os.Remove(tmpFile.Name())  // Always clean up
    
    // ... use tmpFile
    
    return output, nil
}
```

### Using wait() Function
For asynchronous operations (RunWithPipes):

```go
func (r *Exec) RunWithPipes(...) (...) {
    // Create resources
    profileFile, _ := os.CreateTemp("", "profile-*.sb")
    
    // Start command
    cmd.Start()
    
    // Clean up in wait function
    waitFunc := func() error {
        err := cmd.Wait()
        
        // Always clean up, even on error
        if removeErr := os.Remove(profileFile.Name()); removeErr != nil {
            r.logger.Debug("Warning: failed to remove file: %v", removeErr)
        }
        
        return err
    }
    
    return stdin, stdout, stderr, waitFunc, nil
}
```

### Docker Container Cleanup
Always clean up containers, even on errors **with proper error checking**:

```go
// Create container
containerName := "go-restricted-runner-123"
createCmd.Run()

// On any error after creation - CHECK cleanup errors
if err != nil {
    cleanupCmd := exec.Command("docker", "rm", "-f", containerName)
    if cleanupErr := cleanupCmd.Run(); cleanupErr != nil {
        r.logger.Debug("Warning: failed to cleanup container during error handling: %v", cleanupErr)
    }
    return nil, nil, nil, nil, err
}

// In wait function - CHECK cleanup errors
waitFunc := func() error {
    execErr := cmd.Wait()

    // Always clean up container
    cleanupCmd := exec.Command("docker", "rm", "-f", containerName)
    if cleanupOutput, cleanupErr := cleanupCmd.CombinedOutput(); cleanupErr != nil {
        r.logger.Debug("Warning: failed to remove container: %v, output: %s",
            cleanupErr, string(cleanupOutput))
    }

    return execErr  // Return original error, not cleanup error
}
```

**Critical Pattern**: Always check cleanup errors but don't let them override the original error.

## Error Types and Handling

### exec.ExitError
Handle command exit errors:

```go
if err := cmd.Run(); err != nil {
    if exitErr, ok := err.(*exec.ExitError); ok {
        // Command ran but exited with non-zero status
        exitCode := exitErr.ExitCode()
        r.logger.Debug("Command exited with code %d", exitCode)
    }
    return "", err
}
```

### Context Errors
Distinguish between cancellation and timeout:

```go
if err := cmd.Wait(); err != nil {
    if errors.Is(err, context.Canceled) {
        r.logger.Debug("Command cancelled by context")
    } else if errors.Is(err, context.DeadlineExceeded) {
        r.logger.Debug("Command timed out")
    }
    return err
}
```

### Template Errors
Handle template processing errors:

```go
// Critical templates - fail on error
result, err := common.ProcessTemplate(template, args)
if err != nil {
    return fmt.Errorf("failed to process template: %w", err)
}

// Optional templates - use flexible processing
result := common.ProcessTemplateListFlexible(list, args)
// Falls back to original on error, no error returned
```

## Logging Errors

### Debug Level
For internal operations and diagnostics:

```go
r.logger.Debug("Failed to create temporary file: %v", err)
r.logger.Debug("Command failed with stderr: %s", stderr.String())
r.logger.Debug("Warning: failed to remove temporary file: %v", err)
```

### Error Level
For user-facing errors:

```go
r.logger.Error("Command execution failed: %v", err)
r.logger.Error("Failed to start process: %v", err)
```

### Cleanup Warnings
Log cleanup failures at Debug level (don't fail the operation):

```go
if err := os.Remove(tempFile); err != nil {
    r.logger.Debug("Warning: failed to remove temporary file %s: %v", tempFile, err)
}
```

## Testing Error Cases

### Test Error Conditions
```go
func TestExecRunner_RunWithPipes_CommandNotFound(t *testing.T) {
    runner, _ := NewExec(Options{}, logger)
    
    _, _, _, _, err := runner.RunWithPipes(ctx, "nonexistent-command", nil, nil, nil)
    if err == nil {
        t.Error("Expected error for non-existent command")
    }
}
```

### Test Context Cancellation
```go
func TestExecRunner_RunWithPipes_ContextCancellation(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    
    stdin, stdout, stderr, wait, err := runner.RunWithPipes(ctx, "sleep", []string{"60"}, nil, nil)
    require.NoError(t, err)
    
    // Cancel after short delay
    time.Sleep(100 * time.Millisecond)
    cancel()
    
    // Clean up
    stdin.Close()
    io.ReadAll(stdout)
    io.ReadAll(stderr)
    
    // Wait should return error
    err = wait()
    if err == nil {
        t.Error("Expected error after context cancellation")
    }
}
```

## Anti-Patterns to Avoid

### ❌ Ignoring Cleanup Errors Silently
```go
// BAD
os.Remove(tempFile)  // Error ignored

// GOOD
if err := os.Remove(tempFile); err != nil {
    r.logger.Debug("Warning: failed to remove file: %v", err)
}
```

### ❌ Not Cleaning Up on Errors
```go
// BAD
stdinPipe, _ := cmd.StdinPipe()
stdoutPipe, err := cmd.StdoutPipe()
if err != nil {
    return err  // stdin pipe leaked!
}

// GOOD
stdinPipe, _ := cmd.StdinPipe()
stdoutPipe, err := cmd.StdoutPipe()
if err != nil {
    stdinPipe.Close()  // Clean up
    return err
}
```

### ❌ Returning Cleanup Errors
```go
// BAD
waitFunc := func() error {
    cmd.Wait()
    return os.Remove(tempFile)  // Returns cleanup error, not command error!
}

// GOOD
waitFunc := func() error {
    err := cmd.Wait()
    os.Remove(tempFile)  // Ignore cleanup error
    return err           // Return command error
}
```

### ❌ Generic Error Messages
```go
// BAD
return errors.New("failed")

// GOOD
return fmt.Errorf("failed to create stdin pipe: %w", err)
```

