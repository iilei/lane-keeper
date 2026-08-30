// Package config provides configuration parsing and validation for lane-keeper.
package config

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"slices"
	"strings"

	"github.com/canonical/starlark/syntax"
	"github.com/pelletier/go-toml/v2"
)

type (
	// Predicate represents a lane-keeper check predicate in TOML.
	Predicate struct {
		CheckName string // e.g., "target-has-baseline-and-benign-changes"
		Code      string // The Starlark source code
		StartPos  int    // Position of entire predicate = """ """ block
		EndPos    int
		CodeStart int // Position of code start (after opening """)
		CodeEnd   int // Position of code end (before closing """)
	}

	// FormatResult contains formatted TOML content and any errors found while formatting predicates.
	FormatResult struct {
		Content string
		Changed bool
		Errors  []error
	}
)

// ExtractPredicates finds all Starlark predicate blocks in TOML content.
func ExtractPredicates(tomlContent string) ([]Predicate, error) {
	// Match: predicate = """...""" non-greedy
	// Uses DOTALL mode to match across lines
	pattern := regexp.MustCompile(`(?s)\[_\.lane-keeper\.checks\.([^\]]+)\].*?predicate\s*=\s*"""(.*?)"""`)

	var predicates []Predicate
	matches := pattern.FindAllStringSubmatchIndex(tomlContent, -1)

	for _, match := range matches {
		// match[0:2] = entire block, [2:4] = check name, [4:6] = code
		checkName := tomlContent[match[2]:match[3]]
		codeStart := match[4]
		codeEnd := match[5]
		code := tomlContent[codeStart:codeEnd]

		predicates = append(predicates, Predicate{
			CheckName: checkName,
			Code:      code,
			StartPos:  match[0],
			EndPos:    match[1],
			CodeStart: codeStart,
			CodeEnd:   codeEnd,
		})
	}

	return predicates, nil
}

// ValidateStarlark checks whether code is syntactically valid Starlark without executing it.
func ValidateStarlark(code string) error {
	options := syntax.FileOptions{TopLevelControl: true}
	if _, err := options.Parse("predicate.star", code, 0); err != nil {
		return fmt.Errorf("starlark syntax error: %w", err)
	}
	return nil
}

// ValidateTOML checks if a string is valid TOML.
func ValidateTOML(content string) error {
	var data map[string]any
	err := toml.Unmarshal([]byte(content), &data)
	if err != nil {
		return fmt.Errorf("toml parse error: %w", err)
	}
	return nil
}

// CheckPredicatesInFile validates all Starlark predicates in a TOML file.
// Returns list of errors found (empty = success).
func CheckPredicatesInFile(tomlPath, content string) []error {
	predicates, err := ExtractPredicates(content)
	if err != nil {
		return []error{fmt.Errorf("extract predicates: %w", err)}
	}

	var errs []error

	for _, pred := range predicates {
		if err := ValidateStarlark(pred.Code); err != nil {
			errs = append(errs, fmt.Errorf("%s: check %q: %w",
				tomlPath, pred.CheckName, err))
		}
	}

	// Validate TOML is still well-formed
	if err := ValidateTOML(content); err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", tomlPath, err))
	}

	return errs
}

// FormatPredicates validates and formats predicates with an external Buildifier executable.
func FormatPredicates(ctx context.Context, tomlPath, content string) FormatResult {
	predicates, err := ExtractPredicates(content)
	if err != nil {
		return FormatResult{Content: content, Errors: []error{fmt.Errorf("%s: extract predicates: %w", tomlPath, err)}}
	}
	if err := ValidateTOML(content); err != nil {
		return FormatResult{Content: content, Errors: []error{fmt.Errorf("%s: %w", tomlPath, err)}}
	}

	formattedContent := content
	var errs []error
	for _, predicate := range slices.Backward(predicates) {
		if err := ValidateStarlark(predicate.Code); err != nil {
			errs = append(errs, fmt.Errorf("%s: check %q: %w", tomlPath, predicate.CheckName, err))
			continue
		}
		formatted, err := formatStarlark(ctx, predicate.Code)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: check %q: %w", tomlPath, predicate.CheckName, err))
			continue
		}
		formattedContent = formattedContent[:predicate.CodeStart] + embeddedCode(
			formatted,
		) + formattedContent[predicate.CodeEnd:]
	}
	if len(errs) > 0 {
		return FormatResult{Content: content, Errors: errs}
	}
	if err := ValidateTOML(formattedContent); err != nil {
		return FormatResult{Content: content, Errors: []error{fmt.Errorf("%s: formatted output: %w", tomlPath, err)}}
	}
	return FormatResult{Content: formattedContent, Changed: formattedContent != content}
}

func formatStarlark(ctx context.Context, code string) (string, error) {
	command := exec.CommandContext(ctx, "buildifier", "-type=default", "-mode=fix")
	command.Stdin = strings.NewReader(code)
	formatted, err := command.Output()
	if err == nil {
		return string(formatted), nil
	}
	if errors.Is(err, exec.ErrNotFound) {
		return "", errors.New("--fmt requires external tool \"buildifier\" on PATH")
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return "", fmt.Errorf("buildifier: %s", strings.TrimSpace(string(exitError.Stderr)))
	}
	return "", fmt.Errorf("run buildifier: %w", err)
}

func embeddedCode(code string) string {
	return "\n" + strings.TrimSuffix(code, "\n") + "\n"
}
