# Documentation Standards

**Trigger**: If the user prompt mentions "documentation", "godoc", "README", "comments", or "examples".

## Godoc Comments

### Package Documentation
Every package should have a package comment:

```go
// Package runner provides isolated command execution environments.
//
// This package defines the Runner interface and implementations for executing
// commands in various isolation environments including direct execution,
// firejail (Linux), sandbox-exec (macOS), and Docker containers.
package runner
```

### Function/Method Documentation
All exported functions and methods require documentation:

```go
// Run executes a command and returns the output.
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - shell: The shell to use for execution (empty for default)
//   - command: The command to execute
//   - env: Environment variables in KEY=VALUE format
//   - params: Template parameters for variable substitution
//   - tmpfile: Whether to use a temporary file for the command
//
// Returns:
//   - The command output as a string
//   - An error if execution fails
func (r *Exec) Run(ctx context.Context, shell string, command string, 
    env []string, params map[string]interface{}, tmpfile bool) (string, error) {
    // Implementation
}
```

### Struct Documentation
Document structs and their fields:

```go
// DockerOptions represents configuration options for the Docker runner.
type DockerOptions struct {
    // The Docker image to use (required)
    Image string `json:"image"`
    
    // Whether to allow networking in the container
    AllowNetworking bool `json:"allow_networking"`
    
    // Memory limit (e.g. "512m", "1g")
    Memory string `json:"memory"`
}
```

### Interface Documentation
Document interfaces and their contracts:

```go
// Runner is an interface for running commands in isolated environments
type Runner interface {
    // Run executes a command and returns the output.
    Run(ctx context.Context, ...) (string, error)
    
    // RunWithPipes executes a command with access to stdin/stdout/stderr pipes.
    //
    // Lifecycle:
    //   1. Call RunWithPipes to start the process and get pipes
    //   2. Write data to stdin as needed
    //   3. Read from stdout/stderr as needed
    //   4. Close stdin when done writing
    //   5. Call wait() to wait for completion
    RunWithPipes(ctx context.Context, ...) (stdin, stdout, stderr, wait, error)
}
```

## README.md Structure

### Main README.md
Should include:
1. Project title and description
2. Installation instructions
3. Quick start example
4. Runner types overview
5. API reference
6. Links to detailed documentation
7. Development instructions
8. License

### Example Structure
```markdown
# go-restricted-runner

Brief description

## Installation
```bash
go get github.com/inercia/go-restricted-runner
```

## Quick Start
```go
// Simple example
```

## Runner Types
- Exec - Direct execution
- SandboxExec - macOS isolation
- Firejail - Linux isolation
- Docker - Container isolation

## Interactive Process Communication
Explain RunWithPipes with examples

## API Reference
Interface definitions

## Development
Build and test commands
```

## Per-Runner Documentation

Each runner should have detailed documentation in `docs/`:
- `docs/runner-exec.md`
- `docs/runner-sandbox-exec.md`
- `docs/runner-firejail.md`
- `docs/runner-docker.md`

### Runner Doc Structure
```markdown
# Runner Name

## Overview
What it does, when to use it

## Platform Requirements
OS, executables, etc.

## Isolation Level
What's isolated, what's not

## API Usage

### Basic Usage
Simple example

### Options
Table of all options

### With Environment Variables
Example

### With Restrictions
Example

## Limitations
What doesn't work

## Troubleshooting
Common issues
```

## Code Examples

### Inline Examples
Include examples in godoc comments:

```go
// ProcessTemplate processes a template with the given arguments.
//
// Example:
//   template := "Hello {{.name}}"
//   args := map[string]interface{}{"name": "World"}
//   result, err := ProcessTemplate(template, args)
//   // result: "Hello World"
func ProcessTemplate(text string, args map[string]interface{}) (string, error) {
    // Implementation
}
```

### Example Files
Create runnable examples in `examples/`:

```go
// examples/pipes_example.go
package main

import (
    "context"
    "fmt"
    "io"
    "log"
    
    "github.com/inercia/go-restricted-runner/pkg/common"
    "github.com/inercia/go-restricted-runner/pkg/runner"
)

func main() {
    // Create logger
    logger, _ := common.NewLogger("", "", common.LogLevelInfo, false)
    
    // Create runner
    r, err := runner.New(runner.TypeExec, runner.Options{}, logger)
    if err != nil {
        log.Fatal(err)
    }
    
    // Use RunWithPipes
    ctx := context.Background()
    stdin, stdout, stderr, wait, err := r.RunWithPipes(ctx, "cat", nil, nil, nil)
    if err != nil {
        log.Fatal(err)
    }
    
    // Send input
    fmt.Fprintln(stdin, "Hello from RunWithPipes!")
    stdin.Close()
    
    // Read output
    output, _ := io.ReadAll(stdout)
    io.ReadAll(stderr)
    wait()
    
    fmt.Print(string(output))
}
```

## Documentation for New Features

When adding a new feature (like RunWithPipes), update:

### 1. Godoc Comments
Add comprehensive documentation to the interface and all implementations

### 2. README.md
Add a new section explaining:
- What the feature does
- When to use it vs existing features
- Basic usage example
- Advanced usage example
- Important notes and gotchas

### 3. Per-Runner Docs
Update each runner's documentation with:
- How the feature works for that runner
- Runner-specific considerations
- Examples

### 4. Examples
Create a runnable example demonstrating the feature

## Documentation Best Practices

### 1. Be Specific
```go
// BAD
// Run runs a command
func Run(...) (string, error)

// GOOD
// Run executes a command with the given shell and returns the output.
// It implements the Runner interface.
//
// Note: For Windows native shells (cmd, powershell), the 'tmpfile' 
// parameter is ignored and commands are executed directly.
func Run(...) (string, error)
```

### 2. Document Lifecycle
For complex operations, document the lifecycle:

```go
// RunWithPipes executes a command with access to stdin/stdout/stderr pipes.
//
// Lifecycle:
//   1. Call RunWithPipes to start the process and get pipes
//   2. Write data to stdin as needed
//   3. Read from stdout/stderr as needed (can be done concurrently)
//   4. Close stdin when done writing to signal EOF to the process
//   5. Read any remaining output from stdout/stderr
//   6. Call wait() to wait for process completion and get exit status
```

### 3. Document Cleanup Requirements
```go
// Important notes:
//   - The wait() function MUST be called to properly clean up process resources
//   - Reading from stdout/stderr after the process exits is safe
//   - If context is cancelled, wait() will return context.Canceled
```

### 4. Provide Examples
Include examples for non-obvious usage:

```go
// Example: Interactive Python REPL
//   stdin, stdout, stderr, wait, err := r.RunWithPipes(
//       ctx, "python3", []string{"-i"}, nil, nil)
//   fmt.Fprintln(stdin, "print('Hello')")
//   stdin.Close()
//   output, _ := io.ReadAll(stdout)
//   wait()
```

### 5. Document Platform Differences
```go
// Note: For Windows native shells (cmd, powershell), the 'tmpfile' 
// parameter is ignored and commands are executed directly to avoid 
// issues with output capturing.
```

### 6. Link to Related Documentation
```go
// See also:
//   - Run() for one-shot command execution
//   - CheckImplicitRequirements() for prerequisite checking
```

## Keeping Documentation Updated

When making changes:
1. Update godoc comments if signatures change
2. Update README.md if behavior changes
3. Update examples if they break
4. Add new examples for new features
5. Update per-runner docs if runner-specific behavior changes

## Documentation Review Checklist

Before submitting changes:
- [ ] All exported functions have godoc comments
- [ ] Comments explain what, why, and how
- [ ] Examples are included for complex features
- [ ] README.md is updated
- [ ] Per-runner docs are updated if needed
- [ ] Examples compile and run correctly
- [ ] Platform-specific notes are included
- [ ] Lifecycle and cleanup are documented

