// Package git provides read-only repository inspection for Lane-Keeper.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type (
	// Inspector executes read-only Git queries within one repository.
	Inspector struct {
		repositoryRoot string
	}

	// Diff describes changed paths between two resolved commits.
	Diff struct {
		Files   []string
		IsEmpty bool
	}

	// TagResult describes whether a reachable tag was found.
	TagResult struct {
		Name  string
		Found bool
	}
)

// NewInspector returns a read-only Git inspector rooted at repositoryRoot.
func NewInspector(repositoryRoot string) *Inspector {
	return &Inspector{repositoryRoot: repositoryRoot}
}

// Resolve returns the commit object ID addressed by ref.
func (inspector *Inspector) Resolve(ctx context.Context, ref string) (string, error) {
	output, err := inspector.output(ctx, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("resolve ref %q: %w", ref, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// ShortSHA returns Git's standard abbreviated object ID for ref.
func (inspector *Inspector) ShortSHA(ctx context.Context, ref string) (string, error) {
	sha, err := inspector.Resolve(ctx, ref)
	if err != nil {
		return "", err
	}
	output, err := inspector.output(ctx, "rev-parse", "--short", "--end-of-options", sha)
	if err != nil {
		return "", fmt.Errorf("abbreviate ref %q: %w", ref, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// LatestTag returns the newest reachable tag by creator date.
func (inspector *Inspector) LatestTag(ctx context.Context, ref string) (TagResult, error) {
	sha, err := inspector.Resolve(ctx, ref)
	if err != nil {
		return TagResult{}, err
	}
	output, err := inspector.output(
		ctx,
		"for-each-ref",
		"--count=1",
		"--sort=-creatordate",
		"--format=%(refname:short)",
		"--merged="+sha,
		"refs/tags",
	)
	if err != nil {
		return TagResult{}, fmt.Errorf("find latest tag reachable from %q: %w", ref, err)
	}
	tag := strings.TrimSpace(string(output))
	return TagResult{Name: tag, Found: tag != ""}, nil
}

// Diff returns changed file paths between fromRef and toRef.
func (inspector *Inspector) Diff(ctx context.Context, fromRef, toRef string) (Diff, error) {
	fromSHA, err := inspector.Resolve(ctx, fromRef)
	if err != nil {
		return Diff{}, err
	}
	toSHA, err := inspector.Resolve(ctx, toRef)
	if err != nil {
		return Diff{}, err
	}
	output, err := inspector.output(
		ctx,
		"diff",
		"--name-only",
		"--no-ext-diff",
		"--no-textconv",
		"--no-renames",
		"--no-relative",
		"-z",
		fromSHA,
		toSHA,
		"--",
	)
	if err != nil {
		return Diff{}, fmt.Errorf("diff %q to %q: %w", fromRef, toRef, err)
	}
	files := splitNUL(output)
	return Diff{Files: files, IsEmpty: len(files) == 0}, nil
}

func (inspector *Inspector) output(ctx context.Context, args ...string) ([]byte, error) {
	commandArgs := append([]string{"-C", inspector.repositoryRoot}, args...)
	//nolint:gosec // Git is fixed; repository, refs, and options are separate argv values without a shell.
	command := exec.CommandContext(ctx, "git", commandArgs...)
	output, err := command.Output()
	if err == nil {
		return output, nil
	}
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) {
		return nil, err
	}
	diagnostic := strings.TrimSpace(string(exitError.Stderr))
	if diagnostic == "" {
		return nil, err
	}
	return nil, fmt.Errorf("%s: %w", diagnostic, err)
}

func splitNUL(output []byte) []string {
	output = bytes.TrimSuffix(output, []byte{0})
	if len(output) == 0 {
		return nil
	}
	parts := bytes.Split(output, []byte{0})
	files := make([]string, 0, len(parts))
	for _, part := range parts {
		files = append(files, string(part))
	}
	return files
}
