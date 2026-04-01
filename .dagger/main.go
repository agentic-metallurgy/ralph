// CI module for ralph-go that runs tests and generates coverage reports
//
// This module provides functions to run Go tests and generate coverage reports
// for the ralph-go project using Dagger.

package main

import (
	"context"
	"dagger/ralph-ci/internal/dagger"
)

type RalphCi struct{}

// goContainer returns a Go container with the source code mounted and ready for testing
func (m *RalphCi) goContainer(source *dagger.Directory) *dagger.Container {
	return dag.Container().
		From("golang:1.25.3-alpine").
		WithMountedDirectory("/src", source).
		WithWorkdir("/src")
}

// Test runs all tests in verbose mode
func (m *RalphCi) Test(ctx context.Context, source *dagger.Directory) (string, error) {
	return m.goContainer(source).
		WithExec([]string{"go", "test", "-v", "./tests"}).
		Stdout(ctx)
}

// TestWithCoverage runs all tests with coverage profiling and displays a coverage summary
func (m *RalphCi) TestWithCoverage(ctx context.Context, source *dagger.Directory) (string, error) {
	return m.goContainer(source).
		WithExec([]string{
			"go", "test", "-v",
			"-coverprofile=coverage.out",
			"-covermode=atomic",
			"-coverpkg=./internal/...,./cmd/...",
			"./tests",
		}).
		WithExec([]string{"go", "tool", "cover", "-func=coverage.out"}).
		Stdout(ctx)
}

// integrationContainer returns a Go container with Node.js and the Claude CLI installed.
// Runs as a non-root user because claude refuses --dangerously-skip-permissions as root.
func (m *RalphCi) integrationContainer(source *dagger.Directory) *dagger.Container {
	return dag.Container().
		From("golang:1.25.3-alpine").
		WithExec([]string{"apk", "add", "--no-cache", "nodejs", "npm", "shadow"}).
		WithExec([]string{"npm", "install", "-g", "@anthropic-ai/claude-code"}).
		WithExec([]string{"useradd", "-m", "-d", "/home/testuser", "testuser"}).
		WithEnvVariable("GOPATH", "/home/testuser/go").
		WithEnvVariable("GOCACHE", "/home/testuser/.cache/go-build").
		WithMountedDirectory("/src", source).
		WithWorkdir("/src").
		WithExec([]string{"chown", "-R", "testuser:testuser", "/src"}).
		WithUser("testuser")
}

// IntegrationTest runs integration tests that require the Claude CLI.
// These tests use a deliberately invalid API key to verify error handling —
// no real API key or credits are needed.
func (m *RalphCi) IntegrationTest(ctx context.Context, source *dagger.Directory) (string, error) {
	return m.integrationContainer(source).
		WithEnvVariable("ANTHROPIC_API_KEY", "sk-ant-test-invalid-key").
		WithExec([]string{
			"go", "test", "-v", "-tags=integration",
			"-timeout=120s",
			"-run", "TestIntegration",
			"./tests/",
		}).
		Stdout(ctx)
}
