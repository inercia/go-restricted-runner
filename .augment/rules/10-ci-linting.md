# CI and Linting Best Practices

**Trigger**: If the user prompt mentions "CI", "lint", "linter", "errcheck", "golangci-lint", "GitHub Actions", or discusses code quality checks.

## CI Configuration

### GitHub Actions Workflow

The project uses GitHub Actions with three main jobs:
- **Test**: Runs tests on Ubuntu, macOS, and Windows
- **Lint**: Runs golangci-lint with errcheck and other linters
- **Format**: Checks code formatting with gofmt

### Go Version Management

**Always use stable Go versions in CI:**

```yaml
# .github/workflows/ci.yml
strategy:
  matrix:
    os: [ubuntu-latest, macos-latest, windows-latest]
    go-version: ['1.23']  # Use stable version, not unreleased
```

**Match go.mod with CI:**
```go
// go.mod
go 1.23  // Must match CI configuration
```

**Common Issue**: Using unreleased Go versions (e.g., 1.25) causes CI failures.

## Linting Rules

### errcheck - Unchecked Error Returns

The most common linting violation in this codebase is **unchecked error returns**.

**Rule**: Every function that returns an error MUST have its error checked.

### Critical Patterns to Check

#### 1. Close() Operations
```go
// ❌ BAD - Linter error
stdin.Close()

// ✅ GOOD
if err := stdin.Close(); err != nil {
    r.logger.Debug("Warning: failed to close stdin: %v", err)
}
```

#### 2. I/O Operations
```go
// ❌ BAD - Linter error
io.ReadAll(stderr)
fmt.Fprintln(stdin, "data")

// ✅ GOOD
if _, err := io.ReadAll(stderr); err != nil {
    r.logger.Debug("Warning: failed to read stderr: %v", err)
}

if _, err := fmt.Fprintln(stdin, "data"); err != nil {
    log.Printf("Warning: failed to write: %v", err)
}
```

#### 3. Cleanup Operations
```go
// ❌ BAD - Linter error
os.Remove(tempFile)
cleanupCmd.Run()

// ✅ GOOD
if err := os.Remove(tempFile); err != nil {
    r.logger.Debug("Warning: failed to remove file: %v", err)
}

if err := cleanupCmd.Run(); err != nil {
    r.logger.Debug("Warning: cleanup failed: %v", err)
}
```

#### 4. Wait Functions
```go
// ❌ BAD - Linter error in tests
wait()

// ✅ GOOD
if err := wait(); err != nil {
    t.Logf("Note: wait returned error: %v", err)
}
```

## Error Checking Strategy

### When to Log vs Fail

**Log as Warning** (don't fail the operation):
- Cleanup operations (Close, Remove, docker rm)
- Non-critical I/O in error paths
- Resource deallocation

**Return the Error** (fail the operation):
- Resource creation (StdinPipe, StdoutPipe, StderrPipe)
- Command start failures
- Critical operations

### Pattern for Cleanup in Error Paths

```go
stdinPipe, err := cmd.StdinPipe()
if err != nil {
    return nil, nil, nil, nil, fmt.Errorf("failed to create stdin pipe: %w", err)
}

stdoutPipe, err := cmd.StdoutPipe()
if err != nil {
    // Cleanup: check error but don't fail on cleanup failure
    if closeErr := stdinPipe.Close(); closeErr != nil {
        r.logger.Debug("Warning: failed to close stdin pipe: %v", closeErr)
    }
    // Return the original error
    return nil, nil, nil, nil, fmt.Errorf("failed to create stdout pipe: %w", err)
}
```

## Testing Code Quality

### Error Checking in Tests

Tests should also check errors for code quality:

```go
func TestSomething(t *testing.T) {
    stdin, stdout, stderr, wait, err := runner.RunWithPipes(...)
    require.NoError(t, err)
    
    // ❌ BAD
    stdin.Close()
    io.ReadAll(stdout)
    wait()
    
    // ✅ GOOD
    if err := stdin.Close(); err != nil {
        t.Logf("Warning: failed to close stdin: %v", err)
    }
    
    if _, err := io.ReadAll(stdout); err != nil {
        t.Logf("Warning: failed to read stdout: %v", err)
    }
    
    if err := wait(); err != nil {
        t.Logf("Note: wait returned error: %v", err)
    }
}
```

### Example Code Quality

Example code should demonstrate best practices:

```go
// examples/pipes_example.go
if _, err := fmt.Fprintln(stdin, "Hello"); err != nil {
    log.Printf("Warning: failed to write: %v", err)
}

if err := stdin.Close(); err != nil {
    log.Printf("Warning: failed to close stdin: %v", err)
}
```

## Running Linters Locally

```bash
# Run all linters
make lint

# Run specific linter
golangci-lint run

# Auto-fix some issues (use with caution)
golangci-lint run --fix
```

## Common Linting Errors

### 1. errcheck
**Error**: `Error return value of X is not checked`
**Fix**: Add error checking with appropriate handling

### 2. ineffassign
**Error**: `Ineffectual assignment to X`
**Fix**: Remove unused assignments or use the value

### 3. staticcheck
**Error**: Various code quality issues
**Fix**: Follow the specific suggestion from staticcheck

## CI Debugging Workflow

### When CI Fails

1. **Check the CI logs** on GitHub Actions
2. **Identify the failing job** (Test, Lint, or Format)
3. **Run locally** to reproduce:
   ```bash
   make test      # For test failures
   make lint      # For lint failures
   make format    # For format issues
   ```
4. **Fix the issues** following the patterns in this guide
5. **Verify locally** before pushing
6. **Push and monitor** CI

### Viewing CI Logs

```bash
# Using GitHub CLI
gh run list --branch main --limit 5
gh run view <run-id>
gh run view <run-id> --log-failed
```

## Fixing Bulk Linting Errors

When you have many linting errors (like we had with 24 errcheck violations):

### 1. Categorize Errors
Group by file and error type:
- Pipe Close() operations
- I/O operations (ReadAll, Write, Fprintln)
- Cleanup operations (Remove, docker rm)
- Wait functions

### 2. Fix Systematically
Fix one file at a time, one pattern at a time:

```bash
# Fix docker.go pipe errors
# Fix docker.go cleanup errors
# Fix exec.go pipe errors
# ... etc
```

### 3. Verify After Each File
```bash
make lint  # Check progress
go test ./...  # Ensure tests still pass
```

### 4. Commit When Clean
```bash
git add <files>
git commit -m "fix: add error checking for unchecked return values"
```

## Best Practices

### 1. Check Errors Early
Add error checking as you write code, not as an afterthought.

### 2. Use Consistent Patterns
Follow the established patterns in the codebase:
- Log cleanup errors at Debug level
- Return critical errors
- Use descriptive error messages

### 3. Test Locally Before Pushing
```bash
make lint && make test && make format
```

### 4. Keep CI Fast
- Don't add unnecessary dependencies
- Use caching where appropriate
- Run tests in parallel

### 5. Monitor CI Health
- Fix failures quickly
- Don't let broken CI become normal
- Keep the main branch green

## Troubleshooting

### CI Passes Locally But Fails in GitHub Actions

**Possible causes:**
1. Different Go version (check go.mod vs CI config)
2. Platform-specific issues (test on all platforms)
3. Missing dependencies in CI
4. Environment differences

**Solution:**
- Match Go versions exactly
- Use platform-specific test skipping
- Ensure all dependencies are in go.mod

### Linter Finds Issues Not Caught Locally

**Possible causes:**
1. Different linter version
2. Different linter configuration
3. Cached results locally

**Solution:**
```bash
# Clear cache and re-run
golangci-lint cache clean
make lint
```

## Quick Reference

### Error Checking Checklist

When adding new code, check errors for:
- [ ] All Close() operations
- [ ] All I/O operations (Read, Write, ReadAll)
- [ ] All cleanup operations (Remove, docker rm)
- [ ] All wait() or Wait() calls
- [ ] All command execution (Run, Start)
- [ ] All pipe creation (StdinPipe, StdoutPipe, StderrPipe)

### CI Commands

```bash
# Local verification
make test
make lint
make format

# CI status
gh run list --branch main
gh run view <run-id>

# Re-run failed jobs
gh run rerun <run-id> --failed
```

