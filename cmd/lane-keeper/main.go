// Package main provides the Lane-Keeper command-line interface.
package main

import (
	"fmt"
	"os"
)

const usageExitCode = 2

var version = "dev"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "version" {
		fmt.Println(version)
		return
	}

	fprintln(os.Stderr, "usage: lane-keeper <command>")
	os.Exit(usageExitCode)
}

func fprintln(file *os.File, message string) {
	_, _ = fmt.Fprintln(file, message)
}
