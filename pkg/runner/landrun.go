package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/inercia/go-restricted-runner/pkg/common"
	"github.com/landlock-lsm/go-landlock/landlock"
)

// Landrun implements the Runner interface using Linux Landlock LSM
type Landrun struct {
	logger  *common.Logger
	options LandrunOptions
}

// LandrunOptions is the options for the Landrun runner
type LandrunOptions struct {
	// Filesystem access
	AllowReadFolders      []string `json:"allow_read_folders"`       // Read-only access to directories
	AllowReadExecFolders  []string `json:"allow_read_exec_folders"`  // Read and execute access to directories
	AllowWriteFolders     []string `json:"allow_write_folders"`      // Write access to directories
	AllowWriteExecFolders []string `json:"allow_write_exec_folders"` // Write and execute access to directories

	// Network access (requires kernel 6.7+)
	AllowBindTCP    []uint16 `json:"allow_bind_tcp"`    // TCP ports allowed for binding
	AllowConnectTCP []uint16 `json:"allow_connect_tcp"` // TCP ports allowed for connecting

	// Unrestricted modes
	AllowNetworking        bool `json:"allow_networking"`        // Allow unrestricted network access
	UnrestrictedFilesystem bool `json:"unrestricted_filesystem"` // Allow unrestricted filesystem access

	// Best effort mode - gracefully degrade on older kernels
	BestEffort bool `json:"best_effort"`
}

// NewLandrunOptions creates a new LandrunOptions from Options
func NewLandrunOptions(options Options) (LandrunOptions, error) {
	var opts LandrunOptions
	jsonStr, err := options.ToJSON()
	if err != nil {
		return LandrunOptions{}, err
	}
	err = json.Unmarshal([]byte(jsonStr), &opts)
	return opts, err
}

// NewLandrun creates a new Landrun runner with the provided logger.
// If logger is nil, a default logger is created.
func NewLandrun(options Options, logger *common.Logger) (*Landrun, error) {
	if logger == nil {
		logger = common.GetLogger()
	}

	// Parse landrun-specific options
	landrunOpts, err := NewLandrunOptions(options)
	if err != nil {
		logger.Debug("Failed to parse landrun options: %v", err)
		return nil, fmt.Errorf("failed to parse landrun options: %w", err)
	}

	return &Landrun{
		logger:  logger,
		options: landrunOpts,
	}, nil
}

// CheckImplicitRequirements verifies that Landlock is available on the system.
func (r *Landrun) CheckImplicitRequirements() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("landrun runner requires Linux")
	}

	// Try to check if Landlock is available by attempting a minimal restriction
	// We use V1 as it's the minimum version (kernel 5.13+)
	err := landlock.V1.BestEffort().RestrictPaths()
	if err != nil {
		return fmt.Errorf("landlock not available on this kernel: %w", err)
	}

	r.logger.Debug("Landlock is available on this system")
	return nil
}

// buildLandlockRules constructs Landlock rules from the options and params
func (r *Landrun) buildLandlockRules(params map[string]interface{}) ([]landlock.Rule, error) {
	var rules []landlock.Rule

	// Process template variables in paths
	allowReadFolders := r.options.AllowReadFolders
	if len(allowReadFolders) > 0 {
		allowReadFolders = common.ProcessTemplateListFlexible(allowReadFolders, params)
	}

	allowReadExecFolders := r.options.AllowReadExecFolders
	if len(allowReadExecFolders) > 0 {
		allowReadExecFolders = common.ProcessTemplateListFlexible(allowReadExecFolders, params)
	}

	allowWriteFolders := r.options.AllowWriteFolders
	if len(allowWriteFolders) > 0 {
		allowWriteFolders = common.ProcessTemplateListFlexible(allowWriteFolders, params)
	}

	allowWriteExecFolders := r.options.AllowWriteExecFolders
	if len(allowWriteExecFolders) > 0 {
		allowWriteExecFolders = common.ProcessTemplateListFlexible(allowWriteExecFolders, params)
	}

	// Add filesystem rules
	if !r.options.UnrestrictedFilesystem {
		// Always allow access to /dev and /tmp for basic system operations
		// /dev is required for process execution and I/O operations
		// /tmp is required for temporary files used by tests and commands
		r.logger.Debug("Adding read-write access to /dev and /tmp for system operations")
		rules = append(rules, landlock.RWDirs("/dev", "/tmp"))

		if len(allowReadFolders) > 0 {
			r.logger.Debug("Adding read-only access to: %v", allowReadFolders)
			rules = append(rules, landlock.RODirs(allowReadFolders...))
		}

		if len(allowReadExecFolders) > 0 {
			r.logger.Debug("Adding read-execute access to: %v", allowReadExecFolders)
			// RODirs already includes execute permissions for files
			rules = append(rules, landlock.RODirs(allowReadExecFolders...))
		}

		if len(allowWriteFolders) > 0 {
			r.logger.Debug("Adding read-write access to: %v", allowWriteFolders)
			rules = append(rules, landlock.RWDirs(allowWriteFolders...))
		}

		if len(allowWriteExecFolders) > 0 {
			r.logger.Debug("Adding read-write-execute access to: %v", allowWriteExecFolders)
			rules = append(rules, landlock.RWDirs(allowWriteExecFolders...))
		}
	}

	// Add network rules (only if not allowing unrestricted networking)
	if !r.options.AllowNetworking {
		for _, port := range r.options.AllowBindTCP {
			r.logger.Debug("Adding TCP bind permission for port: %d", port)
			rules = append(rules, landlock.BindTCP(port))
		}

		for _, port := range r.options.AllowConnectTCP {
			r.logger.Debug("Adding TCP connect permission for port: %d", port)
			rules = append(rules, landlock.ConnectTCP(port))
		}
	}

	return rules, nil
}

// Run executes a command with Landlock restrictions and returns the output.
// It implements the Runner interface.
//
// Note: tmpfile parameter is ignored for landrun as restrictions are applied
// at the process level before command execution.
func (r *Landrun) Run(ctx context.Context, shell string, command string,
	env []string, params map[string]interface{}, tmpfile bool) (string, error) {

	// Check if context is done
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
		// Continue execution
	}

	r.logger.Debug("Landrun: executing command with Landlock restrictions")

	// Build Landlock rules
	rules, err := r.buildLandlockRules(params)
	if err != nil {
		return "", fmt.Errorf("failed to build landlock rules: %w", err)
	}

	// Apply Landlock restrictions to this process
	// Note: This affects the current process and all its children
	// Only apply restrictions if we actually have rules to enforce
	if len(rules) > 0 {
		config := landlock.V6
		if r.options.BestEffort {
			r.logger.Debug("Using best-effort mode for Landlock")
			config = config.BestEffort()
		}

		r.logger.Debug("Applying Landlock restrictions with %d rules", len(rules))
		if err := config.Restrict(rules...); err != nil {
			return "", fmt.Errorf("failed to apply landlock restrictions: %w", err)
		}
		r.logger.Debug("Landlock restrictions applied successfully")
	} else {
		r.logger.Debug("No Landlock restrictions to apply (unrestricted mode)")
	}

	// Now execute the command - it will inherit the Landlock restrictions
	configShell := getShell(shell)
	r.logger.Debug("Using shell: %s", configShell)

	// Get the appropriate command arguments for this shell
	shellPath, args := getShellCommandArgs(configShell, command)
	execCmd := exec.CommandContext(ctx, shellPath, args...)
	r.logger.Debug("Created command: %s with args %v", shellPath, args)

	// Set environment variables if provided
	if len(env) > 0 {
		r.logger.Debug("Adding %d environment variables to command", len(env))
		for _, e := range env {
			r.logger.Debug("... adding environment variable: %s", e)
		}
		execCmd.Env = append(os.Environ(), env...)
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr

	// Run the command
	r.logger.Debug("Executing command")

	if err := execCmd.Run(); err != nil {
		// If there's error output, include it in the error
		if stderr.Len() > 0 {
			errMsg := strings.TrimSpace(stderr.String())
			r.logger.Debug("Command failed with stderr: %s", errMsg)
			return "", errors.New(errMsg)
		}
		r.logger.Debug("Command failed with error: %v", err)
		return "", err
	}

	// Get the output
	outputStr := strings.TrimSpace(stdout.String())

	r.logger.Debug("Command executed successfully, output length: %d bytes", len(outputStr))
	if stderr.Len() > 0 {
		r.logger.Debug("Command generated stderr (but no error): %s", strings.TrimSpace(stderr.String()))
	}

	// Return the stdout output
	return outputStr, nil
}

// RunWithPipes executes a command with access to stdin/stdout/stderr pipes with Landlock restrictions.
// It implements the Runner interface for interactive process communication.
//
// The Landlock restrictions are applied before starting the command, and the command
// and all its children will inherit these restrictions.
func (r *Landrun) RunWithPipes(ctx context.Context, cmd string, args []string, env []string, params map[string]interface{}) (
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

	r.logger.Debug("RunWithPipes: executing command with Landlock: %s with args: %v", cmd, args)

	// Build Landlock rules
	rules, err := r.buildLandlockRules(params)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to build landlock rules: %w", err)
	}

	// Apply Landlock restrictions to this process
	// Only apply restrictions if we actually have rules to enforce
	if len(rules) > 0 {
		config := landlock.V6
		if r.options.BestEffort {
			r.logger.Debug("Using best-effort mode for Landlock")
			config = config.BestEffort()
		}

		r.logger.Debug("Applying Landlock restrictions with %d rules", len(rules))
		if err := config.Restrict(rules...); err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to apply landlock restrictions: %w", err)
		}
		r.logger.Debug("Landlock restrictions applied successfully")
	} else {
		r.logger.Debug("No Landlock restrictions to apply (unrestricted mode)")
	}

	// Create the command
	execCmd := exec.CommandContext(ctx, cmd, args...)

	// Set environment variables if provided
	if len(env) > 0 {
		r.logger.Debug("Adding %d environment variables to command", len(env))
		execCmd.Env = append(os.Environ(), env...)
	}

	// Create pipes
	stdinPipe, err := execCmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdoutPipe, err := execCmd.StdoutPipe()
	if err != nil {
		stdinPipe.Close()
		return nil, nil, nil, nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderrPipe, err := execCmd.StderrPipe()
	if err != nil {
		stdinPipe.Close()
		stdoutPipe.Close()
		return nil, nil, nil, nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := execCmd.Start(); err != nil {
		stdinPipe.Close()
		stdoutPipe.Close()
		stderrPipe.Close()
		return nil, nil, nil, nil, fmt.Errorf("failed to start command: %w", err)
	}

	r.logger.Debug("Command started successfully with PID: %d", execCmd.Process.Pid)

	// Create wait function
	waitFunc := func() error {
		err := execCmd.Wait()
		if err != nil {
			r.logger.Debug("Command exited with error: %v", err)
		} else {
			r.logger.Debug("Command exited successfully")
		}
		return err
	}

	return stdinPipe, stdoutPipe, stderrPipe, waitFunc, nil
}
