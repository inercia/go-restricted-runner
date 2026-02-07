# Common Utilities Package

**Trigger**: If the user prompt mentions "common", "logger", "logging", "template", "prerequisites", or utility functions.

## Package Overview

The `pkg/common/` package provides shared utilities used across all runners:
- `logging.go` - Structured logging
- `templates.go` - Template processing with Sprig functions
- `prerequisites.go` - Executable and requirement checking

## Logging (logging.go)

### Logger Structure
```go
type Logger struct {
    *log.Logger          // Embedded standard logger
    level    LogLevel    // Current log level
    filePath string      // Log file path (if any)
    file     *os.File    // Log file handle (if any)
}
```

### Log Levels
```go
const (
    LogLevelNone  LogLevel = iota  // No logging
    LogLevelError                   // Errors only
    LogLevelInfo                    // Info and errors
    LogLevelDebug                   // All messages
)
```

### Creating a Logger
```go
// Stderr only
logger, err := common.NewLogger("", "", common.LogLevelInfo, false)

// With file logging
logger, err := common.NewLogger(
    "[prefix] ",           // Prefix for all messages
    "/path/to/log.txt",   // Log file path
    common.LogLevelDebug, // Log level
    false,                // Append (true = truncate)
)

// Always close when done
defer logger.Close()
```

### Logging Methods
```go
logger.Debug("Debug message: %s", value)  // Only if level >= Debug
logger.Info("Info message: %s", value)    // Only if level >= Info
logger.Warn("Warning: %s", value)         // Only if level >= Info
logger.Error("Error: %s", value)          // Only if level >= Error
```

### Global Logger
```go
// Get global logger (creates default if not set)
logger := common.GetLogger()

// Set global logger
common.SetLogger(myLogger)
```

### Logger in Runners
All runner constructors accept an optional logger:
```go
func NewExec(options Options, logger *common.Logger) (*Exec, error) {
    if logger == nil {
        logger = common.GetLogger()  // Use global if not provided
    }
    return &Exec{logger: logger, ...}, nil
}
```

## Template Processing (templates.go)

### ProcessTemplate Function
Processes Go templates with Sprig functions:

```go
func ProcessTemplate(text string, args map[string]interface{}) (string, error)
```

**Example**:
```go
template := "Hello {{.name}}, you are {{.age}} years old"
args := map[string]interface{}{
    "name": "Alice",
    "age":  30,
}
result, err := common.ProcessTemplate(template, args)
// result: "Hello Alice, you are 30 years old"
```

### ProcessTemplateListFlexible Function
Processes a list of templates, falling back to original on error:

```go
func ProcessTemplateListFlexible(list []string, args map[string]interface{}) []string
```

**Example**:
```go
list := []string{
    "{{.workdir}}/src",
    "/tmp",
    "{{.invalid",  // Invalid template
}
args := map[string]interface{}{"workdir": "/home/user"}

result := common.ProcessTemplateListFlexible(list, args)
// result: ["/home/user/src", "/tmp", "{{.invalid"]
```

### Sprig Functions
The template engine includes all Sprig functions:
- String functions: `upper`, `lower`, `trim`, `replace`
- Math functions: `add`, `sub`, `mul`, `div`
- Date functions: `now`, `date`
- Path functions: `base`, `dir`, `ext`
- And many more: https://masterminds.github.io/sprig/

**Example**:
```go
template := "{{.path | base}}"
args := map[string]interface{}{"path": "/home/user/file.txt"}
result, _ := common.ProcessTemplate(template, args)
// result: "file.txt"
```

### Missing Key Handling
Templates use `missingkey=zero` option:
```go
template := "Value: {{.missing}}"
result, _ := common.ProcessTemplate(template, map[string]interface{}{})
// result: "Value: " (not an error)
```

The code also removes `<no value>` strings:
```go
// After template execution
res = strings.ReplaceAll(res, "<no value>", "")
```

## Prerequisites Checking (prerequisites.go)

### CheckExecutableExists
Checks if an executable exists in PATH:

```go
func CheckExecutableExists(name string) bool
```

**Example**:
```go
if !common.CheckExecutableExists("docker") {
    return fmt.Errorf("docker not found in PATH")
}
```

**Implementation**:
```go
func CheckExecutableExists(name string) bool {
    _, err := exec.LookPath(name)
    return err == nil
}
```

### Usage in Runners
```go
func (r *Docker) CheckImplicitRequirements() error {
    if !common.CheckExecutableExists("docker") {
        return fmt.Errorf("docker executable not found in PATH")
    }
    
    // Check if Docker daemon is running
    cmd := exec.Command("docker", "info")
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("docker daemon is not running: %w", err)
    }
    
    return nil
}
```

## Utility Functions (pkg/runner/util.go)

### isSingleExecutableCommand
Checks if a command is a single executable (optimization):

```go
func isSingleExecutableCommand(command string) bool
```

**Checks**:
1. No spaces or shell metacharacters
2. Executable exists (absolute/relative path or in PATH)
3. Not a directory
4. Has execute permissions

**Example**:
```go
isSingleExecutableCommand("ls")           // true
isSingleExecutableCommand("ls -la")       // false (has space)
isSingleExecutableCommand("echo hello")   // false (has space)
isSingleExecutableCommand("/bin/cat")     // true (if exists)
```

### contains
Checks if a string slice contains a value:

```go
func contains(slice []string, item string) bool
```

### shellQuote
Returns a shell-safe quoted string:

```go
func shellQuote(s string) string
```

**Example**:
```go
shellQuote("hello")           // "hello" (no special chars)
shellQuote("hello world")     // "'hello world'"
shellQuote("it's")            // "'it'"'"'s'" (escapes single quote)
```

## Best Practices

### 1. Always Use Logger
```go
// BAD
fmt.Println("Debug info")

// GOOD
logger.Debug("Debug info: %v", value)
```

### 2. Template Error Handling
```go
// For critical templates
result, err := common.ProcessTemplate(template, args)
if err != nil {
    return fmt.Errorf("template processing failed: %w", err)
}

// For optional templates (restrictions)
result := common.ProcessTemplateListFlexible(list, args)
// Falls back to original on error
```

### 3. Check Prerequisites Early
```go
func NewRunner(...) (*Runner, error) {
    runner := &Runner{...}
    
    // Check requirements immediately
    if err := runner.CheckImplicitRequirements(); err != nil {
        return nil, err
    }
    
    return runner, nil
}
```

### 4. Log at Appropriate Levels
```go
logger.Debug("Internal operation: %v", details)  // Implementation details
logger.Info("Starting operation")                // User-visible actions
logger.Warn("Deprecated feature used")           // Warnings
logger.Error("Operation failed: %v", err)        // Errors
```

## Common Patterns

### Logger Initialization in Tests
```go
func TestSomething(t *testing.T) {
    logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)
    runner, err := NewExec(Options{}, logger)
    // ... test code
}
```

### Template Processing in Runners
```go
// Process all restriction lists
if len(r.options.AllowReadFolders) > 0 {
    r.options.AllowReadFolders = common.ProcessTemplateListFlexible(
        r.options.AllowReadFolders, params)
}
if len(r.options.AllowWriteFolders) > 0 {
    r.options.AllowWriteFolders = common.ProcessTemplateListFlexible(
        r.options.AllowWriteFolders, params)
}
```

