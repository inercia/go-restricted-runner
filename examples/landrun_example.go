package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/inercia/go-restricted-runner/pkg/common"
	"github.com/inercia/go-restricted-runner/pkg/runner"
)

func main() {
	// Create a logger
	logger, err := common.NewLogger("", "", common.LogLevelInfo, false)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	fmt.Println("=== Landrun Runner Examples ===\n")

	// Example 1: Basic command with unrestricted access
	fmt.Println("Example 1: Basic command with unrestricted access")
	runBasicCommand(logger)

	// Example 2: Filesystem restrictions
	fmt.Println("\nExample 2: Filesystem restrictions")
	runWithFilesystemRestrictions(logger)

	// Example 3: Interactive process with RunWithPipes
	fmt.Println("\nExample 3: Interactive process with RunWithPipes")
	runInteractiveProcess(logger)

	// Example 4: Template variables
	fmt.Println("\nExample 4: Template variables in paths")
	runWithTemplateVariables(logger)
}

func runBasicCommand(logger *common.Logger) {
	// Create Landrun runner with unrestricted access
	r, err := runner.New(runner.TypeLandrun, runner.Options{
		"unrestricted_filesystem": true,
		"allow_networking":        true,
		"best_effort":             true,
	}, logger)
	if err != nil {
		log.Printf("Failed to create Landrun runner: %v (Landlock may not be available)", err)
		return
	}

	ctx := context.Background()
	output, err := r.Run(ctx, "sh", "echo 'Hello from Landrun!'", nil, nil, false)
	if err != nil {
		log.Printf("Run failed: %v", err)
		return
	}

	fmt.Printf("Output: %s\n", output)
}

func runWithFilesystemRestrictions(logger *common.Logger) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "landrun-example-")
	if err != nil {
		log.Printf("Failed to create temp dir: %v", err)
		return
	}
	defer os.RemoveAll(tmpDir)

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("This is a test file"), 0644); err != nil {
		log.Printf("Failed to create test file: %v", err)
		return
	}

	// Create Landrun runner with read access to temp directory
	r, err := runner.New(runner.TypeLandrun, runner.Options{
		"allow_read_folders":      []string{tmpDir, "/usr", "/lib", "/lib64"},
		"allow_read_exec_folders": []string{"/usr/bin", "/bin"},
		"best_effort":             true,
	}, logger)
	if err != nil {
		log.Printf("Failed to create Landrun runner: %v", err)
		return
	}

	ctx := context.Background()
	output, err := r.Run(ctx, "sh", fmt.Sprintf("cat %s", testFile), nil, nil, false)
	if err != nil {
		log.Printf("Run failed: %v", err)
		return
	}

	fmt.Printf("File contents: %s\n", output)
}

func runInteractiveProcess(logger *common.Logger) {
	// Create Landrun runner
	r, err := runner.New(runner.TypeLandrun, runner.Options{
		"unrestricted_filesystem": true,
		"allow_networking":        true,
		"best_effort":             true,
	}, logger)
	if err != nil {
		log.Printf("Failed to create Landrun runner: %v", err)
		return
	}

	ctx := context.Background()
	stdin, stdout, stderr, wait, err := r.RunWithPipes(ctx, "cat", nil, nil, nil)
	if err != nil {
		log.Printf("RunWithPipes failed: %v", err)
		return
	}

	// Write to stdin
	fmt.Fprintln(stdin, "Line 1")
	fmt.Fprintln(stdin, "Line 2")
	fmt.Fprintln(stdin, "Line 3")
	stdin.Close()

	// Read from stdout
	output, err := io.ReadAll(stdout)
	if err != nil {
		log.Printf("Failed to read from stdout: %v", err)
		return
	}

	// Read from stderr (should be empty)
	io.ReadAll(stderr)

	// Wait for command to complete
	if err := wait(); err != nil {
		log.Printf("Command failed: %v", err)
		return
	}

	fmt.Printf("Output from interactive process:\n%s", string(output))
}

func runWithTemplateVariables(logger *common.Logger) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "landrun-template-")
	if err != nil {
		log.Printf("Failed to create temp dir: %v", err)
		return
	}
	defer os.RemoveAll(tmpDir)

	// Create Landrun runner with template variable
	r, err := runner.New(runner.TypeLandrun, runner.Options{
		"allow_read_folders":      []string{"{{.workdir}}", "/usr", "/lib", "/lib64"},
		"allow_read_exec_folders": []string{"/usr/bin", "/bin"},
		"best_effort":             true,
	}, logger)
	if err != nil {
		log.Printf("Failed to create Landrun runner: %v", err)
		return
	}

	// Create a test file
	testFile := filepath.Join(tmpDir, "template-test.txt")
	if err := os.WriteFile(testFile, []byte("Template variable test"), 0644); err != nil {
		log.Printf("Failed to create test file: %v", err)
		return
	}

	ctx := context.Background()
	params := map[string]interface{}{
		"workdir": tmpDir,
	}

	output, err := r.Run(ctx, "sh", fmt.Sprintf("cat %s", testFile), nil, params, false)
	if err != nil {
		log.Printf("Run failed: %v", err)
		return
	}

	fmt.Printf("Output with template variable: %s\n", output)
	fmt.Printf("(workdir was: %s)\n", tmpDir)
}
