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
