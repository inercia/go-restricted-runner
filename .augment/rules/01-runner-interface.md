# Runner Interface Rules

**Trigger**: If the user prompt mentions "runner", "interface", "Run", "RunWithPipes", or discusses adding new runner methods.

## Runner Interface Design

The `Runner` interface is the core abstraction in `pkg/runner/runner.go`. All runner implementations must implement this interface completely.

### Current Interface Methods

1. **Run()** - Execute command and return output as string
   - For one-shot commands that complete and return
   - Captures stdout/stderr and returns combined output
   - Uses shell for command execution (supports pipes, redirects, etc.)

2. **RunWithPipes()** - Execute command with interactive I/O
   - For long-running or interactive processes
   - Returns separate stdin/stdout/stderr pipes
   - Direct command execution (no shell interpretation)
   - Requires explicit cleanup via wait() function

3. **CheckImplicitRequirements()** - Verify runner prerequisites
   - Check OS compatibility
   - Verify required executables exist
   - Called automatically by `New()` factory

## Adding New Methods to Runner Interface

When adding a new method to the `Runner` interface:

### 1. Update the Interface
```go
// In pkg/runner/runner.go
type Runner interface {
    // ... existing methods ...
    
    // NewMethod does something useful
    // 
    // Comprehensive documentation here explaining:
    // - What the method does
    // - When to use it vs other methods
    // - Lifecycle and cleanup requirements
    // - All parameters and return values
    NewMethod(ctx context.Context, ...) (result, error)
}
```

### 2. Implement in ALL Runners
You MUST implement the new method in:
- `pkg/runner/exec.go` - ExecRunner
- `pkg/runner/sandbox.go` - SandboxExec
- `pkg/runner/firejail.go` - Firejail
- `pkg/runner/docker.go` - Docker

### 3. Implementation Patterns

**For ExecRunner** (simplest, baseline implementation):
```go
func (r *Exec) NewMethod(ctx context.Context, ...) (result, error) {
    // Check context
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }
    
    r.logger.Debug("NewMethod: starting...")
    
    // Implementation using exec.CommandContext
    cmd := exec.CommandContext(ctx, ...)
    
    // Return result
    return result, nil
}
```

**For SandboxExec/Firejail** (add isolation):
```go
func (r *SandboxExec) NewMethod(ctx context.Context, ...) (result, error) {
    // Process template variables in restrictions
    if len(r.options.AllowReadFolders) > 0 {
        r.options.AllowReadFolders = common.ProcessTemplateListFlexible(
            r.options.AllowReadFolders, params)
    }
    
    // Generate sandbox/firejail profile
    var profileBuf bytes.Buffer
    if err := r.profileTpl.Execute(&profileBuf, r.options); err != nil {
        return nil, fmt.Errorf("failed to render profile: %w", err)
    }
    
    // Create temporary profile file
    profileFile, err := os.CreateTemp("", "sandbox-profile-*.sb")
    // ... write profile, execute with sandbox-exec/firejail
    
    // Clean up profile in defer or wait function
}
```

**For Docker** (container-based):
```go
func (r *Docker) NewMethod(ctx context.Context, ...) (result, error) {
    // Option 1: Use docker run with script file
    // Option 2: Create long-running container + docker exec
    // Option 3: Delegate to ExecRunner with docker command
    
    // Apply Docker restrictions (network, mounts, resources)
    // Clean up containers/volumes when done
}
```

### 4. Testing Requirements

Create tests in `pkg/runner/runner_*_test.go`:

```go
func TestExecRunner_NewMethod(t *testing.T) {
    logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)
    runner, err := NewExec(Options{}, logger)
    require.NoError(t, err)
    
    ctx := context.Background()
    
    // Test basic functionality
    result, err := runner.NewMethod(ctx, ...)
    require.NoError(t, err)
    assert.Equal(t, expected, result)
}

func TestExecRunner_NewMethod_ContextCancellation(t *testing.T) {
    // Test context cancellation
}

func TestExecRunner_NewMethod_ErrorHandling(t *testing.T) {
    // Test error cases
}
```

### 5. Documentation Updates

- Update `README.md` with usage examples
- Add method to API Reference section
- Explain when to use new method vs existing methods
- Document any new options or configuration

## Run() vs RunWithPipes() Decision Guide

**Use Run() when:**
- Command completes quickly
- You need the full output after completion
- Shell features needed (pipes, redirects, variables)
- Simple one-shot execution

**Use RunWithPipes() when:**
- Interactive process (REPL, shell)
- Streaming data to/from process
- Long-running process
- Need separate stdout/stderr
- Real-time output processing

