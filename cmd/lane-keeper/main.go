// Package main provides the Lane-Keeper command-line interface.
package main

import (
	"fmt"
	"os"
)

const usageExitCode = 2

var version string // Populated by GoReleaser via -ldflags

func main() {
	const minArgs = 2
	if len(os.Args) < minArgs {
		fprintln(os.Stderr, "usage: lane-keeper <command> [args...]")
		os.Exit(usageExitCode)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "--version", "-v", "version":
		fmt.Println(version)
		os.Exit(0)

	case "help", "--help", "-h":
		printUsage()

	case "config-check", "config-introspection":
		os.Exit(configCheck(args))

	case "readiness":
		os.Exit(readiness(args))

	case "branch":
		os.Exit(branch(args))

	case "mr":
		os.Exit(mr(args))

	default:
		fprintln(os.Stderr, "usage: lane-keeper <command>")
		os.Exit(usageExitCode)
	}
}

func printUsage() {
	fmt.Fprintln(
		os.Stderr,
		"lane-keeper is a small, read-only repository workflow tool intended to make readiness checks,",
	)
	fmt.Fprintln(
		os.Stderr,
		"awaiting readiness, branch naming, and merge-request message rendering consistent between local",
	)
	fmt.Fprintln(os.Stderr, "development and GitLab CI")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Usage: See https://github.com/iilei/lane-keeper")
}

func fprintln(file *os.File, message string) {
	_, _ = fmt.Fprintln(file, message)
}
