package config

import (
	"fmt"

	"github.com/pelletier/go-toml/v2"

	templateapi "github.com/iilei/lane-keeper/internal/template"
)

type dateLayoutDocument struct {
	Metadata struct {
		LaneKeeper struct {
			Defaults struct {
				TemplateDateFormats map[string]string `toml:"template_date_formats"`
			} `toml:"defaults"`
		} `toml:"lane-keeper"`
	} `toml:"_"`
}

// PreviewDateLayouts extracts, validates, and renders configured custom date layouts.
func PreviewDateLayouts(content string) ([]templateapi.DateLayoutPreview, error) {
	var document dateLayoutDocument
	if err := toml.Unmarshal([]byte(content), &document); err != nil {
		return nil, fmt.Errorf("toml parse error: %w", err)
	}

	previews, err := templateapi.PreviewDateLayouts(document.Metadata.LaneKeeper.Defaults.TemplateDateFormats)
	if err != nil {
		return nil, fmt.Errorf("template date formats: %w", err)
	}
	return previews, nil
}
