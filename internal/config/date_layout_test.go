package config_test

import (
	"testing"

	"github.com/iilei/lane-keeper/internal/config"
)

func TestPreviewDateLayoutsExtractsLaneKeeperDefaults(t *testing.T) {
	t.Parallel()

	content := `
[_.lane-keeper.defaults.template_date_formats]
wordsOnly = "release-day"
releaseStamp = "2006.01.02"
`
	previews, err := config.PreviewDateLayouts(content)
	if err != nil {
		t.Fatalf("PreviewDateLayouts() error = %v", err)
	}
	if got, want := len(previews), 2; got != want {
		t.Fatalf("len(previews) = %d, want %d", got, want)
	}
	if got, want := previews[0].Name, "releaseStamp"; got != want {
		t.Errorf("previews[0].Name = %q, want %q", got, want)
	}
	if got, want := previews[1].Rendered, "release-day"; got != want {
		t.Errorf("previews[1].Rendered = %q, want %q", got, want)
	}
}
