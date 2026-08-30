package config_test

import (
	"strings"
	"testing"

	"github.com/iilei/lane-keeper/internal/config"
)

func TestModelValidateAcceptsValidSharedSource(t *testing.T) {
	t.Parallel()

	model := config.Model{
		Version: 1,
		Shared:  config.Shared{Source: "value = 1\n"},
	}
	for _, err := range model.Validate() {
		if strings.Contains(err.Error(), "shared") {
			t.Errorf("Validate() error = %v, want no shared error", err)
		}
	}
}

func TestModelValidateRejectsInvalidSharedSyntax(t *testing.T) {
	t.Parallel()

	model := config.Model{
		Version: 1,
		Shared:  config.Shared{Source: "def broken(:\n"},
	}
	var found bool
	for _, err := range model.Validate() {
		if strings.Contains(err.Error(), "shared") {
			found = true
		}
	}
	if !found {
		t.Fatal("Validate() did not report a shared error for invalid syntax")
	}
}
