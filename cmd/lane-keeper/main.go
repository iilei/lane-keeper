// Package main provides the Lane-Keeper command-line interface.
package main

import (
	"fmt"
	"os"
)

const usageExitCode = 2

var version = "dev"

func main() {
	const minArgs = 2
	if len(os.Args) < minArgs {
		fprintln(os.Stderr, "usage: lane-keeper <command> [args...]")
		os.Exit(usageExitCode)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "version":
		fmt.Println(version)
		os.Exit(0)

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

func fprintln(file *os.File, message string) {
	_, _ = fmt.Fprintln(file, message)
}
