package runner

import (
	"context"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/inercia/go-restricted-runner/pkg/common"
)

// TestExecRunner_RunWithPipes_BasicEcho tests basic stdin/stdout functionality
func TestExecRunner_RunWithPipes_BasicEcho(t *testing.T) {
	logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)
	runner, err := NewExec(Options{}, logger)
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}

	ctx := context.Background()

	// Test: cat command echoes input back
	stdin, stdout, stderr, wait, err := runner.RunWithPipes(ctx, "cat", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunWithPipes failed: %v", err)
	}

	// Write to stdin
	testInput := "hello world\n"
	_, err = stdin.Write([]byte(testInput))
	if err != nil {
		t.Fatalf("Failed to write to stdin: %v", err)
	}
	stdin.Close()

	// Read from stdout
	output, err := io.ReadAll(stdout)
	if err != nil {
		t.Fatalf("Failed to read from stdout: %v", err)
	}

	// Read from stderr (should be empty)
	stderrOutput, err := io.ReadAll(stderr)
	if err != nil {
		t.Fatalf("Failed to read from stderr: %v", err)
	}

	// Wait for completion
	err = wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	// Verify output
	if string(output) != testInput {
		t.Errorf("Expected output %q, got %q", testInput, string(output))
	}

	if len(stderrOutput) > 0 {
		t.Errorf("Expected empty stderr, got %q", string(stderrOutput))
	}
}

// TestExecRunner_RunWithPipes_MultipleWrites tests multiple writes to stdin
func TestExecRunner_RunWithPipes_MultipleWrites(t *testing.T) {
	logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)
	runner, err := NewExec(Options{}, logger)
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}

	ctx := context.Background()

	stdin, stdout, stderr, wait, err := runner.RunWithPipes(ctx, "cat", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunWithPipes failed: %v", err)
	}

	// Write multiple lines
	lines := []string{"line1\n", "line2\n", "line3\n"}
	for _, line := range lines {
		_, err = stdin.Write([]byte(line))
		if err != nil {
			t.Fatalf("Failed to write to stdin: %v", err)
		}
	}
	stdin.Close()

	// Read all output
	output, err := io.ReadAll(stdout)
	if err != nil {
		t.Fatalf("Failed to read from stdout: %v", err)
	}

	// Read stderr
	io.ReadAll(stderr)

	// Wait for completion
	err = wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	// Verify output
	expected := strings.Join(lines, "")
	if string(output) != expected {
		t.Errorf("Expected output %q, got %q", expected, string(output))
	}
}

// TestExecRunner_RunWithPipes_StderrCapture tests stderr capture
func TestExecRunner_RunWithPipes_StderrCapture(t *testing.T) {
	logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)
	runner, err := NewExec(Options{}, logger)
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}

	ctx := context.Background()

	// Use a shell command that writes to stderr
	var cmd string
	var args []string
	if runtime.GOOS == "windows" {
		cmd = "cmd"
		args = []string{"/c", "echo error message 1>&2"}
	} else {
		cmd = "sh"
		args = []string{"-c", "echo 'error message' >&2"}
	}

	stdin, stdout, stderr, wait, err := runner.RunWithPipes(ctx, cmd, args, nil, nil)
	if err != nil {
		t.Fatalf("RunWithPipes failed: %v", err)
	}

	stdin.Close()

	// Read from both stdout and stderr
	stdoutOutput, _ := io.ReadAll(stdout)
	stderrOutput, _ := io.ReadAll(stderr)

	// Wait for completion
	wait()

	// Verify stderr contains the error message
	if !strings.Contains(string(stderrOutput), "error message") {
		t.Errorf("Expected stderr to contain 'error message', got %q", string(stderrOutput))
	}

	// stdout should be empty
	if len(stdoutOutput) > 0 {
		t.Logf("Note: stdout was not empty: %q", string(stdoutOutput))
	}
}

// TestExecRunner_RunWithPipes_ContextCancellation tests context cancellation
func TestExecRunner_RunWithPipes_ContextCancellation(t *testing.T) {
	logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)
	runner, err := NewExec(Options{}, logger)
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Start a long-running command
	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "timeout"
	} else {
		cmd = "sleep"
	}

	stdin, stdout, stderr, wait, err := runner.RunWithPipes(ctx, cmd, []string{"60"}, nil, nil)
	if err != nil {
		t.Fatalf("RunWithPipes failed: %v", err)
	}

	// Cancel the context after a short delay
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Close pipes
	stdin.Close()
	io.ReadAll(stdout)
	io.ReadAll(stderr)

	// Wait should return context.Canceled
	err = wait()
	if err == nil {
		t.Error("Expected wait to return an error after context cancellation")
	}
}

// TestExecRunner_RunWithPipes_CommandNotFound tests error handling for non-existent command
func TestExecRunner_RunWithPipes_CommandNotFound(t *testing.T) {
	logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)
	runner, err := NewExec(Options{}, logger)
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}

	ctx := context.Background()

	// Try to run a command that doesn't exist
	_, _, _, _, err = runner.RunWithPipes(ctx, "nonexistent-command-12345", nil, nil, nil)
	if err == nil {
		t.Error("Expected error for non-existent command, got nil")
	}
}

// TestExecRunner_RunWithPipes_WithEnvironment tests environment variable passing
func TestExecRunner_RunWithPipes_WithEnvironment(t *testing.T) {
	logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)
	runner, err := NewExec(Options{}, logger)
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}

	ctx := context.Background()

	// Use a shell command to echo an environment variable
	var cmd string
	var args []string
	if runtime.GOOS == "windows" {
		cmd = "cmd"
		args = []string{"/c", "echo %TEST_VAR%"}
	} else {
		cmd = "sh"
		args = []string{"-c", "echo $TEST_VAR"}
	}

	env := []string{"TEST_VAR=test_value_123"}

	stdin, stdout, stderr, wait, err := runner.RunWithPipes(ctx, cmd, args, env, nil)
	if err != nil {
		t.Fatalf("RunWithPipes failed: %v", err)
	}

	stdin.Close()

	output, _ := io.ReadAll(stdout)
	io.ReadAll(stderr)

	err = wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	// Verify the environment variable was set
	if !strings.Contains(string(output), "test_value_123") {
		t.Errorf("Expected output to contain 'test_value_123', got %q", string(output))
	}
}

// TestExecRunner_RunWithPipes_CommandExitsEarly tests when command exits before stdin is closed
func TestExecRunner_RunWithPipes_CommandExitsEarly(t *testing.T) {
	logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)
	runner, err := NewExec(Options{}, logger)
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}

	ctx := context.Background()

	// Use 'true' command which exits immediately
	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "cmd"
	} else {
		cmd = "true"
	}

	stdin, stdout, stderr, wait, err := runner.RunWithPipes(ctx, cmd, nil, nil, nil)
	if err != nil {
		t.Fatalf("RunWithPipes failed: %v", err)
	}

	// Try to write after command might have exited
	time.Sleep(100 * time.Millisecond)

	// Writing might fail, but that's okay
	stdin.Write([]byte("test\n"))
	stdin.Close()

	io.ReadAll(stdout)
	io.ReadAll(stderr)

	// Wait should succeed (command exited with 0)
	err = wait()
	if err != nil {
		t.Logf("Note: wait returned error (this may be expected): %v", err)
	}
}

// TestExecRunner_RunWithPipes_ConcurrentReadWrite tests concurrent reading and writing
func TestExecRunner_RunWithPipes_ConcurrentReadWrite(t *testing.T) {
	logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)
	runner, err := NewExec(Options{}, logger)
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}

	ctx := context.Background()

	stdin, stdout, stderr, wait, err := runner.RunWithPipes(ctx, "cat", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunWithPipes failed: %v", err)
	}

	done := make(chan bool)
	var readOutput string

	// Read in a goroutine
	go func() {
		output, _ := io.ReadAll(stdout)
		readOutput = string(output)
		done <- true
	}()

	// Write some data
	testData := "concurrent test\n"
	stdin.Write([]byte(testData))
	stdin.Close()

	// Wait for read to complete
	<-done
	io.ReadAll(stderr)

	err = wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	if readOutput != testData {
		t.Errorf("Expected output %q, got %q", testData, readOutput)
	}
}
