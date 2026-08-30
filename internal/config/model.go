package config

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"

	templateapi "github.com/iilei/lane-keeper/internal/template"
)

const (
	supportedSchemaVersion = 1
	builtinAwaitInterval   = 30 * time.Second
	builtinAwaitTimeout    = 30 * time.Minute
	maximumAwaitTimeout    = 24 * time.Hour

	// AwaitIntervalEnvironment overrides the configured polling interval.
	AwaitIntervalEnvironment = "LANE_KEEPER_AWAIT_INTERVAL"
	// AwaitTimeoutEnvironment overrides the configured total retry timeout.
	AwaitTimeoutEnvironment = "LANE_KEEPER_AWAIT_TIMEOUT"
	// AllowLongAwaitMaximumEnvironment raises the timeout ceiling using an integer number of seconds.
	AllowLongAwaitMaximumEnvironment = "LANE_KEEPER_UNSAFE_ALLOW_LONG_AWAIT_MAXIMUM"
)

type (
	// Model is the typed Lane-Keeper configuration beneath [_.lane-keeper].
	Model struct {
		Version   int                 `toml:"version"`
		Defaults  Defaults            `toml:"defaults"`
		Shared    Shared              `toml:"shared"`
		Checks    map[string]Check    `toml:"checks"`
		Templates map[string]Template `toml:"templates"`
		Workflows map[string]Workflow `toml:"workflows"`
	}

	// ParseResult distinguishes absent Lane-Keeper configuration from a parsed model.
	ParseResult struct {
		Model *Model
		Found bool
	}

	// Defaults contains repository-wide workflow defaults.
	Defaults struct {
		Remote              string            `toml:"remote"`
		AwaitInterval       string            `toml:"await_interval"`
		AwaitTimeout        string            `toml:"await_timeout"`
		TemplateDateFormats map[string]string `toml:"template_date_formats"`
	}

	// Shared contains Starlark source defining functions/data reusable across checks.
	// It MUST NOT reference the host API (workflow, input, git, succeed, fail).
	Shared struct {
		Source string `toml:"source"`
	}

	// Check contains one inline Starlark readiness predicate.
	Check struct {
		Description string `toml:"description"`
		Predicate   string `toml:"predicate"`
	}

	// Template contains either a scalar template or a structured message template.
	Template struct {
		Template string `toml:"template"`
		Title    string `toml:"title"`
		Body     string `toml:"body"`
	}

	// Workflow selects checks, templates, and target-branch resolution.
	Workflow struct {
		Description          string       `toml:"description"`
		Checks               []string     `toml:"checks"`
		Remote               string       `toml:"remote"`
		TargetBranch         TargetBranch `toml:"target_branch"`
		BranchTemplate       string       `toml:"branch_template"`
		MergeRequestTemplate string       `toml:"merge_request_template"`
		Await                Await        `toml:"await"`
	}

	// TargetBranch selects a built-in target-branch resolver.
	TargetBranch struct {
		Resolve string `toml:"resolve"`
		Value   string `toml:"value"`
	}

	// Await configures foreground readiness polling.
	Await struct {
		Interval string `toml:"interval"`
		Timeout  string `toml:"timeout"`
	}

	// AwaitSettings contains effective readiness polling durations.
	AwaitSettings struct {
		Interval time.Duration
		Timeout  time.Duration
	}
)

// Parse reads only the [_.lane-keeper] subtree and ignores unrelated Mise configuration.
func Parse(content string) (ParseResult, error) {
	var document map[string]any
	if err := toml.Unmarshal([]byte(content), &document); err != nil {
		return ParseResult{}, fmt.Errorf("toml parse error: %w", err)
	}

	metadata, ok := document["_"].(map[string]any)
	if !ok {
		return ParseResult{}, nil
	}
	laneKeeper, ok := metadata["lane-keeper"].(map[string]any)
	if !ok {
		return ParseResult{}, nil
	}

	subtree, err := toml.Marshal(laneKeeper)
	if err != nil {
		return ParseResult{}, fmt.Errorf("encode [_.lane-keeper]: %w", err)
	}

	var model Model
	decoder := toml.NewDecoder(strings.NewReader(string(subtree)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&model); err != nil {
		return ParseResult{}, fmt.Errorf("decode [_.lane-keeper]: %w", err)
	}
	return ParseResult{Model: &model, Found: true}, nil
}

// Validate returns all semantic configuration errors in deterministic order.
func (model *Model) Validate() []error {
	if model == nil {
		return nil
	}

	var errs []error
	if model.Version != supportedSchemaVersion {
		errs = append(errs, fmt.Errorf("version must be %d, got %d", supportedSchemaVersion, model.Version))
	}
	if err := validateInterval("defaults.await_interval", model.Defaults.AwaitInterval); err != nil {
		errs = append(errs, err)
	}
	if err := validateTimeout("defaults.await_timeout", model.Defaults.AwaitTimeout, maximumAwaitTimeout); err != nil {
		errs = append(errs, err)
	}
	if _, err := templateapi.Functions(model.Defaults.TemplateDateFormats); err != nil {
		errs = append(errs, fmt.Errorf("template date formats: %w", err))
	}
	if strings.TrimSpace(model.Shared.Source) != "" {
		if err := ValidateStarlark(model.Shared.Source); err != nil {
			errs = append(errs, fmt.Errorf("shared: %w", err))
		}
	}
	for _, name := range slices.Sorted(maps.Keys(model.Checks)) {
		if strings.TrimSpace(model.Checks[name].Predicate) == "" {
			errs = append(errs, fmt.Errorf("check %q: predicate is required", name))
		}
	}
	for _, name := range slices.Sorted(maps.Keys(model.Templates)) {
		errs = append(errs, validateTemplate(name, model.Templates[name])...)
	}
	for _, name := range slices.Sorted(maps.Keys(model.Workflows)) {
		workflow := model.Workflows[name]
		errs = append(errs, model.validateWorkflow(name, &workflow)...)
	}
	return errs
}

// ResolveAwaitSettings applies built-in, default, workflow, and environment precedence.
func (model *Model) ResolveAwaitSettings(
	workflowName string,
	lookupEnv func(string) (string, bool),
) (AwaitSettings, error) {
	workflow, ok := model.Workflows[workflowName]
	if !ok {
		return AwaitSettings{}, fmt.Errorf("unknown workflow %q", workflowName)
	}

	intervalValue := firstNonEmpty(workflow.Await.Interval, model.Defaults.AwaitInterval)
	timeoutValue := firstNonEmpty(workflow.Await.Timeout, model.Defaults.AwaitTimeout)
	if value, found := lookupEnv(AwaitIntervalEnvironment); found {
		intervalValue = value
	}
	if value, found := lookupEnv(AwaitTimeoutEnvironment); found {
		timeoutValue = value
	}
	timeoutMaximum, err := resolveAwaitTimeoutMaximum(lookupEnv)
	if err != nil {
		return AwaitSettings{}, err
	}

	interval, err := parseInterval(AwaitIntervalEnvironment, intervalValue, builtinAwaitInterval)
	if err != nil {
		return AwaitSettings{}, err
	}
	timeout, err := parseTimeout(AwaitTimeoutEnvironment, timeoutValue, builtinAwaitTimeout, timeoutMaximum)
	if err != nil {
		return AwaitSettings{}, err
	}
	return AwaitSettings{Interval: interval, Timeout: timeout}, nil
}

func (model *Model) validateWorkflow(name string, workflow *Workflow) []error {
	errs := model.validateWorkflowChecks(name, workflow.Checks)
	if err := validateTargetBranch(name, workflow.TargetBranch); err != nil {
		errs = append(errs, err)
	}
	if workflow.Remote == "" && model.Defaults.Remote == "" {
		errs = append(errs, fmt.Errorf("workflow %q: remote is required", name))
	}
	if err := validateInterval(fmt.Sprintf("workflow %q await.interval", name), workflow.Await.Interval); err != nil {
		errs = append(errs, err)
	}
	if err := validateTimeout(
		fmt.Sprintf("workflow %q await.timeout", name),
		workflow.Await.Timeout,
		maximumAwaitTimeout,
	); err != nil {
		errs = append(errs, err)
	}
	return append(errs, model.validateWorkflowTemplates(name, workflow)...)
}

func (model *Model) validateWorkflowChecks(name string, checks []string) []error {
	if len(checks) == 0 {
		return []error{fmt.Errorf("workflow %q: checks must not be empty", name)}
	}

	var errs []error
	seenChecks := make(map[string]struct{}, len(checks))
	for _, checkName := range checks {
		if _, duplicate := seenChecks[checkName]; duplicate {
			errs = append(errs, fmt.Errorf("workflow %q: duplicate check %q", name, checkName))
			continue
		}
		seenChecks[checkName] = struct{}{}
		if _, ok := model.Checks[checkName]; !ok {
			errs = append(errs, fmt.Errorf("workflow %q: unknown check %q", name, checkName))
		}
	}
	return errs
}

func (model *Model) validateWorkflowTemplates(name string, workflow *Workflow) []error {
	var errs []error
	if workflow.BranchTemplate != "" {
		template, ok := model.Templates[workflow.BranchTemplate]
		if !ok || template.Template == "" {
			errs = append(errs, fmt.Errorf("workflow %q: unknown branch template %q", name, workflow.BranchTemplate))
		}
	}
	if workflow.MergeRequestTemplate != "" {
		template, ok := model.Templates[workflow.MergeRequestTemplate]
		if !ok || template.Title == "" || template.Body == "" {
			errs = append(
				errs,
				fmt.Errorf("workflow %q: unknown merge-request template %q", name, workflow.MergeRequestTemplate),
			)
		}
	}
	return errs
}

func validateTargetBranch(workflowName string, target TargetBranch) error {
	switch target.Resolve {
	case "literal":
		if strings.TrimSpace(target.Value) == "" {
			return fmt.Errorf(
				"workflow %q: target_branch.value is required for resolver %q",
				workflowName,
				target.Resolve,
			)
		}
	case "git-remote-head":
		if target.Value != "" {
			return fmt.Errorf(
				"workflow %q: target_branch.value is forbidden for resolver %q",
				workflowName,
				target.Resolve,
			)
		}
	case "":
		return fmt.Errorf("workflow %q: target_branch.resolve is required", workflowName)
	default:
		return fmt.Errorf("workflow %q: unknown target_branch resolver %q", workflowName, target.Resolve)
	}
	return nil
}

func validateTemplate(name string, template Template) []error {
	hasScalar := template.Template != ""
	hasMessage := template.Title != "" || template.Body != ""
	if hasScalar == hasMessage {
		return []error{fmt.Errorf("template %q: configure either template or title and body", name)}
	}
	if hasMessage && (template.Title == "" || template.Body == "") {
		return []error{fmt.Errorf("template %q: title and body are both required", name)}
	}
	return nil
}

func validateInterval(field, value string) error {
	if value == "" {
		return nil
	}
	_, err := parseInterval(field, value, builtinAwaitInterval)
	return err
}

func validateTimeout(field, value string, maximum time.Duration) error {
	if value == "" {
		return nil
	}
	_, err := parseTimeout(field, value, builtinAwaitTimeout, maximum)
	return err
}

func parseInterval(field, value string, fallback time.Duration) (time.Duration, error) {
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid duration %q: %w", field, value, err)
	}
	if duration <= 0 {
		return 0, errors.New(field + ": duration must be positive")
	}
	return duration, nil
}

func parseTimeout(field, value string, fallback, maximum time.Duration) (time.Duration, error) {
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid duration %q: %w", field, value, err)
	}
	if duration < 0 {
		return 0, errors.New(field + ": duration must not be negative")
	}
	if duration > maximum {
		return 0, fmt.Errorf("%s: duration must not exceed %s", field, maximum)
	}
	return duration, nil
}

func resolveAwaitTimeoutMaximum(lookupEnv func(string) (string, bool)) (time.Duration, error) {
	value, found := lookupEnv(AllowLongAwaitMaximumEnvironment)
	if !found {
		return maximumAwaitTimeout, nil
	}
	seconds, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf(
			"%s must be a positive integer number of seconds: %w",
			AllowLongAwaitMaximumEnvironment,
			err,
		)
	}
	if seconds <= uint64(maximumAwaitTimeout/time.Second) {
		return 0, fmt.Errorf(
			"%s must exceed %d seconds",
			AllowLongAwaitMaximumEnvironment,
			maximumAwaitTimeout/time.Second,
		)
	}
	maximum, err := time.ParseDuration(value + "s")
	if err != nil {
		return 0, fmt.Errorf("%s is not representable as a Go duration: %w", AllowLongAwaitMaximumEnvironment, err)
	}
	return maximum, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
