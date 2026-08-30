package output_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/iilei/lane-keeper/internal/output"
)

const (
	testBaselineKey   = "baseline"
	testBaselineValue = "v1.42.0"
	testStatus        = "ready"
	testTarget        = "main"
	testWorkflow      = "deploy"
)

func TestParseFormatDefaultsToText(t *testing.T) {
	t.Parallel()

	format, err := output.ParseFormat("")
	if err != nil {
		t.Fatalf("ParseFormat() error = %v", err)
	}
	if format != output.FormatText {
		t.Errorf("ParseFormat(\"\") = %q, want %q", format, output.FormatText)
	}
}

func TestParseFormatAcceptsKnownValues(t *testing.T) {
	t.Parallel()

	for _, value := range []string{output.FormatText, output.FormatJSON} {
		format, err := output.ParseFormat(value)
		if err != nil {
			t.Fatalf("ParseFormat(%q) error = %v", value, err)
		}
		if format != value {
			t.Errorf("ParseFormat(%q) = %q, want %q", value, format, value)
		}
	}
}

func TestParseFormatRejectsUnknownValue(t *testing.T) {
	t.Parallel()

	if _, err := output.ParseFormat("xml"); err == nil {
		t.Fatal("ParseFormat(\"xml\") error = nil, want error")
	}
}

func TestResultWriteJSONEncodesStableEnvelope(t *testing.T) {
	t.Parallel()

	result := output.Result{
		Status:   testStatus,
		Workflow: testWorkflow,
		Target:   testTarget,
		Data:     map[string]any{testBaselineKey: testBaselineValue},
	}
	var buffer bytes.Buffer
	if err := result.WriteJSON(&buffer); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(buffer.Bytes(), &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	want := map[string]any{
		"status":   testStatus,
		"workflow": testWorkflow,
		"target":   testTarget,
		"data":     map[string]any{testBaselineKey: testBaselineValue},
	}
	for key := range want {
		if decoded[key] == nil {
			t.Errorf("decoded[%q] missing", key)
		}
	}
	if decoded["status"] != want["status"] || decoded["workflow"] != want["workflow"] ||
		decoded["target"] != want["target"] {
		t.Errorf("decoded = %#v, want %#v", decoded, want)
	}
}

func TestResultWriteJSONOmitsEmptyFields(t *testing.T) {
	t.Parallel()

	result := output.Result{Status: "ok"}
	var buffer bytes.Buffer
	if err := result.WriteJSON(&buffer); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
	if got, want := buffer.String(), "{\"status\":\"ok\"}\n"; got != want {
		t.Errorf("WriteJSON() = %q, want %q", got, want)
	}
}
