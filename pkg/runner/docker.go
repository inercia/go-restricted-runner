// Package runner provides isolated command execution environments.
package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/inercia/go-restricted-runner/pkg/common"
)

// Docker executes commands inside a Docker container.
type Docker struct {
	logger *common.Logger
	opts   DockerOptions
}

// DockerOptions represents configuration options for the Docker runner.
type DockerOptions struct {
	// The Docker image to use (required)
	Image string `json:"image"`

	// Additional Docker run options
	DockerRunOpts string `json:"docker_run_opts"`

	// Mount points in the format "hostpath:containerpath"
	Mounts []string `json:"mounts"`

	// Whether to allow networking in the container
	AllowNetworking bool `json:"allow_networking"`

	// Specific network to connect container to (e.g. "host", "bridge", or custom network name)
	Network string `json:"network"`

	// User to run as inside the container (defaults to current user)
	User string `json:"user"`

	// Working directory inside the container
	WorkDir string `json:"workdir"`

	// PrepareCommand is a command to run before the main command
	PrepareCommand string `json:"prepare_command"`

	// Memory limit (e.g. "512m", "1g")
	Memory string `json:"memory"`

	// Memory soft limit (e.g. "256m", "512m")
	MemoryReservation string `json:"memory_reservation"`

	// Swap limit equal to memory plus swap: '-1' to enable unlimited swap
	MemorySwap string `json:"memory_swap"`

	// Tune container memory swappiness (0 to 100)
	MemorySwappiness int `json:"memory_swappiness"`

	// Linux capabilities to add to the container
	CapAdd []string `json:"cap_add"`

	// Linux capabilities to drop from the container
	CapDrop []string `json:"cap_drop"`

	// Custom DNS servers for the container
	DNS []string `json:"dns"`

	// Custom DNS search domains for the container
	DNSSearch []string `json:"dns_search"`

	// Set platform if server is multi-platform capable (e.g., "linux/amd64", "linux/arm64")
	Platform string `json:"platform"`
}

// GetBaseDockerCommand creates the common parts of a docker run command with all configured options.
// It returns a slice of command parts that can be further customized by the calling method.
func (o *DockerOptions) GetBaseDockerCommand(env []string) []string {
	// Start with basic docker run command
	parts := []string{"docker run --rm"}

	// Add networking option
	if !o.AllowNetworking {
		parts = append(parts, "--network none")
	} else if o.Network != "" {
		parts = append(parts, fmt.Sprintf("--network %s", o.Network))
	}

	// Add user if specified
	if o.User != "" {
		parts = append(parts, fmt.Sprintf("--user %s", o.User))
	}

	// Add working directory if specified
	if o.WorkDir != "" {
		parts = append(parts, fmt.Sprintf("--workdir %s", o.WorkDir))
	}

	// Add memory options if specified
	if o.Memory != "" {
		parts = append(parts, fmt.Sprintf("--memory %s", o.Memory))
	}

	if o.MemoryReservation != "" {
		parts = append(parts, fmt.Sprintf("--memory-reservation %s", o.MemoryReservation))
	}

	if o.MemorySwap != "" {
		parts = append(parts, fmt.Sprintf("--memory-swap %s", o.MemorySwap))
	}

	if o.MemorySwappiness != -1 {
		parts = append(parts, fmt.Sprintf("--memory-swappiness %d", o.MemorySwappiness))
	}

	// Add Linux capabilities options
	for _, cap := range o.CapAdd {
		parts = append(parts, fmt.Sprintf("--cap-add %s", cap))
	}

	for _, cap := range o.CapDrop {
		parts = append(parts, fmt.Sprintf("--cap-drop %s", cap))
	}

	// Add DNS servers
	for _, dns := range o.DNS {
		parts = append(parts, fmt.Sprintf("--dns %s", dns))
	}

	// Add DNS search domains
	for _, dnsSearch := range o.DNSSearch {
		parts = append(parts, fmt.Sprintf("--dns-search %s", dnsSearch))
	}

	// Add platform if specified
	if o.Platform != "" {
		parts = append(parts, fmt.Sprintf("--platform %s", o.Platform))
	}

	// Add custom docker run options
	if o.DockerRunOpts != "" {
		parts = append(parts, o.DockerRunOpts)
	}

	// Add additional mounts
	for _, mount := range o.Mounts {
		parts = append(parts, fmt.Sprintf("-v %s", mount))
	}

	// Add environment variables (shell-quoted to handle values with spaces)
	for _, e := range env {
		parts = append(parts, fmt.Sprintf("-e %s", shellQuote(e)))
	}

	return parts
}

// GetDockerCommand constructs the docker run command with a script file.
func (o *DockerOptions) GetDockerCommand(scriptFile string, env []string) string {
	// Get base docker command parts
	parts := o.GetBaseDockerCommand(env)

	// Mount the script file
	scriptName := filepath.Base(scriptFile)
	containerScriptPath := filepath.Join("/tmp", scriptName)
	parts = append(parts, fmt.Sprintf("-v %s:%s", scriptFile, containerScriptPath))

	// Add image and the command to execute the script
	parts = append(parts, o.Image)
	parts = append(parts, fmt.Sprintf("sh %s", containerScriptPath))

	// Join all parts
	return strings.Join(parts, " ")
}

// GetDirectExecutionCommand constructs the docker run command for direct executable execution.
// This is used to optimize the case where we're just running a single executable without a temp script.
func (o *DockerOptions) GetDirectExecutionCommand(cmd string, env []string) string {
	// Get base docker command parts
	parts := o.GetBaseDockerCommand(env)

	// Add image and direct command
	parts = append(parts, o.Image)
	parts = append(parts, cmd)

	// Join all parts into a single command
	return strings.Join(parts, " ")
}

// NewDockerOptions extracts Docker-specific options from generic runner options.
func NewDockerOptions(genericOpts Options) (DockerOptions, error) {
	opts := DockerOptions{
		AllowNetworking:  true, // Default to allowing networking
		User:             "",   // Default to Docker's default user
		WorkDir:          "",   // Default to Docker's default working directory
		MemorySwappiness: -1,   // Default to Docker's default swappiness
	}

	// Parse image (required)
	if image, ok := genericOpts["image"].(string); ok {
		opts.Image = image
	} else {
		return opts, fmt.Errorf("docker runner requires 'image' option")
	}

	// Parse optional docker run options
	if dockerRunOpts, ok := genericOpts["docker_run_opts"].(string); ok {
		opts.DockerRunOpts = dockerRunOpts
	}

	// Parse optional mounts
	if mounts, ok := genericOpts["mounts"].([]interface{}); ok {
		for _, m := range mounts {
			if mountStr, ok := m.(string); ok {
				opts.Mounts = append(opts.Mounts, mountStr)
			}
		}
	}

	// Parse networking option
	if allowNetworking, ok := genericOpts["allow_networking"].(bool); ok {
		opts.AllowNetworking = allowNetworking
	}

	// Parse network option
	if network, ok := genericOpts["network"].(string); ok {
		opts.Network = network
	}

	// Parse user option
	if user, ok := genericOpts["user"].(string); ok {
		opts.User = user
	}

	// Parse working directory option
	if workDir, ok := genericOpts["workdir"].(string); ok {
		opts.WorkDir = workDir
	}

	// Parse prepare command option
	if prepareCommand, ok := genericOpts["prepare_command"].(string); ok {
		opts.PrepareCommand = prepareCommand
	}

	// Parse memory option
	if memory, ok := genericOpts["memory"].(string); ok {
		opts.Memory = memory
	}

	// Parse memory reservation option
	if memoryReservation, ok := genericOpts["memory_reservation"].(string); ok {
		opts.MemoryReservation = memoryReservation
	}

	// Parse memory swap option
	if memorySwap, ok := genericOpts["memory_swap"].(string); ok {
		opts.MemorySwap = memorySwap
	}

	// Parse memory swappiness option (integer)
	if swappiness, ok := genericOpts["memory_swappiness"].(float64); ok {
		opts.MemorySwappiness = int(swappiness)
	}

	// Parse capabilities to add
	if capAdd, ok := genericOpts["cap_add"].([]interface{}); ok {
		for _, cap := range capAdd {
			if capStr, ok := cap.(string); ok {
				opts.CapAdd = append(opts.CapAdd, capStr)
			}
		}
	}

	// Parse capabilities to drop
	if capDrop, ok := genericOpts["cap_drop"].([]interface{}); ok {
		for _, cap := range capDrop {
			if capStr, ok := cap.(string); ok {
				opts.CapDrop = append(opts.CapDrop, capStr)
			}
		}
	}

	// Parse DNS servers
	if dns, ok := genericOpts["dns"].([]interface{}); ok {
		for _, server := range dns {
			if serverStr, ok := server.(string); ok {
				opts.DNS = append(opts.DNS, serverStr)
			}
		}
	}

	// Parse DNS search domains
	if dnsSearch, ok := genericOpts["dns_search"].([]interface{}); ok {
		for _, domain := range dnsSearch {
			if domainStr, ok := domain.(string); ok {
				opts.DNSSearch = append(opts.DNSSearch, domainStr)
			}
		}
	}

	// Parse platform option
	if platform, ok := genericOpts["platform"].(string); ok {
		opts.Platform = platform
	}

	return opts, nil
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// NewDocker creates a new Docker runner with the specified options.
func NewDocker(options Options, logger *common.Logger) (*Docker, error) {
	if logger == nil {
		logger = common.GetLogger()
	}

	dockerOpts, err := NewDockerOptions(options)
	if err != nil {
		return nil, err
	}

	// Docker executable and daemon checks are now handled by CheckImplicitRequirements()
	return &Docker{
		logger: logger,
		opts:   dockerOpts,
	}, nil
}

// CheckImplicitRequirements checks if the runner meets its implicit requirements.
// Docker runner requires the docker executable and a running daemon.
func (r *Docker) CheckImplicitRequirements() error {
	// Check if docker executable exists
	if !common.CheckExecutableExists("docker") {
		return fmt.Errorf("docker executable not found in PATH")
	}

	// Check if Docker daemon is running
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "stats", "--no-stream")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker daemon is not running: %w", err)
	}

	return nil
}

// Run executes the command using Docker.
func (r *Docker) Run(ctx context.Context, shell string, cmd string, env []string, params map[string]interface{}, tmpfile bool) (string, error) {
	// Create an exec runner that we'll use to execute the docker command
	execRunner, err := NewExec(Options{}, r.logger)
	if err != nil {
		return "", fmt.Errorf("failed to create exec runner: %w", err)
	}

	var dockerCmd string

	// Determine if we should run directly or via script
	if isSingleExecutableCommand(cmd) {
		r.logger.Debug("Optimization: running single executable command directly in Docker: %s", cmd)

		// Build docker command to directly execute the command without a temp script
		dockerCmd = r.opts.GetDirectExecutionCommand(cmd, env)
	} else {
		// Create a temporary script file
		scriptFile, err := r.createScriptFile(shell, cmd, env)
		if err != nil {
			return "", fmt.Errorf("failed to create script file: %w", err)
		}

		// Clean up the temporary script file when done
		defer func() {
			if err := os.Remove(scriptFile); err != nil {
				r.logger.Debug("Warning: failed to remove temporary script file %s: %v", scriptFile, err)
			}
		}()

		r.logger.Debug("Created temporary script file: %s", scriptFile)

		// Construct the docker run command with the script file
		dockerCmd = r.opts.GetDockerCommand(scriptFile, env)
	}

	r.logger.Debug("Running command in Docker: %s", dockerCmd)

	// Run the docker command - we set tmpfile to false because dockerCmd is already a full command
	output, err := execRunner.Run(ctx, "sh", dockerCmd, nil, params, false)
	if err != nil {
		return "", fmt.Errorf("docker command execution failed: %w", err)
	}

	return output, nil
}

// createScriptFile writes the command to a temporary script file.
func (r *Docker) createScriptFile(shell string, cmd string, env []string) (string, error) {
	// Create a temporary file with a specific pattern
	tmpFile, err := os.CreateTemp("", "mcpshell-docker-*.sh")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary script file: %w", err)
	}

	// Get the name for later usage
	scriptPath := tmpFile.Name()

	// Prepare script content
	var content strings.Builder
	content.WriteString("#!/bin/sh\n\n")

	// Add environment variables (shell-quoted to handle values with spaces)
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			fmt.Fprintf(&content, "export %s=%s\n", parts[0], shellQuote(parts[1]))
		}
	}

	// Add preparation command if specified
	if r.opts.PrepareCommand != "" {
		content.WriteString("\n# Preparation commands\n")
		content.WriteString(r.opts.PrepareCommand)
		content.WriteString("\n\n")
		r.logger.Debug("Added preparation command to script: %s", r.opts.PrepareCommand)
	}

	// Add the main command (trim whitespace to avoid issues with trailing newlines from YAML literal blocks)
	content.WriteString("# Main command to execute\n")
	trimmedCmd := strings.TrimSpace(cmd)
	if shell != "" {
		fmt.Fprintf(&content, "exec %s -c %q\n", shell, trimmedCmd)
	} else {
		fmt.Fprintf(&content, "exec sh -c %q\n", trimmedCmd)
	}

	// Write the content to the file
	if _, err := tmpFile.WriteString(content.String()); err != nil {
		// Close and remove the file in case of an error
		_ = tmpFile.Close()       // Ignore close error, we already have a write error
		_ = os.Remove(scriptPath) // Best effort cleanup
		return "", fmt.Errorf("failed to write to temporary script file: %w", err)
	}

	// Make the file executable (chmod +x)
	if err := os.Chmod(scriptPath, 0755); err != nil {
		_ = tmpFile.Close()       // Ignore close error, we already have a chmod error
		_ = os.Remove(scriptPath) // Best effort cleanup
		return "", fmt.Errorf("failed to make script file executable: %w", err)
	}

	// Close the file
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(scriptPath) // Best effort cleanup
		return "", fmt.Errorf("failed to close temporary script file: %w", err)
	}

	r.logger.Debug("Created temporary script file at: %s", scriptPath)
	return scriptPath, nil
}

// RunWithPipes executes a command with access to stdin/stdout/stderr pipes inside a Docker container.
// It implements the Runner interface for interactive process communication with Docker isolation.
//
// This implementation creates a temporary container, runs the command with interactive mode,
// and provides pipes for communication. All Docker restrictions (network, mounts, etc.) are applied.
//
// Note: For Docker, we use a different approach than Run(). We create a temporary long-running
// container and use 'docker exec -i' to run the command interactively within it.
func (r *Docker) RunWithPipes(ctx context.Context, cmd string, args []string, env []string, params map[string]interface{}) (
	stdin io.WriteCloser,
	stdout io.ReadCloser,
	stderr io.ReadCloser,
	wait func() error,
	err error,
) {
	// Check if context is already done
	select {
	case <-ctx.Done():
		return nil, nil, nil, nil, ctx.Err()
	default:
		// Continue execution
	}

	r.logger.Debug("RunWithPipes: executing command in Docker: %s with args: %v", cmd, args)

	// First, create a long-running container that we can exec into
	// We'll use a sleep command to keep the container alive
	containerName := fmt.Sprintf("go-restricted-runner-%d", time.Now().UnixNano())

	// Build docker run command for the background container
	dockerRunArgs := []string{"run", "--name", containerName, "-d"}

	// Add resource limits
	if r.opts.Memory != "" {
		dockerRunArgs = append(dockerRunArgs, "--memory", r.opts.Memory)
	}
	if r.opts.MemoryReservation != "" {
		dockerRunArgs = append(dockerRunArgs, "--memory-reservation", r.opts.MemoryReservation)
	}
	if r.opts.MemorySwap != "" {
		dockerRunArgs = append(dockerRunArgs, "--memory-swap", r.opts.MemorySwap)
	}

	// Add network configuration
	if !r.opts.AllowNetworking {
		dockerRunArgs = append(dockerRunArgs, "--network", "none")
	} else if r.opts.Network != "" {
		dockerRunArgs = append(dockerRunArgs, "--network", r.opts.Network)
	}

	// Add user if specified
	if r.opts.User != "" {
		dockerRunArgs = append(dockerRunArgs, "--user", r.opts.User)
	}

	// Add working directory if specified
	if r.opts.WorkDir != "" {
		dockerRunArgs = append(dockerRunArgs, "--workdir", r.opts.WorkDir)
	}

	// Add mounts
	for _, mount := range r.opts.Mounts {
		dockerRunArgs = append(dockerRunArgs, "-v", mount)
	}

	// Add environment variables
	for _, envVar := range env {
		dockerRunArgs = append(dockerRunArgs, "-e", envVar)
	}

	// Add the image and a sleep command to keep container alive
	dockerRunArgs = append(dockerRunArgs, r.opts.Image, "sleep", "infinity")

	r.logger.Debug("Creating background container: docker %v", dockerRunArgs)

	// Create the container
	createCmd := exec.CommandContext(ctx, "docker", dockerRunArgs...)
	if output, err := createCmd.CombinedOutput(); err != nil {
		r.logger.Debug("Failed to create container: %v, output: %s", err, string(output))
		return nil, nil, nil, nil, fmt.Errorf("failed to create container: %w: %s", err, string(output))
	}

	r.logger.Debug("Created container: %s", containerName)

	// Build the docker exec command with interactive mode
	// docker exec -i <container> <cmd> <args...>
	execArgs := []string{"exec", "-i", containerName, cmd}
	execArgs = append(execArgs, args...)

	r.logger.Debug("Executing in container: docker %v", execArgs)

	execCmd := exec.CommandContext(ctx, "docker", execArgs...)

	// Create pipes for stdin, stdout, and stderr
	stdinPipe, err := execCmd.StdinPipe()
	if err != nil {
		// Clean up the container
		cleanupCmd := exec.Command("docker", "rm", "-f", containerName)
		cleanupCmd.Run()
		r.logger.Debug("Failed to create stdin pipe: %v", err)
		return nil, nil, nil, nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdoutPipe, err := execCmd.StdoutPipe()
	if err != nil {
		stdinPipe.Close()
		cleanupCmd := exec.Command("docker", "rm", "-f", containerName)
		cleanupCmd.Run()
		r.logger.Debug("Failed to create stdout pipe: %v", err)
		return nil, nil, nil, nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderrPipe, err := execCmd.StderrPipe()
	if err != nil {
		stdinPipe.Close()
		stdoutPipe.Close()
		cleanupCmd := exec.Command("docker", "rm", "-f", containerName)
		cleanupCmd.Run()
		r.logger.Debug("Failed to create stderr pipe: %v", err)
		return nil, nil, nil, nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the exec command
	r.logger.Debug("Starting docker exec command")
	if err := execCmd.Start(); err != nil {
		stdinPipe.Close()
		stdoutPipe.Close()
		stderrPipe.Close()
		cleanupCmd := exec.Command("docker", "rm", "-f", containerName)
		cleanupCmd.Run()
		r.logger.Debug("Failed to start docker exec: %v", err)
		return nil, nil, nil, nil, fmt.Errorf("failed to start docker exec: %w", err)
	}

	r.logger.Debug("Docker exec started successfully")

	// Create wait function that waits for the command to complete and cleans up the container
	waitFunc := func() error {
		r.logger.Debug("Waiting for docker exec to complete")
		execErr := execCmd.Wait()

		// Clean up the container
		r.logger.Debug("Cleaning up container: %s", containerName)
		cleanupCmd := exec.Command("docker", "rm", "-f", containerName)
		if cleanupOutput, cleanupErr := cleanupCmd.CombinedOutput(); cleanupErr != nil {
			r.logger.Debug("Warning: failed to remove container %s: %v, output: %s", containerName, cleanupErr, string(cleanupOutput))
		} else {
			r.logger.Debug("Container %s removed successfully", containerName)
		}

		if execErr != nil {
			r.logger.Debug("Docker exec completed with error: %v", execErr)
			return execErr
		}
		r.logger.Debug("Docker exec completed successfully")
		return nil
	}

	return stdinPipe, stdoutPipe, stderrPipe, waitFunc, nil
}
