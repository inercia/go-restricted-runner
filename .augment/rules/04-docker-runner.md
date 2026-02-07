# Docker Runner Implementation

**Trigger**: If the user prompt mentions "docker", "container", "DockerRunner", or "DockerOptions".

## Docker Runner Overview

The Docker runner (`pkg/runner/docker.go`) executes commands inside Docker containers for high isolation.

### DockerOptions Structure

```go
type DockerOptions struct {
    Image              string   // Required: Docker image to use
    DockerRunOpts      string   // Additional docker run options
    Mounts             []string // Mount points "hostpath:containerpath"
    AllowNetworking    bool     // Whether to allow networking
    Network            string   // Specific network (host, bridge, custom)
    User               string   // User to run as inside container
    WorkDir            string   // Working directory inside container
    PrepareCommand     string   // Command to run before main command
    Memory             string   // Memory limit (e.g., "512m", "1g")
    MemoryReservation  string   // Memory soft limit
    MemorySwap         string   // Swap limit
    MemorySwappiness   int      // Memory swappiness (0-100)
    CapAdd             []string // Linux capabilities to add
    CapDrop            []string // Linux capabilities to drop
    DNS                []string // Custom DNS servers
    DNSSearch          []string // Custom DNS search domains
    Platform           string   // Platform (e.g., "linux/amd64")
}
```

## Run() Implementation Pattern

Docker's `Run()` method has two execution paths:

### 1. Single Executable Optimization
```go
if isSingleExecutableCommand(cmd) {
    // Direct execution without temp script
    dockerCmd = r.opts.GetDirectExecutionCommand(cmd, env)
}
```

### 2. Script-Based Execution
```go
else {
    // Create temporary script file
    scriptFile, err := r.createScriptFile(shell, cmd, env)
    defer os.Remove(scriptFile)
    
    // Mount script and execute
    dockerCmd = r.opts.GetDockerCommand(scriptFile, env)
}
```

### 3. Delegate to ExecRunner
```go
// Run the docker command using ExecRunner
execRunner, _ := NewExec(Options{}, r.logger)
output, err := execRunner.Run(ctx, "sh", dockerCmd, nil, params, false)
```

## RunWithPipes() Implementation Pattern

Docker's `RunWithPipes()` uses a different approach:

### 1. Create Long-Running Container
```go
containerName := fmt.Sprintf("go-restricted-runner-%d", time.Now().UnixNano())

dockerRunArgs := []string{"run", "--name", containerName, "-d"}
// Add all restrictions and options
dockerRunArgs = append(dockerRunArgs, r.opts.Image, "sleep", "infinity")

createCmd := exec.CommandContext(ctx, "docker", dockerRunArgs...)
createCmd.CombinedOutput()
```

### 2. Execute Command with docker exec -i
```go
execArgs := []string{"exec", "-i", containerName, cmd}
execArgs = append(execArgs, args...)

execCmd := exec.CommandContext(ctx, "docker", execArgs...)
```

### 3. Create Pipes
```go
stdinPipe, _ := execCmd.StdinPipe()
stdoutPipe, _ := execCmd.StdoutPipe()
stderrPipe, _ := execCmd.StderrPipe()

execCmd.Start()
```

### 4. Cleanup in wait()
```go
waitFunc := func() error {
    execErr := execCmd.Wait()
    
    // Always clean up container
    cleanupCmd := exec.Command("docker", "rm", "-f", containerName)
    cleanupCmd.Run()
    
    return execErr
}
```

## Building Docker Commands

### GetBaseDockerCommand()
Builds common docker run options:
```go
func (o *DockerOptions) GetBaseDockerCommand(env []string) []string {
    parts := []string{"docker run --rm"}
    
    // Network
    if !o.AllowNetworking {
        parts = append(parts, "--network none")
    }
    
    // User
    if o.User != "" {
        parts = append(parts, fmt.Sprintf("--user %s", o.User))
    }
    
    // Memory limits
    if o.Memory != "" {
        parts = append(parts, fmt.Sprintf("--memory %s", o.Memory))
    }
    
    // Mounts
    for _, mount := range o.Mounts {
        parts = append(parts, fmt.Sprintf("-v %s", mount))
    }
    
    // Environment
    for _, e := range env {
        parts = append(parts, fmt.Sprintf("-e %s", e))
    }
    
    return parts
}
```

### GetDockerCommand()
For script-based execution:
```go
func (o *DockerOptions) GetDockerCommand(scriptFile string, env []string) string {
    parts := o.GetBaseDockerCommand(env)
    
    // Mount script file
    scriptName := filepath.Base(scriptFile)
    containerScriptPath := filepath.Join("/tmp", scriptName)
    parts = append(parts, fmt.Sprintf("-v %s:%s", scriptFile, containerScriptPath))
    
    // Add image and command
    parts = append(parts, o.Image)
    parts = append(parts, fmt.Sprintf("sh %s", containerScriptPath))
    
    return strings.Join(parts, " ")
}
```

## Error Handling

### Container Cleanup on Errors
Always clean up containers, even on errors:

```go
// In RunWithPipes
stdinPipe, err := execCmd.StdinPipe()
if err != nil {
    // Clean up the container we just created
    cleanupCmd := exec.Command("docker", "rm", "-f", containerName)
    cleanupCmd.Run()
    return nil, nil, nil, nil, err
}
```

### Logging Docker Commands
```go
r.logger.Debug("Creating background container: docker %v", dockerRunArgs)
r.logger.Debug("Executing in container: docker %v", execArgs)
r.logger.Debug("Container %s removed successfully", containerName)
```

## Testing Docker Runner

### Check Docker Availability
```go
func isDockerAvailable() bool {
    cmd := exec.Command("docker", "info")
    return cmd.Run() == nil
}

func TestDocker_Run(t *testing.T) {
    if !isDockerAvailable() {
        t.Skip("Docker not installed or not running, skipping test")
    }
    // ... test code
}
```

### Required Test Cases
1. Basic command execution
2. Environment variables
3. Network restrictions
4. Mounts
5. PrepareCommand
6. Single executable optimization
7. RunWithPipes with container cleanup

## Common Issues and Solutions

### Issue: Container Not Cleaned Up
**Solution**: Always use `docker rm -f` in wait() function, even if exec fails

### Issue: Network Isolation Not Working
**Solution**: Verify `--network none` is added when `AllowNetworking: false`

### Issue: Mounts Not Working
**Solution**: Ensure paths are absolute and accessible to Docker daemon

### Issue: Container Name Conflicts
**Solution**: Use timestamp-based unique names: `fmt.Sprintf("go-restricted-runner-%d", time.Now().UnixNano())`

## Docker-Specific Options

### Memory Limits
```go
Options{
    "memory": "512m",              // Hard limit
    "memory_reservation": "256m",  // Soft limit
    "memory_swap": "1g",           // Total memory + swap
}
```

### Network Configuration
```go
Options{
    "allow_networking": false,  // Disable all networking
    "network": "host",          // Use host network
    "network": "my-network",    // Use custom network
}
```

### Security Options
```go
Options{
    "cap_add": []string{"NET_ADMIN"},
    "cap_drop": []string{"ALL"},
    "user": "1000:1000",
}
```

