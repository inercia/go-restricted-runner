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
	"runtime"
	"strings"
	"text/template"

	"github.com/inercia/go-restricted-runner/pkg/common"
)

//go:embed firejail_profile.tpl
var firejailProfileTemplate string

// Firejail implements the Runner interface using firejail on Linux
type Firejail struct {
	logger     *common.Logger
	profileTpl *template.Template
	options    FirejailOptions
}

// FirejailOptions is the options for the Firejail runner
type FirejailOptions struct {
	Shell             string   `json:"shell"`
	AllowNetworking   bool     `json:"allow_networking"`
	AllowUserFolders  bool     `json:"allow_user_folders"`
	AllowReadFolders  []string `json:"allow_read_folders"`
	AllowWriteFolders []string `json:"allow_write_folders"`
	AllowReadFiles    []string `json:"allow_read_files"`
	AllowWriteFiles   []string `json:"allow_write_files"`
	CustomProfile     string   `json:"custom_profile"`
}

// NewFirejailOptions creates a new FirejailOptions from Options
func NewFirejailOptions(options Options) (FirejailOptions, error) {
	var opts FirejailOptions
	jsonStr, err := options.ToJSON()
	if err != nil {
		return FirejailOptions{}, err
	}
	err = json.Unmarshal([]byte(jsonStr), &opts)
	return opts, err
}

// NewFirejail creates a new Firejail runner with the provided logger.
// If logger is nil, a default logger is created.
func NewFirejail(options Options, logger *common.Logger) (*Firejail, error) {
	if logger == nil {
		logger = common.GetLogger()
	}

	// Parse the firejail profile template
	profileTpl, err := template.New("firejail-profile").Parse(firejailProfileTemplate)
	if err != nil {
		logger.Debug("Failed to parse firejail profile template: %v", err)
		return nil, err
	}

	// Parse firejail-specific options
	firejailOpts, err := NewFirejailOptions(options)
	if err != nil {
		logger.Debug("Failed to parse firejail options: %v", err)
		return nil, fmt.Errorf("failed to parse firejail options: %w", err)
	}

	return &Firejail{
		logger:     logger,
		profileTpl: profileTpl,
		options:    firejailOpts,
	}, nil
}

// Run executes a command inside the firejail sandbox and returns the output.
// It implements the Runner interface.
//
// note: tmpfile is ignored for firejail because it's not supported
func (r *Firejail) Run(ctx context.Context,
	shell string, command string,
	env []string, params map[string]interface{}, tmpfile bool,
) (string, error) {
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
	}
	if len(r.options.AllowWriteFiles) > 0 {
		r.options.AllowWriteFiles = common.ProcessTemplateListFlexible(r.options.AllowWriteFiles, params)
	}

	// Generate the profile by rendering the template
	var profileBuf bytes.Buffer
	if err := r.profileTpl.Execute(&profileBuf, r.options); err != nil {
		r.logger.Debug("Failed to render firejail profile template: %v", err)
		return "", fmt.Errorf("failed to render firejail profile: %w", err)
	}

	profile := profileBuf.String()
	r.logger.Debug("Firejail options: %+v", r.options)
	r.logger.Debug("Generated firejail profile: %s", profile)

	// Create a temporary file for the firejail profile
	profileFile, err := os.CreateTemp("", "firejail-profile-*.profile")
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
		execCmd = exec.CommandContext(ctx, "firejail", "--profile="+profileFile.Name(), fullCmd)
	} else {
		// Create a temporary file for the command
		tmpScript, err := os.CreateTemp("", "firejail-command-*.sh")
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

		execCmd = exec.CommandContext(ctx, "firejail", "--profile="+profileFile.Name(), tmpScript.Name())
	}

	// Check if context is done
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
		// Continue execution
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

// RunWithPipes executes a command with access to stdin/stdout/stderr pipes within the firejail sandbox.
// It implements the Runner interface for interactive process communication with firejail restrictions.
//
// The command is executed within the firejail sandbox with all configured restrictions applied
// (network isolation, filesystem access controls, etc.).
func (r *Firejail) RunWithPipes(ctx context.Context, cmd string, args []string, env []string, params map[string]interface{}) (
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

	r.logger.Debug("RunWithPipes: executing command in firejail: %s with args: %v", cmd, args)

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

	// Generate the firejail profile
	var profileBuf bytes.Buffer
	if err := r.profileTpl.Execute(&profileBuf, r.options); err != nil {
		r.logger.Debug("Failed to render firejail profile template: %v", err)
		return nil, nil, nil, nil, fmt.Errorf("failed to render firejail profile: %w", err)
	}

	// Create a temporary file for the firejail profile
	profileFile, err := os.CreateTemp("", "firejail-profile-*.profile")
	if err != nil {
		r.logger.Debug("Failed to create temporary profile file: %v", err)
		return nil, nil, nil, nil, fmt.Errorf("failed to create temporary profile file: %w", err)
	}

	// Write the profile to the file
	if _, err := profileFile.Write(profileBuf.Bytes()); err != nil {
		profileFile.Close()
		os.Remove(profileFile.Name())
		r.logger.Debug("Failed to write firejail profile: %v", err)
		return nil, nil, nil, nil, fmt.Errorf("failed to write firejail profile: %w", err)
	}

	// Close the file so firejail can read it
	if err := profileFile.Close(); err != nil {
		os.Remove(profileFile.Name())
		r.logger.Debug("Failed to close profile file: %v", err)
		return nil, nil, nil, nil, fmt.Errorf("failed to close profile file: %w", err)
	}

	r.logger.Debug("Created firejail profile at: %s", profileFile.Name())

	// Build the command with firejail
	// firejail --profile=<profile> <cmd> <args...>
	firejailArgs := []string{"--profile=" + profileFile.Name(), cmd}
	firejailArgs = append(firejailArgs, args...)

	execCmd := exec.CommandContext(ctx, "firejail", firejailArgs...)

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
	r.logger.Debug("Starting firejail command with pipes")
	if err := execCmd.Start(); err != nil {
		stdinPipe.Close()
		stdoutPipe.Close()
		stderrPipe.Close()
		os.Remove(profileFile.Name())
		r.logger.Debug("Failed to start command: %v", err)
		return nil, nil, nil, nil, errors.New("failed to start command: " + err.Error())
	}

	r.logger.Debug("Firejail command started successfully with PID: %d", execCmd.Process.Pid)

	// Create wait function that waits for the command to complete and cleans up
	waitFunc := func() error {
		r.logger.Debug("Waiting for firejail command to complete")
		err := execCmd.Wait()

		// Clean up the profile file
		if removeErr := os.Remove(profileFile.Name()); removeErr != nil {
			r.logger.Debug("Warning: failed to remove firejail profile file %s: %v", profileFile.Name(), removeErr)
		}

		if err != nil {
			r.logger.Debug("Firejail command completed with error: %v", err)
			return err
		}
		r.logger.Debug("Firejail command completed successfully")
		return nil
	}

	return stdinPipe, stdoutPipe, stderrPipe, waitFunc, nil
}

// CheckImplicitRequirements checks if the runner meets its implicit requirements.
// Firejail runner requires Linux and the firejail executable.
func (r *Firejail) CheckImplicitRequirements() error {
	// Firejail is Linux only
	if runtime.GOOS != "linux" {
		return fmt.Errorf("firejail runner requires Linux")
	}

	// Check if firejail is available
	if !common.CheckExecutableExists("firejail") {
		return fmt.Errorf("firejail executable not found in PATH")
	}

	return nil
}
