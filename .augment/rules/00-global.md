# Global Rules for go-restricted-runner

**Trigger**: Always included for all prompts.

## Project Overview

`go-restricted-runner` is a Go library for executing commands in isolated/restricted environments. It provides a unified `Runner` interface with multiple backend implementations for different isolation levels.

### Core Architecture

- **Package Structure**:
  - `pkg/runner/` - Runner interface and implementations (exec, sandbox-exec, firejail, docker)
  - `pkg/common/` - Shared utilities (logging, templates, prerequisites)
  - `docs/` - Per-runner documentation
  - `examples/` - Usage examples

- **Runner Types**:
  - `TypeExec` - Direct execution (no isolation)
  - `TypeSandboxExec` - macOS sandbox-exec (medium isolation)
  - `TypeFirejail` - Linux firejail (medium isolation)
  - `TypeDocker` - Docker containers (high isolation)

### Key Interfaces

```go
type Runner interface {
    Run(ctx, shell, command, env, params, tmpfile) (string, error)
    RunWithPipes(ctx, cmd, args, env, params) (stdin, stdout, stderr, wait, error)
    CheckImplicitRequirements() error
}
```

## Development Principles

### 1. Interface Consistency
- All runners MUST implement the complete `Runner` interface
- Method signatures must be identical across all implementations
- Error handling patterns should be consistent

### 2. Platform-Specific Code
- Use build tags or runtime checks for platform-specific features
- SandboxExec is macOS-only (darwin)
- Firejail is Linux-only
- Docker and Exec work on all platforms

### 3. Testing Requirements
- Every new method MUST have comprehensive tests
- Test both success and failure cases
- Include platform-specific test skipping where appropriate
- Use table-driven tests for multiple scenarios

### 4. Documentation Standards
- All public functions/methods require godoc comments
- Include parameter descriptions and return value explanations
- Document lifecycle and cleanup requirements
- Provide usage examples in README.md

### 5. Error Handling
- Return descriptive errors with context
- Use `fmt.Errorf` with `%w` for error wrapping
- Log errors at appropriate levels (Debug for internal, Error for user-facing)
- Clean up resources even when errors occur

### 6. Resource Management
- Always clean up temporary files (profiles, scripts)
- Close file handles and pipes properly
- Implement cleanup in defer blocks or wait() functions
- Handle cleanup errors gracefully (log but don't fail)

## Code Style

### Naming Conventions
- Interfaces: `Runner`, `Logger`
- Implementations: `Exec`, `Docker`, `SandboxExec`, `Firejail`
- Options structs: `ExecOptions`, `DockerOptions`, etc.
- Factory functions: `New()`, `NewExec()`, `NewDocker()`, etc.

### Import Organization
```go
import (
    // Standard library
    "context"
    "fmt"
    
    // Third-party
    "github.com/external/package"
    
    // Internal
    "github.com/inercia/go-restricted-runner/pkg/common"
)
```

### Logging
- Use structured logging via `common.Logger`
- Debug level for internal operations
- Info level for user-visible actions
- Error level for failures
- Always pass logger through constructors

## Build and Test Commands

```bash
make test              # Run all tests
make test-race         # Run with race detection
make test-coverage     # Generate coverage report
make lint              # Run golangci-lint
make format            # Format code and tidy modules
```

## File Organization

- Keep related functionality together
- Separate platform-specific code into `_unix.go` and `_windows.go` files
- Use `//go:embed` for template files
- Place tests in `*_test.go` files in the same package

