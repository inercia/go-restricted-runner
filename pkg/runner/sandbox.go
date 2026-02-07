package runner

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/inercia/go-restricted-runner/pkg/common"
)

//go:embed sandbox_profile.tpl
var sandboxProfileTemplate string

// SandboxExec implements the Runner interface using macOS sandbox-exec
type SandboxExec struct {
	logger     *common.Logger
	profileTpl *template.Template
	options    SandboxExecOptions
}

// SandboxExecOptions is the options for the SandboxExec runner
type SandboxExecOptions struct {
	Shell             string   `json:"shell"`
	AllowNetworking   bool     `json:"allow_networking"`
	AllowUserFolders  bool     `json:"allow_user_folders"`
	AllowReadFolders  []string `json:"allow_read_folders"`
	AllowWriteFolders []string `json:"allow_write_folders"`
	AllowReadFiles    []string `json:"allow_read_files"`
	AllowWriteFiles   []string `json:"allow_write_files"`
	CustomProfile     string   `json:"custom_profile"`
}

// NewSandboxExecOptions creates a new SandboxExecOptions from Options
func NewSandboxExecOptions(options Options) (SandboxExecOptions, error) {
	var opts SandboxExecOptions
	jsonStr, err := options.ToJSON()
	if err != nil {
		return SandboxExecOptions{}, err
	}
	err = json.Unmarshal([]byte(jsonStr), &opts)
	return opts, err
}

// NewSandboxExec creates a new SandboxExec runner with the provided logger.
// If logger is nil, a default logger is created.
func NewSandboxExec(options Options, logger *common.Logger) (*SandboxExec, error) {
	if logger == nil {
		logger = common.GetLogger()
	}

	// Parse the sandbox profile template
	profileTpl, err := template.New("sandbox-profile").Parse(sandboxProfileTemplate)
	if err != nil {
		logger.Debug("Failed to parse sandbox profile template: %v", err)
		return nil, err
	}

	// Parse sandbox-specific options
	sandboxOpts, err := NewSandboxExecOptions(options)
	if err != nil {
		logger.Debug("Failed to parse sandbox options: %v", err)
		return nil, fmt.Errorf("failed to parse sandbox options: %w", err)
	}

	return &SandboxExec{
		logger:     logger,
		profileTpl: profileTpl,
		options:    sandboxOpts,
	}, nil
}

// Run executes a command inside the macOS sandbox and returns the output.
// It implements the Runner interface.
//
// note: tmpfile is ignored for sandbox because it's not supported
func (r *SandboxExec) Run(ctx context.Context, shell string, command string, env []string, params map[string]interface{}, tmpfile bool) (string, error) {
	fullCmd := command

	// Check if context is done
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
		// Continue execution
	}

	// replace template variables in allow read and write folders and files
	if len(r.options.AllowReadFolders) > 0 {
		r.options.AllowReadFolders = common.ProcessTemplateListFlexible(r.options.AllowReadFolders, params)
	}
	if len(r.options.AllowWriteFolders) > 0 {
		r.options.AllowWriteFolders = common.ProcessTemplateListFlexible(r.options.AllowWriteFolders, params)
	}
	if len(r.options.AllowReadFiles) > 0 {
		r.options.AllowReadFiles = common.ProcessTemplateListFlexible(r.options.AllowReadFiles, params)

		// For macOS sandbox, we need to allow read access to parent directories
		// of files to enable directory traversal
		for _, filePath := range r.options.AllowReadFiles {
			dir := filepath.Dir(filePath)
			// Add parent directory if not already in the list
			if !contains(r.options.AllowReadFolders, dir) {
				r.options.AllowReadFolders = append(r.options.AllowReadFolders, dir)
				r.logger.Debug("[DEBUG] Added parent directory to allow list: %s", dir)
			}
		}
	}
	if len(r.options.AllowWriteFiles) > 0 {
		r.options.AllowWriteFiles = common.ProcessTemplateListFlexible(r.options.AllowWriteFiles, params)

		// For macOS sandbox, we need to allow write access to parent directories
		// of files to enable directory traversal
		for _, filePath := range r.options.AllowWriteFiles {
			dir := filepath.Dir(filePath)
			// Add parent directory if not already in the list
			if !contains(r.options.AllowWriteFolders, dir) {
				r.options.AllowWriteFolders = append(r.options.AllowWriteFolders, dir)
				r.logger.Debug("[DEBUG] Added parent directory to allow list: %s", dir)
			}
		}
	}

	// Generate the profile by rendering the template
	var profileBuf bytes.Buffer
	if err := r.profileTpl.Execute(&profileBuf, r.options); err != nil {
		r.logger.Debug("Failed to render sandbox profile template: %v", err)
		return "", fmt.Errorf("failed to render sandbox profile: %w", err)
	}

	profile := profileBuf.String()
	r.logger.Debug("Sandbox options: %+v", r.options)
	r.logger.Debug("Generated sandbox profile:\n%s", profile)

	// Create a temporary file for the sandbox profile
	profileFile, err := os.CreateTemp("", "sandbox-profile-*.sb")
	if err != nil {
		r.logger.Debug("Failed to create temporary profile file: %v", err)
		return "", fmt.Errorf("failed to create temporary profile file: %w", err)
	}
	defer func() {
		profileFilePath := profileFile.Name()
		if err := profileFile.Close(); err != nil {
			r.logger.Debug("Warning: failed to close profile file: %v", err)
		}
		if err := os.Remove(profileFilePath); err != nil {
			r.logger.Debug("Warning: failed to remove temporary profile file: %v", err)
		}
	}()

	// Write the profile to the temporary file
	if _, err := profileFile.WriteString(profile); err != nil {
		r.logger.Debug("Failed to write profile to temporary file: %v", err)
		return "", fmt.Errorf("failed to write profile to temporary file: %w", err)
	}

	// Flush data to ensure it's written to disk
	if err := profileFile.Sync(); err != nil {
		r.logger.Debug("Failed to sync profile file: %v", err)
		return "", fmt.Errorf("failed to sync profile file: %w", err)
	}

	var execCmd *exec.Cmd

	// Check if we can optimize by running a single executable directly
	if isSingleExecutableCommand(fullCmd) {
		r.logger.Debug("Optimization: running single executable command directly: %s", fullCmd)
		execCmd = exec.CommandContext(ctx, "sandbox-exec", "-f", profileFile.Name(), fullCmd)
	} else {
		// Create a temporary file for the command
		tmpScript, err := os.CreateTemp("", "sandbox-script-*.sh")
		if err != nil {
			r.logger.Debug("Failed to create temporary command file: %v", err)
			return "", fmt.Errorf("failed to create temporary command file: %w", err)
		}
		// Ensure temporary file is deleted when this function exits
		defer func() {
			tmpScriptPath := tmpScript.Name()
			if err := tmpScript.Close(); err != nil {
				r.logger.Debug("Warning: failed to close script file: %v", err)
			}
			if err := os.Remove(tmpScriptPath); err != nil {
				r.logger.Debug("Warning: failed to remove temporary script file: %v", err)
			}
		}()

		// Write the command to the temporary file
		if _, err := tmpScript.WriteString(fullCmd); err != nil {
			r.logger.Debug("Failed to write command to temporary file: %v", err)
			return "", fmt.Errorf("failed to write command to temporary file: %w", err)
		}

		// Flush data to ensure it's written to disk
		if err := tmpScript.Sync(); err != nil {
			r.logger.Debug("Failed to sync script file: %v", err)
			return "", fmt.Errorf("failed to sync script file: %w", err)
		}

		// Make the temporary file executable
		if err := os.Chmod(tmpScript.Name(), 0o700); err != nil {
			r.logger.Debug("Failed to make temporary file executable: %v", err)
			return "", fmt.Errorf("failed to make temporary file executable: %w", err)
		}

		execCmd = exec.CommandContext(ctx, "sandbox-exec", "-f", profileFile.Name(), tmpScript.Name())
	}

	r.logger.Debug("Created command: %s", execCmd.String())

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

// RunWithPipes executes a command with access to stdin/stdout/stderr pipes within the macOS sandbox.
// It implements the Runner interface for interactive process communication with sandbox restrictions.
//
// The command is executed within the macOS sandbox with all configured restrictions applied
// (network isolation, filesystem access controls, etc.).
func (r *SandboxExec) RunWithPipes(ctx context.Context, cmd string, args []string, env []string, params map[string]interface{}) (
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

	r.logger.Debug("RunWithPipes: executing command in sandbox: %s with args: %v", cmd, args)

	// Process template variables in allow read and write folders and files
	if len(r.options.AllowReadFolders) > 0 {
		r.options.AllowReadFolders = common.ProcessTemplateListFlexible(r.options.AllowReadFolders, params)
	}
	if len(r.options.AllowWriteFolders) > 0 {
		r.options.AllowWriteFolders = common.ProcessTemplateListFlexible(r.options.AllowWriteFolders, params)
	}
	if len(r.options.AllowReadFiles) > 0 {
		r.options.AllowReadFiles = common.ProcessTemplateListFlexible(r.options.AllowReadFiles, params)
	}
	if len(r.options.AllowWriteFiles) > 0 {
		r.options.AllowWriteFiles = common.ProcessTemplateListFlexible(r.options.AllowWriteFiles, params)
	}

	// Generate the sandbox profile
	var profileBuf bytes.Buffer
	if err := r.profileTpl.Execute(&profileBuf, r.options); err != nil {
		r.logger.Debug("Failed to render sandbox profile template: %v", err)
		return nil, nil, nil, nil, fmt.Errorf("failed to render sandbox profile: %w", err)
	}

	// Create a temporary file for the sandbox profile
	profileFile, err := os.CreateTemp("", "sandbox-profile-*.sb")
	if err != nil {
		r.logger.Debug("Failed to create temporary profile file: %v", err)
		return nil, nil, nil, nil, fmt.Errorf("failed to create temporary profile file: %w", err)
	}

	// Write the profile to the file
	if _, err := profileFile.Write(profileBuf.Bytes()); err != nil {
		profileFile.Close()
		os.Remove(profileFile.Name())
		r.logger.Debug("Failed to write sandbox profile: %v", err)
		return nil, nil, nil, nil, fmt.Errorf("failed to write sandbox profile: %w", err)
	}

	// Close the file so sandbox-exec can read it
	if err := profileFile.Close(); err != nil {
		os.Remove(profileFile.Name())
		r.logger.Debug("Failed to close profile file: %v", err)
		return nil, nil, nil, nil, fmt.Errorf("failed to close profile file: %w", err)
	}

	r.logger.Debug("Created sandbox profile at: %s", profileFile.Name())

	// Build the command with sandbox-exec
	// sandbox-exec -f <profile> <cmd> <args...>
	sandboxArgs := []string{"-f", profileFile.Name(), cmd}
	sandboxArgs = append(sandboxArgs, args...)

	execCmd := exec.CommandContext(ctx, "sandbox-exec", sandboxArgs...)

	// Set environment variables if provided
	if len(env) > 0 {
		r.logger.Debug("Adding %d environment variables to command", len(env))
		execCmd.Env = append(os.Environ(), env...)
	}

	// Create pipes for stdin, stdout, and stderr
	stdinPipe, err := execCmd.StdinPipe()
	if err != nil {
		os.Remove(profileFile.Name())
		r.logger.Debug("Failed to create stdin pipe: %v", err)
		return nil, nil, nil, nil, errors.New("failed to create stdin pipe: " + err.Error())
	}

	stdoutPipe, err := execCmd.StdoutPipe()
	if err != nil {
		stdinPipe.Close()
		os.Remove(profileFile.Name())
		r.logger.Debug("Failed to create stdout pipe: %v", err)
		return nil, nil, nil, nil, errors.New("failed to create stdout pipe: " + err.Error())
	}

	stderrPipe, err := execCmd.StderrPipe()
	if err != nil {
		stdinPipe.Close()
		stdoutPipe.Close()
		os.Remove(profileFile.Name())
		r.logger.Debug("Failed to create stderr pipe: %v", err)
		return nil, nil, nil, nil, errors.New("failed to create stderr pipe: " + err.Error())
	}

	// Start the command
	r.logger.Debug("Starting sandboxed command with pipes")
	if err := execCmd.Start(); err != nil {
		stdinPipe.Close()
		stdoutPipe.Close()
		stderrPipe.Close()
		os.Remove(profileFile.Name())
		r.logger.Debug("Failed to start command: %v", err)
		return nil, nil, nil, nil, errors.New("failed to start command: " + err.Error())
	}

	r.logger.Debug("Sandboxed command started successfully with PID: %d", execCmd.Process.Pid)

	// Create wait function that waits for the command to complete and cleans up
	waitFunc := func() error {
		r.logger.Debug("Waiting for sandboxed command to complete")
		err := execCmd.Wait()

		// Clean up the profile file
		if removeErr := os.Remove(profileFile.Name()); removeErr != nil {
			r.logger.Debug("Warning: failed to remove sandbox profile file %s: %v", profileFile.Name(), removeErr)
		}

		if err != nil {
			r.logger.Debug("Sandboxed command completed with error: %v", err)
			return err
		}
		r.logger.Debug("Sandboxed command completed successfully")
		return nil
	}

	return stdinPipe, stdoutPipe, stderrPipe, waitFunc, nil
}

// CheckImplicitRequirements checks if the runner meets its implicit requirements.
// SandboxExec runner requires macOS and the sandbox-exec executable.
func (r *SandboxExec) CheckImplicitRequirements() error {
	// Sandbox exec is macOS only
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("sandbox-exec runner requires macOS")
	}

	// Check if sandbox-exec is available
	if !common.CheckExecutableExists("sandbox-exec") {
		return fmt.Errorf("sandbox-exec executable not found in PATH")
	}

	return nil
}
