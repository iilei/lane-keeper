package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/iilei/lane-keeper/internal/config"
)

type configIntrospectionResult struct {
	Errors             []error
	DateLayoutPreviews []string
}

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
		result := introspectConfigFile(context.Background(), tomlPath, *format)
		for _, preview := range result.DateLayoutPreviews {
			fprintln(os.Stdout, preview)
		}
		if len(result.Errors) > 0 {
			allOK = false
			for _, err := range result.Errors {
				fprintln(os.Stderr, err.Error())
			}
		}
	}
	if !allOK {
		return 1
	}
	return 0
}

func introspectConfigFile(ctx context.Context, tomlPath string, format bool) configIntrospectionResult {
	content, err := os.ReadFile(tomlPath) //nolint:gosec // command accepts explicit user-provided TOML file paths
	if err != nil {
		return configIntrospectionResult{Errors: []error{fmt.Errorf("read %s: %w", tomlPath, err)}}
	}

	parseResult, err := config.Parse(string(content))
	if err != nil {
		return configIntrospectionResult{Errors: []error{fmt.Errorf("%s: %w", tomlPath, err)}}
	}
	result := configIntrospectionResult{}
	if parseResult.Found {
		for _, validationError := range parseResult.Model.Validate() {
			result.Errors = append(result.Errors, fmt.Errorf("%s: %w", tomlPath, validationError))
		}
	}
	if len(result.Errors) > 0 {
		return result
	}

	previews, err := config.PreviewDateLayouts(string(content))
	if err != nil {
		return configIntrospectionResult{Errors: []error{fmt.Errorf("%s: %w", tomlPath, err)}}
	}
	result.DateLayoutPreviews = make([]string, 0, len(previews))
	for _, preview := range previews {
		result.DateLayoutPreviews = append(result.DateLayoutPreviews, fmt.Sprintf(
			"%s: date layout %q (%q) renders as %q for Go reference time",
			tomlPath,
			preview.Name,
			preview.Layout,
			preview.Rendered,
		))
	}
	if !format {
		return result
	}

	formatResult := config.FormatPredicates(ctx, tomlPath, string(content))
	if len(formatResult.Errors) > 0 || !formatResult.Changed {
		result.Errors = formatResult.Errors
		return result
	}
	if err := writeFormattedConfig(tomlPath, formatResult.Content); err != nil {
		result.Errors = []error{fmt.Errorf("write %s: %w", tomlPath, err)}
	}
	return result
}

func writeFormattedConfig(tomlPath, content string) error {
	fileInfo, err := os.Stat(tomlPath)
	if err != nil {
		return err
	}
	if err := os.WriteFile(tomlPath, []byte(content), fileInfo.Mode()); err != nil {
		return err
	}
	return nil
}
