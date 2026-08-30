package template_test

import (
	"testing"

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
	context := template.Context{
		Ticket:       ticket,
		Environment:  environment,
		Version:      version,
		SHA:          sha,
		ShortSHA:     shortSHA,
		TargetBranch: targetBranch,
		YYMMDD:       "260830",
		HHmm:         "0610",
	}

	values := context.Values()

	want := map[string]string{
		"ticket":       ticket,
		"environment":  environment,
		"version":      version,
		"sha":          sha,
		"shortSha":     shortSHA,
		"targetBranch": targetBranch,
		"yyMMdd":       "260830",
		"HHmm":         "0610",
	}
	for field, expected := range want {
		actual, ok := values[field].(string)
		if !ok || actual != expected {
			t.Errorf("Values()[%q] = %q, %t; want %q, true", field, actual, ok, expected)
		}
	}
}
