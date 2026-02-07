// Package runner provides isolated command execution environments.
//
// This package defines the Runner interface and implementations for executing
// commands in various isolation environments including direct execution,
// firejail (Linux), sandbox-exec (macOS), and Docker containers.
package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/inercia/go-restricted-runner/pkg/common"
)

// Type is an identifier for the type of runner to use.
// Each runner has its own set of implicit requirements that are checked
// automatically, so users don't need to explicitly specify common requirements
// in their tool configurations.
type Type string

const (
	// TypeExec is the standard command execution runner with no additional requirements
	TypeExec Type = "exec"

	// TypeSandboxExec is the macOS-specific sandbox-exec runner
	// Implicit requirements: OS=darwin, executables=[sandbox-exec]
	TypeSandboxExec Type = "sandbox-exec"

	// TypeFirejail is the Linux-specific firejail runner
	// Implicit requirements: OS=linux, executables=[firejail]
	TypeFirejail Type = "firejail"

	// TypeDocker is the Docker-based runner
	// Implicit requirements: executables=[docker]
	TypeDocker Type = "docker"
)

// Options is a map of options for the runner
type Options map[string]interface{}

// ToJSON converts the options to a JSON string
func (o Options) ToJSON() (string, error) {
	jsonBytes, err := json.Marshal(o)
	return string(jsonBytes), err
}

// Runner is an interface for running commands in isolated environments
type Runner interface {
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
	Run(ctx context.Context, shell string, command string, env []string, params map[string]interface{}, tmpfile bool) (string, error)

	// RunWithPipes executes a command with access to stdin/stdout/stderr pipes for interactive communication.
	//
	// This method is useful for long-running processes that require interactive input/output,
	// such as REPLs, interactive shells, or streaming data processors.
	//
	// Lifecycle:
	//   1. Call RunWithPipes to start the process and get pipes
	//   2. Write data to stdin as needed
	//   3. Read from stdout/stderr as needed (can be done concurrently)
	//   4. Close stdin when done writing to signal EOF to the process
	//   5. Read any remaining output from stdout/stderr
	//   6. Call wait() to wait for process completion and get exit status
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout. Cancelling the context will kill the process.
	//   - cmd: The command/executable to run
	//   - args: Command-line arguments for the command
	//   - env: Environment variables in KEY=VALUE format
	//   - params: Template parameters for variable substitution in paths/options
	//
	// Returns:
	//   - stdin: WriteCloser for sending input to the process. Must be closed when done.
	//   - stdout: ReadCloser for reading standard output from the process.
	//   - stderr: ReadCloser for reading standard error from the process.
	//   - wait: Function to call after reading output. Returns process exit error if any.
	//           Must be called to clean up resources even if you don't care about the exit status.
	//   - err: Error if the process failed to start.
	//
	// Important notes:
	//   - All restrictions (path, network, etc.) configured for the runner still apply
	//   - The wait() function MUST be called to properly clean up process resources
	//   - Reading from stdout/stderr after the process exits is safe
	//   - If context is cancelled, wait() will return context.Canceled or context.DeadlineExceeded
	//   - Closing stdin does not automatically terminate the process; some processes may continue running
	RunWithPipes(ctx context.Context, cmd string, args []string, env []string, params map[string]interface{}) (
		stdin io.WriteCloser,
		stdout io.ReadCloser,
		stderr io.ReadCloser,
		wait func() error,
		err error,
	)

	// CheckImplicitRequirements verifies that the runner's prerequisites are met.
	// This includes checking for required executables, OS compatibility, etc.
	//
	// Returns:
	//   - nil if all requirements are satisfied
	//   - An error describing which requirement is not met
	CheckImplicitRequirements() error
}

// New creates a new Runner based on the given type.
//
// Parameters:
//   - runnerType: The type of runner to create
//   - options: Configuration options for the runner
//   - logger: Logger for debug output (uses global logger if nil)
//
// Returns:
//   - A Runner instance if successful
//   - An error if creation fails or requirements are not met
func New(runnerType Type, options Options, logger *common.Logger) (Runner, error) {
	var runner Runner
	var err error

	// Create the runner instance based on type
	switch runnerType {
	case TypeExec:
		runner, err = NewExec(options, logger)
	case TypeSandboxExec:
		runner, err = NewSandboxExec(options, logger)
	case TypeFirejail:
		runner, err = NewFirejail(options, logger)
	case TypeDocker:
		runner, err = NewDocker(options, logger)
	default:
		return nil, fmt.Errorf("unknown runner type: %s", runnerType)
	}

	// Check if runner creation failed
	if err != nil {
		return nil, err
	}

	// Check implicit requirements for the created runner
	if err := runner.CheckImplicitRequirements(); err != nil {
		if logger != nil {
			logger.Debug("Runner %s failed implicit requirements check: %v", runnerType, err)
		}
		return nil, err
	}

	return runner, nil
}
