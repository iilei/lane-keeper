package template_test

import (
	"testing"
	"time"

	"github.com/iilei/lane-keeper/internal/template"
)

const (
	ticket       = "ABC-123"
	environment  = "staging"
	version      = "1.2.3"
	sha          = "a83d0219"
	shortSHA     = "a83d021"
	targetBranch = "main"
)

func TestContextValuesUseTemplateFieldNames(t *testing.T) {
	commitAuthorDate := time.Date(2026, time.August, 30, 6, 10, 0, 0, time.UTC)
	commitDate := time.Date(2026, time.August, 30, 6, 20, 0, 0, time.UTC)
	context := template.Context{
		Ticket:           ticket,
		Environment:      environment,
		Version:          version,
		SHA:              sha,
		ShortSHA:         shortSHA,
		TargetBranch:     targetBranch,
		CommitAuthorDate: commitAuthorDate,
		CommitDate:       commitDate,
	}

	values := context.Values()

	want := map[string]string{
		"ticket":       ticket,
		"environment":  environment,
		"version":      version,
		"sha":          sha,
		"shortSha":     shortSHA,
		"targetBranch": targetBranch,
	}
	for field, expected := range want {
		actual, ok := values[field].(string)
		if !ok || actual != expected {
			t.Errorf("Values()[%q] = %q, %t; want %q, true", field, actual, ok, expected)
		}
	}
	if got, ok := values["commitAuthorDate"].(time.Time); !ok || !got.Equal(commitAuthorDate) {
		t.Errorf("Values()[commitAuthorDate] = %v, %t; want %v, true", got, ok, commitAuthorDate)
	}
	if got, ok := values["commitDate"].(time.Time); !ok || !got.Equal(commitDate) {
		t.Errorf("Values()[commitDate] = %v, %t; want %v, true", got, ok, commitDate)
	}
}

func TestContextRenderRendersTemplateWithDateFunctions(t *testing.T) {
	t.Parallel()

	context := template.Context{
		Ticket:           ticket,
		ShortSHA:         shortSHA,
		CommitAuthorDate: time.Date(2026, time.August, 30, 0, 0, 0, 0, time.UTC),
	}
	rendered, err := context.Render(
		"branch",
		`{{ .ticket }}-{{ .commitAuthorDate | date "yyMMdd" }}-{{ .shortSha }}`,
		nil,
	)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if got, want := rendered, "ABC-123-260830-a83d021"; got != want {
		t.Errorf("Render() = %q, want %q", got, want)
	}
}

func TestContextRenderRejectsInvalidTemplate(t *testing.T) {
	t.Parallel()

	context := template.Context{}
	if _, err := context.Render("branch", "{{ .unknownField", nil); err == nil {
		t.Fatal("Render() error = nil, want parse error")
	}
}
