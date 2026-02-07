package main

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/inercia/go-restricted-runner/pkg/common"
	"github.com/inercia/go-restricted-runner/pkg/runner"
)

func main() {
	// Create a logger
	logger, _ := common.NewLogger("", "", common.LogLevelInfo, false)

	// Create a runner
	r, err := runner.New(runner.TypeExec, runner.Options{}, logger)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// Example 1: Simple echo with cat
	fmt.Println("=== Example 1: Simple echo with cat ===")
	stdin, stdout, stderr, wait, err := r.RunWithPipes(ctx, "cat", nil, nil, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Send input
	fmt.Fprintln(stdin, "Hello from RunWithPipes!")
	fmt.Fprintln(stdin, "This is interactive communication.")
	stdin.Close()

	// Read output
	output, _ := io.ReadAll(stdout)
	io.ReadAll(stderr)

	// Wait for completion
	if err := wait(); err != nil {
		log.Printf("Process error: %v", err)
	}

	fmt.Print(string(output))

	// Example 2: Using environment variables
	fmt.Println("\n=== Example 2: Environment variables ===")
	stdin2, stdout2, stderr2, wait2, err := r.RunWithPipes(
		ctx,
		"sh",
		[]string{"-c", "echo \"TEST_VAR is: $TEST_VAR\""},
		[]string{"TEST_VAR=HelloWorld"},
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}

	stdin2.Close()
	output2, _ := io.ReadAll(stdout2)
	io.ReadAll(stderr2)
	wait2()

	fmt.Print(string(output2))

	fmt.Println("\n=== All examples completed successfully! ===")
}
