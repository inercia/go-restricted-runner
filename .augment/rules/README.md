# Augment Rules for go-restricted-runner

This directory contains focused rules files that are automatically loaded based on the context of your work.

## Rules Files Overview

### 00-global.md
**Always loaded**
- Project overview and architecture
- Core development principles
- Code style and conventions
- Build and test commands

### 01-runner-interface.md
**Loaded when**: Working with the Runner interface, adding methods, or discussing interface design
- Runner interface design patterns
- How to add new methods to the interface
- Implementation requirements for all runners
- Run() vs RunWithPipes() decision guide

### 02-runwithpipes-implementation.md
**Loaded when**: Working with RunWithPipes, pipes, stdin/stdout/stderr, or interactive processes
- RunWithPipes implementation patterns for each runner
- Pipe creation and cleanup
- Resource management
- Testing patterns for RunWithPipes

### 03-testing-patterns.md
**Loaded when**: Writing tests, discussing test coverage, or debugging test failures
- Test file organization and naming
- Platform-specific test skipping
- Table-driven tests
- Testing RunWithPipes
- Common test pitfalls

### 04-docker-runner.md
**Loaded when**: Working with Docker runner, containers, or DockerOptions
- Docker runner implementation details
- Run() vs RunWithPipes() approaches
- Container lifecycle management
- Docker-specific options and configuration

### 05-sandbox-firejail.md
**Loaded when**: Working with SandboxExec, Firejail, isolation, or sandbox profiles
- SandboxExec (macOS) implementation
- Firejail (Linux) implementation
- Profile template processing
- Restriction enforcement

### 06-common-utilities.md
**Loaded when**: Working with logging, templates, or utility functions
- Logger usage and configuration
- Template processing with Sprig
- Prerequisite checking
- Common utility functions

### 07-error-handling.md
**Loaded when**: Discussing errors, cleanup, defer, or failure cases
- Error handling principles
- Resource cleanup patterns
- Context cancellation
- Logging errors appropriately

### 08-documentation.md
**Loaded when**: Writing documentation, godoc comments, or examples
- Godoc comment standards
- README.md structure
- Per-runner documentation
- Example code guidelines

### 09-platform-specific.md
**Loaded when**: Working with platform-specific code, Windows/macOS/Linux differences
- Platform detection and handling
- Shell differences across platforms
- Platform-specific runners
- Cross-platform testing

## How to Use These Rules

The Augment AI assistant automatically loads relevant rules based on your prompt. You don't need to reference them explicitly.

### Examples

**Prompt**: "Add a new method to the Runner interface"
- Loads: 00-global.md, 01-runner-interface.md

**Prompt**: "Fix the Docker container cleanup in RunWithPipes"
- Loads: 00-global.md, 02-runwithpipes-implementation.md, 04-docker-runner.md, 07-error-handling.md

**Prompt**: "Write tests for the new feature"
- Loads: 00-global.md, 03-testing-patterns.md

**Prompt**: "Update the README with examples"
- Loads: 00-global.md, 08-documentation.md

## Maintaining These Rules

When you make significant changes to the codebase:

1. **Update existing rules** if patterns change
2. **Add new sections** for new patterns discovered
3. **Create new rule files** if a new major component is added
4. **Keep examples current** with the actual codebase

### Adding a New Rule File

1. Create `NN-topic.md` with appropriate number
2. Add clear trigger description at the top
3. Include practical examples and patterns
4. Update this README with the new file

### Rule File Format

Each rule file should have:
```markdown
# Title

**Trigger**: If the user prompt mentions "keyword1", "keyword2", ...

## Section 1
Content with examples

## Section 2
More content

## Best Practices
Actionable guidance

## Common Pitfalls
What to avoid
```

## Quick Reference

### Key Patterns

**Adding a new Runner method**:
1. Update Runner interface (01-runner-interface.md)
2. Implement in all runners (02-runwithpipes-implementation.md)
3. Write comprehensive tests (03-testing-patterns.md)
4. Update documentation (08-documentation.md)

**Error handling**:
1. Descriptive errors with context (07-error-handling.md)
2. Clean up resources on errors
3. Log at appropriate levels
4. Test error cases

**Platform-specific code**:
1. Use runtime.GOOS for detection (09-platform-specific.md)
2. Skip tests on wrong platforms (03-testing-patterns.md)
3. Document platform requirements (08-documentation.md)

**Testing**:
1. Test success and failure cases (03-testing-patterns.md)
2. Clean up resources in tests
3. Use platform-appropriate commands
4. Aim for >80% coverage

## Recent Changes

### 2026-02-07: Initial Rules Creation
- Created comprehensive rules structure
- Documented RunWithPipes implementation patterns
- Added testing, error handling, and documentation guidelines
- Organized rules by topic for automatic context inclusion

## Contributing

When you discover new patterns or best practices:
1. Document them in the appropriate rule file
2. Include code examples
3. Explain the rationale
4. Update this README if adding new files

