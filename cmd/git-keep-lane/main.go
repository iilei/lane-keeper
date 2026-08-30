// Package main provides the Git subcommand shim for Lane-Keeper.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

func main() {
	command := exec.CommandContext(context.Background(), "lane-keeper")
	command.Args = append(command.Args, os.Args[1:]...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	err := command.Run()
	if err == nil {
		return
	}

	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		os.Exit(exitError.ExitCode())
	}

	_, _ = fmt.Fprintf(os.Stderr, "git-keep-lane: %v\n", err)
	os.Exit(1)
}
