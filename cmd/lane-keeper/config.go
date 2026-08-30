package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/iilei/lane-keeper/internal/config"
)

func configCheck(args []string) int {
	fs := flag.NewFlagSet("config-introspection", flag.ContinueOnError)
	fs.Usage = func() {
		fprintln(os.Stderr, "usage: lane-keeper config-introspection [--lint | --fmt] <toml_files...>")
	}
	lint := fs.Bool("lint", false, "Validate TOML and embedded Starlark syntax")
	format := fs.Bool("fmt", false, "Format embedded Starlark predicates in-place using Buildifier")

	if err := fs.Parse(args); err != nil {
		return usageExitCode
	}

	files := fs.Args()
	if len(files) == 0 {
		fs.Usage()
		return usageExitCode
	}
	if *lint && *format {
		fprintln(os.Stderr, "config-introspection: --lint and --fmt cannot be used together")
		return usageExitCode
	}

	allOK := true
	for _, tomlPath := range files {
		if errs := introspectConfigFile(context.Background(), tomlPath, *format); len(errs) > 0 {
			allOK = false
			for _, err := range errs {
				fprintln(os.Stderr, err.Error())
			}
		}
	}
	if !allOK {
		return 1
	}
	return 0
}

func introspectConfigFile(ctx context.Context, tomlPath string, format bool) []error {
	content, err := os.ReadFile(tomlPath) //nolint:gosec // command accepts explicit user-provided TOML file paths
	if err != nil {
		return []error{fmt.Errorf("read %s: %w", tomlPath, err)}
	}
	if !format {
		return config.CheckPredicatesInFile(tomlPath, string(content))
	}

	result := config.FormatPredicates(ctx, tomlPath, string(content))
	if len(result.Errors) > 0 || !result.Changed {
		return result.Errors
	}
	if err := writeFormattedConfig(tomlPath, result.Content); err != nil {
		return []error{fmt.Errorf("write %s: %w", tomlPath, err)}
	}
	return nil
}

func writeFormattedConfig(tomlPath, content string) error {
	fileInfo, err := os.Stat(tomlPath)
	if err != nil {
		return err
	}
	if err := os.WriteFile(
		tomlPath,
		[]byte(content),
		fileInfo.Mode(),
	); err != nil {
		return err
	}
	return nil
}
