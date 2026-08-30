// Package output renders Lane-Keeper command results as human-readable text or JSON.
package output

import (
	"encoding/json"
	"fmt"
	"io"
)

const (
	// FormatText selects human-readable diagnostic output (the default).
	FormatText = "text"
	// FormatJSON selects the stable machine-readable JSON envelope.
	FormatJSON = "json"
)

// Result is the stable machine-readable envelope for Lane-Keeper command output.
type Result struct {
	Status   string         `json:"status"`
	Workflow string         `json:"workflow,omitempty"`
	Target   string         `json:"target,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
}

// ParseFormat validates a requested output format, defaulting to FormatText.
func ParseFormat(value string) (string, error) {
	switch value {
	case "":
		return FormatText, nil
	case FormatText, FormatJSON:
		return value, nil
	default:
		return "", fmt.Errorf("unknown output format %q, want %q or %q", value, FormatText, FormatJSON)
	}
}

// WriteJSON encodes result as a single trailing-newline-terminated JSON object.
func (result Result) WriteJSON(writer io.Writer) error {
	encoder := json.NewEncoder(writer)
	if err := encoder.Encode(result); err != nil {
		return fmt.Errorf("encode result: %w", err)
	}
	return nil
}
