package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/inercia/go-restricted-runner/pkg/common"
)

// Helper function to check if Landlock is available
func isLandlockAvailable() bool {
	if runtime.GOOS != "linux" {
		return false
	}

	// Try to create a minimal Landlock runner with unrestricted mode
	// This ensures we don't apply Landlock restrictions during the check
	logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)
	runner, err := NewLandrun(Options{
		"unrestricted_filesystem": true,
		"allow_networking":        true,
	}, logger)
	if err != nil {
		return false
	}

	return runner.CheckImplicitRequirements() == nil
}

func TestLandrun_CheckImplicitRequirements(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping landrun tests on non-Linux platform")
	}

	logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)
	// Use unrestricted mode to avoid applying Landlock during the check
	runner, err := NewLandrun(Options{
		"unrestricted_filesystem": true,
		"allow_networking":        true,
	}, logger)
	if err != nil {
		t.Fatalf("Failed to create Landrun runner: %v", err)
	}

	err = runner.CheckImplicitRequirements()
	if err != nil {
		t.Skipf("Landlock not available on this kernel: %v", err)
	}
}

func TestLandrun_Run_BasicCommand(t *testing.T) {
	if !isLandlockAvailable() {
		t.Skip("Landlock not available on this system")
	}

	logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)

	// Create runner with unrestricted filesystem to allow basic commands
	runner, err := NewLandrun(Options{
		"unrestricted_filesystem": true,
		"allow_networking":        true,
		"best_effort":             true,
	}, logger)
	if err != nil {
		t.Fatalf("Failed to create Landrun runner: %v", err)
	}

	ctx := context.Background()
	output, err := runner.Run(ctx, "sh", "echo 'Hello, Landlock!'", nil, nil, false)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	expected := "Hello, Landlock!"
	if output != expected {
		t.Errorf("Expected output %q, got %q", expected, output)
	}
}

func TestLandrun_Run_WithFilesystemRestrictions(t *testing.T) {
	if !isLandlockAvailable() {
		t.Skip("Landlock not available on this system")
	}

	logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)

	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "landrun-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create runner with read access to the temp directory
	runner, err := NewLandrun(Options{
		"allow_read_folders":      []string{tmpDir, "/usr", "/lib", "/lib64", "/bin"},
		"allow_read_exec_folders": []string{"/usr/bin", "/bin"},
		"best_effort":             true,
	}, logger)
	if err != nil {
		t.Fatalf("Failed to create Landrun runner: %v", err)
	}

	ctx := context.Background()

	// This should succeed - reading from allowed directory
	output, err := runner.Run(ctx, "sh", fmt.Sprintf("cat %s", testFile), nil, nil, false)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if !strings.Contains(output, "test content") {
		t.Errorf("Expected output to contain 'test content', got %q", output)
	}
}

func TestLandrun_Run_WithWriteRestrictions(t *testing.T) {
	if !isLandlockAvailable() {
		t.Skip("Landlock not available on this system")
	}

	logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)

	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "landrun-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	// Create runner with write access to the temp directory
	runner, err := NewLandrun(Options{
		"allow_write_folders":     []string{tmpDir},
		"allow_read_exec_folders": []string{"/usr/bin", "/bin", "/usr", "/lib", "/lib64"},
		"best_effort":             true,
	}, logger)
	if err != nil {
		t.Fatalf("Failed to create Landrun runner: %v", err)
	}

	ctx := context.Background()
	testFile := filepath.Join(tmpDir, "newfile.txt")

	// This should succeed - writing to allowed directory
	_, err = runner.Run(ctx, "sh", fmt.Sprintf("echo 'test' > %s", testFile), nil, nil, false)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify the file was created
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("Expected file to be created")
	}
}

func TestLandrun_Run_WithTemplateVariables(t *testing.T) {
	if !isLandlockAvailable() {
		t.Skip("Landlock not available on this system")
	}

	logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)

	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "landrun-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	// Create runner with template variable in path
	runner, err := NewLandrun(Options{
		"allow_read_folders":      []string{"{{.workdir}}", "/usr", "/lib", "/lib64"},
		"allow_read_exec_folders": []string{"/usr/bin", "/bin"},
		"best_effort":             true,
	}, logger)
	if err != nil {
		t.Fatalf("Failed to create Landrun runner: %v", err)
	}

	ctx := context.Background()
	params := map[string]interface{}{
		"workdir": tmpDir,
	}

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("template test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// This should succeed - template variable should be processed
	output, err := runner.Run(ctx, "sh", fmt.Sprintf("cat %s", testFile), nil, params, false)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if !strings.Contains(output, "template test") {
		t.Errorf("Expected output to contain 'template test', got %q", output)
	}
}

func TestLandrun_Run_WithEnvironmentVariables(t *testing.T) {
	if !isLandlockAvailable() {
		t.Skip("Landlock not available on this system")
	}

	logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)

	runner, err := NewLandrun(Options{
		"unrestricted_filesystem": true,
		"allow_networking":        true,
		"best_effort":             true,
	}, logger)
	if err != nil {
		t.Fatalf("Failed to create Landrun runner: %v", err)
	}

	ctx := context.Background()
	env := []string{"TEST_VAR=test_value"}

	output, err := runner.Run(ctx, "sh", "echo $TEST_VAR", env, nil, false)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if !strings.Contains(output, "test_value") {
		t.Errorf("Expected output to contain 'test_value', got %q", output)
	}
}

func TestLandrun_Run_ContextCancellation(t *testing.T) {
	if !isLandlockAvailable() {
		t.Skip("Landlock not available on this system")
	}

	logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)

	runner, err := NewLandrun(Options{
		"unrestricted_filesystem": true,
		"allow_networking":        true,
		"best_effort":             true,
	}, logger)
	if err != nil {
		t.Fatalf("Failed to create Landrun runner: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = runner.Run(ctx, "sh", "echo 'test'", nil, nil, false)
	if err == nil {
		t.Error("Expected error due to cancelled context")
	}
}

func TestLandrun_RunWithPipes_BasicEcho(t *testing.T) {
	if !isLandlockAvailable() {
		t.Skip("Landlock not available on this system")
	}

	logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)

	runner, err := NewLandrun(Options{
		"unrestricted_filesystem": true,
		"allow_networking":        true,
		"best_effort":             true,
	}, logger)
	if err != nil {
		t.Fatalf("Failed to create Landrun runner: %v", err)
	}

	ctx := context.Background()
	stdin, stdout, stderr, wait, err := runner.RunWithPipes(ctx, "cat", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunWithPipes failed: %v", err)
	}

	// Write to stdin
	testInput := "Hello from pipes!\n"
	if _, err := fmt.Fprint(stdin, testInput); err != nil {
		t.Fatalf("Failed to write to stdin: %v", err)
	}
	if err := stdin.Close(); err != nil {
		t.Fatalf("Failed to close stdin: %v", err)
	}

	// Read from stdout
	output, err := io.ReadAll(stdout)
	if err != nil {
		t.Fatalf("Failed to read from stdout: %v", err)
	}

	// Read from stderr (should be empty)
	_, _ = io.ReadAll(stderr)

	// Wait for command to complete
	if err := wait(); err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	if string(output) != testInput {
		t.Errorf("Expected output %q, got %q", testInput, string(output))
	}
}

func TestLandrun_RunWithPipes_MultipleWrites(t *testing.T) {
	if !isLandlockAvailable() {
		t.Skip("Landlock not available on this system")
	}

	logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)

	runner, err := NewLandrun(Options{
		"unrestricted_filesystem": true,
		"allow_networking":        true,
		"best_effort":             true,
	}, logger)
	if err != nil {
		t.Fatalf("Failed to create Landrun runner: %v", err)
	}

	ctx := context.Background()
	stdin, stdout, stderr, wait, err := runner.RunWithPipes(ctx, "cat", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunWithPipes failed: %v", err)
	}

	// Write multiple lines
	lines := []string{"line1\n", "line2\n", "line3\n"}
	for _, line := range lines {
		if _, err := fmt.Fprint(stdin, line); err != nil {
			t.Fatalf("Failed to write to stdin: %v", err)
		}
	}
	if err := stdin.Close(); err != nil {
		t.Fatalf("Failed to close stdin: %v", err)
	}

	// Read all output
	output, err := io.ReadAll(stdout)
	if err != nil {
		t.Fatalf("Failed to read from stdout: %v", err)
	}
	_, _ = io.ReadAll(stderr)

	if err := wait(); err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	expected := "line1\nline2\nline3\n"
	if string(output) != expected {
		t.Errorf("Expected output %q, got %q", expected, string(output))
	}
}

func TestLandrun_RunWithPipes_ContextCancellation(t *testing.T) {
	if !isLandlockAvailable() {
		t.Skip("Landlock not available on this system")
	}

	logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)

	runner, err := NewLandrun(Options{
		"unrestricted_filesystem": true,
		"allow_networking":        true,
		"best_effort":             true,
	}, logger)
	if err != nil {
		t.Fatalf("Failed to create Landrun runner: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	stdin, stdout, stderr, wait, err := runner.RunWithPipes(ctx, "sleep", []string{"10"}, nil, nil)
	if err != nil {
		t.Fatalf("RunWithPipes failed: %v", err)
	}

	// Don't write anything, just wait for context to cancel
	time.Sleep(200 * time.Millisecond)

	// Clean up pipes
	_ = stdin.Close()
	_, _ = io.ReadAll(stdout)
	_, _ = io.ReadAll(stderr)

	// Wait should return an error due to context cancellation
	err = wait()
	if err == nil {
		t.Error("Expected error due to context cancellation")
	}
}

func TestLandrun_BestEffortMode(t *testing.T) {
	if !isLandlockAvailable() {
		t.Skip("Landlock not available on this system")
	}

	logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)

	// Create runner with best effort mode
	runner, err := NewLandrun(Options{
		"allow_read_exec_folders": []string{"/usr/bin", "/bin", "/usr", "/lib", "/lib64"},
		"best_effort":             true,
	}, logger)
	if err != nil {
		t.Fatalf("Failed to create Landrun runner: %v", err)
	}

	ctx := context.Background()

	// This should work even on older kernels with best effort
	output, err := runner.Run(ctx, "sh", "echo 'best effort test'", nil, nil, false)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if !strings.Contains(output, "best effort test") {
		t.Errorf("Expected output to contain 'best effort test', got %q", output)
	}
}

// ============================================================================
// Unit Tests - Testing individual components
// ============================================================================

func TestNewLandrunOptions(t *testing.T) {
	tests := []struct {
		name    string
		options Options
		wantErr bool
	}{
		{
			name:    "empty options",
			options: Options{},
			wantErr: false,
		},
		{
			name: "valid filesystem options",
			options: Options{
				"allow_read_folders":      []string{"/usr", "/lib"},
				"allow_read_exec_folders": []string{"/usr/bin"},
				"allow_write_folders":     []string{"/tmp"},
			},
			wantErr: false,
		},
		{
			name: "valid network options",
			options: Options{
				"allow_bind_tcp":    []uint16{8080, 8443},
				"allow_connect_tcp": []uint16{80, 443},
			},
			wantErr: false,
		},
		{
			name: "best effort mode",
			options: Options{
				"best_effort": true,
			},
			wantErr: false,
		},
		{
			name: "unrestricted modes",
			options: Options{
				"unrestricted_filesystem": true,
				"allow_networking":        true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewLandrunOptions(tt.options)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewLandrunOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// Just verify it doesn't panic and returns without error
		})
	}
}

func TestLandrun_buildLandlockRules(t *testing.T) {
	if !isLandlockAvailable() {
		t.Skip("Landlock not available on this system")
	}

	logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)

	tests := []struct {
		name      string
		options   Options
		params    map[string]interface{}
		wantRules int // Expected minimum number of rules
		wantErr   bool
	}{
		{
			name: "filesystem rules only",
			options: Options{
				"allow_read_folders":  []string{"/usr", "/lib"},
				"allow_write_folders": []string{"/tmp"},
			},
			params:    nil,
			wantRules: 2,
			wantErr:   false,
		},
		{
			name: "with template variables",
			options: Options{
				"allow_read_folders": []string{"{{.workdir}}"},
			},
			params: map[string]interface{}{
				"workdir": "/home/user",
			},
			wantRules: 1,
			wantErr:   false,
		},
		{
			name: "network rules",
			options: Options{
				"allow_bind_tcp":    []uint16{8080},
				"allow_connect_tcp": []uint16{80, 443},
			},
			params:    nil,
			wantRules: 3,
			wantErr:   false,
		},
		{
			name: "unrestricted filesystem",
			options: Options{
				"unrestricted_filesystem": true,
			},
			params:    nil,
			wantRules: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner, err := NewLandrun(tt.options, logger)
			if err != nil {
				t.Fatalf("Failed to create runner: %v", err)
			}

			rules, err := runner.buildLandlockRules(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildLandlockRules() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(rules) < tt.wantRules {
				t.Errorf("buildLandlockRules() got %d rules, want at least %d", len(rules), tt.wantRules)
			}
		})
	}
}

// ============================================================================
// Integration Tests - Testing actual sandboxing behavior
// ============================================================================

func TestLandrun_Integration_FilesystemDenial(t *testing.T) {
	if !isLandlockAvailable() {
		t.Skip("Landlock not available on this system")
	}

	logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)

	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "landrun-integration-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	// Create a test file
	testFile := filepath.Join(tmpDir, "secret.txt")
	if err := os.WriteFile(testFile, []byte("secret data"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create runner that does NOT allow access to tmpDir
	runner, err := NewLandrun(Options{
		"allow_read_exec_folders": []string{"/usr/bin", "/bin", "/usr", "/lib", "/lib64"},
		"best_effort":             true,
	}, logger)
	if err != nil {
		t.Fatalf("Failed to create Landrun runner: %v", err)
	}

	ctx := context.Background()

	// This should fail - trying to read from restricted directory
	_, err = runner.Run(ctx, "sh", fmt.Sprintf("cat %s", testFile), nil, nil, false)
	if err == nil {
		t.Error("Expected error when accessing restricted file, but got none")
	}
}

func TestLandrun_Integration_WriteRestriction(t *testing.T) {
	if !isLandlockAvailable() {
		t.Skip("Landlock not available on this system")
	}

	logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)

	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "landrun-integration-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	// Create runner with read-only access to tmpDir
	runner, err := NewLandrun(Options{
		"allow_read_folders":      []string{tmpDir},
		"allow_read_exec_folders": []string{"/usr/bin", "/bin", "/usr", "/lib", "/lib64"},
		"best_effort":             true,
	}, logger)
	if err != nil {
		t.Fatalf("Failed to create Landrun runner: %v", err)
	}

	ctx := context.Background()
	testFile := filepath.Join(tmpDir, "should-fail.txt")

	// This should fail - trying to write to read-only directory
	_, err = runner.Run(ctx, "sh", fmt.Sprintf("echo 'test' > %s", testFile), nil, nil, false)
	if err == nil {
		t.Error("Expected error when writing to read-only directory, but got none")
	}

	// Verify file was not created
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("File should not have been created in read-only directory")
	}
}

func TestLandrun_Integration_ExecuteRestriction(t *testing.T) {
	if !isLandlockAvailable() {
		t.Skip("Landlock not available on this system")
	}

	logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)

	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "landrun-integration-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	// Create an executable script
	scriptFile := filepath.Join(tmpDir, "test.sh")
	scriptContent := "#!/bin/sh\necho 'executed'\n"
	if err := os.WriteFile(scriptFile, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create script: %v", err)
	}

	// Create runner with read access but no execute access to tmpDir
	runner, err := NewLandrun(Options{
		"allow_read_folders":      []string{tmpDir},
		"allow_read_exec_folders": []string{"/usr/bin", "/bin", "/usr", "/lib", "/lib64"},
		"best_effort":             true,
	}, logger)
	if err != nil {
		t.Fatalf("Failed to create Landrun runner: %v", err)
	}

	ctx := context.Background()

	// This should fail - trying to execute from non-executable directory
	_, err = runner.Run(ctx, "sh", scriptFile, nil, nil, false)
	if err == nil {
		t.Error("Expected error when executing from non-executable directory, but got none")
	}
}

func TestLandrun_Integration_MultipleRestrictions(t *testing.T) {
	if !isLandlockAvailable() {
		t.Skip("Landlock not available on this system")
	}

	logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)

	// Create temporary directories
	readDir, err := os.MkdirTemp("", "landrun-read-")
	if err != nil {
		t.Fatalf("Failed to create read dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(readDir)
	}()

	writeDir, err := os.MkdirTemp("", "landrun-write-")
	if err != nil {
		t.Fatalf("Failed to create write dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(writeDir)
	}()

	// Create test files
	readFile := filepath.Join(readDir, "read.txt")
	if err := os.WriteFile(readFile, []byte("readable"), 0644); err != nil {
		t.Fatalf("Failed to create read file: %v", err)
	}

	// Create runner with different permissions for different directories
	runner, err := NewLandrun(Options{
		"allow_read_folders":      []string{readDir},
		"allow_write_folders":     []string{writeDir},
		"allow_read_exec_folders": []string{"/usr/bin", "/bin", "/usr", "/lib", "/lib64"},
		"best_effort":             true,
	}, logger)
	if err != nil {
		t.Fatalf("Failed to create Landrun runner: %v", err)
	}

	ctx := context.Background()

	// Should succeed - reading from allowed directory
	output, err := runner.Run(ctx, "sh", fmt.Sprintf("cat %s", readFile), nil, nil, false)
	if err != nil {
		t.Errorf("Failed to read from allowed directory: %v", err)
	}
	if !strings.Contains(output, "readable") {
		t.Errorf("Expected output to contain 'readable', got %q", output)
	}

	// Should succeed - writing to allowed directory
	writeFile := filepath.Join(writeDir, "write.txt")
	_, err = runner.Run(ctx, "sh", fmt.Sprintf("echo 'writable' > %s", writeFile), nil, nil, false)
	if err != nil {
		t.Errorf("Failed to write to allowed directory: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(writeFile); os.IsNotExist(err) {
		t.Error("File should have been created in writable directory")
	}

	// Should fail - writing to read-only directory
	badFile := filepath.Join(readDir, "bad.txt")
	_, err = runner.Run(ctx, "sh", fmt.Sprintf("echo 'bad' > %s", badFile), nil, nil, false)
	if err == nil {
		t.Error("Expected error when writing to read-only directory")
	}
}

func TestLandrun_Integration_RunWithPipes_Restrictions(t *testing.T) {
	if !isLandlockAvailable() {
		t.Skip("Landlock not available on this system")
	}

	logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)

	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "landrun-pipes-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create runner with read access to tmpDir
	runner, err := NewLandrun(Options{
		"allow_read_folders":      []string{tmpDir, "/usr", "/lib", "/lib64"},
		"allow_read_exec_folders": []string{"/usr/bin", "/bin"},
		"best_effort":             true,
	}, logger)
	if err != nil {
		t.Fatalf("Failed to create Landrun runner: %v", err)
	}

	ctx := context.Background()

	// Use cat to read the file through pipes
	stdin, stdout, stderr, wait, err := runner.RunWithPipes(ctx, "cat", []string{testFile}, nil, nil)
	if err != nil {
		t.Fatalf("RunWithPipes failed: %v", err)
	}

	_ = stdin.Close()

	// Read output
	output, err := io.ReadAll(stdout)
	if err != nil {
		t.Fatalf("Failed to read stdout: %v", err)
	}
	_, _ = io.ReadAll(stderr)

	if err := wait(); err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	if !strings.Contains(string(output), "test content") {
		t.Errorf("Expected output to contain 'test content', got %q", string(output))
	}
}

func TestLandrun_Integration_ErrorHandling(t *testing.T) {
	if !isLandlockAvailable() {
		t.Skip("Landlock not available on this system")
	}

	logger, _ := common.NewLogger("", "", common.LogLevelDebug, false)

	tests := []struct {
		name        string
		options     Options
		command     string
		shouldError bool
		description string
	}{
		{
			name: "command not found",
			options: Options{
				"unrestricted_filesystem": true,
				"allow_networking":        true,
				"best_effort":             true,
			},
			command:     "nonexistent-command-xyz",
			shouldError: true,
			description: "should fail when command doesn't exist",
		},
		{
			name: "invalid shell syntax",
			options: Options{
				"unrestricted_filesystem": true,
				"allow_networking":        true,
				"best_effort":             true,
			},
			command:     "echo 'unclosed quote",
			shouldError: true,
			description: "should fail with invalid shell syntax",
		},
		{
			name: "successful command",
			options: Options{
				"unrestricted_filesystem": true,
				"allow_networking":        true,
				"best_effort":             true,
			},
			command:     "echo 'success'",
			shouldError: false,
			description: "should succeed with valid command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner, err := NewLandrun(tt.options, logger)
			if err != nil {
				t.Fatalf("Failed to create runner: %v", err)
			}

			ctx := context.Background()
			_, err = runner.Run(ctx, "sh", tt.command, nil, nil, false)

			if tt.shouldError && err == nil {
				t.Errorf("%s: expected error but got none", tt.description)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("%s: unexpected error: %v", tt.description, err)
			}
		})
	}
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkLandrun_Run_Unrestricted(b *testing.B) {
	if !isLandlockAvailable() {
		b.Skip("Landlock not available on this system")
	}

	logger, _ := common.NewLogger("", "", common.LogLevelError, false)
	runner, err := NewLandrun(Options{
		"unrestricted_filesystem": true,
		"allow_networking":        true,
		"best_effort":             true,
	}, logger)
	if err != nil {
		b.Fatalf("Failed to create runner: %v", err)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := runner.Run(ctx, "sh", "echo 'benchmark'", nil, nil, false)
		if err != nil {
			b.Fatalf("Run failed: %v", err)
		}
	}
}

func BenchmarkLandrun_Run_WithRestrictions(b *testing.B) {
	if !isLandlockAvailable() {
		b.Skip("Landlock not available on this system")
	}

	logger, _ := common.NewLogger("", "", common.LogLevelError, false)
	runner, err := NewLandrun(Options{
		"allow_read_exec_folders": []string{"/usr/bin", "/bin", "/usr", "/lib", "/lib64"},
		"best_effort":             true,
	}, logger)
	if err != nil {
		b.Fatalf("Failed to create runner: %v", err)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := runner.Run(ctx, "sh", "echo 'benchmark'", nil, nil, false)
		if err != nil {
			b.Fatalf("Run failed: %v", err)
		}
	}
}
