package template_test

import (
	"bytes"
	"testing"
	texttemplate "text/template"
	"time"

	"github.com/iilei/lane-keeper/internal/template"
)

const (
	releaseDayLayout   = "release-day"
	releaseStampLayout = "2006.01.02"
	releaseStampName   = "releaseStamp"
)

func TestDateSupportsBuiltInAndCustomLayouts(t *testing.T) {
	t.Parallel()

	functions, err := template.Functions(map[string]string{releaseStampName: releaseStampLayout})
	if err != nil {
		t.Fatalf("Functions() error = %v", err)
	}
	renderer, err := texttemplate.New("test").
		Funcs(functions).
		Parse(`{{ .commitAuthorDate | date "yyMMdd" }}-{{ .commitAuthorDate | date "releaseStamp" }}`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	var output bytes.Buffer
	context := (&template.Context{CommitAuthorDate: time.Date(2026, time.August, 30, 0, 0, 0, 0, time.UTC)}).Values()
	if err := renderer.Execute(&output, context); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got, want := output.String(), "260830-2026.08.30"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestDateRejectsCustomBuiltinName(t *testing.T) {
	t.Parallel()

	_, err := template.Functions(map[string]string{"yyMMdd": "20060102"})
	if err == nil {
		t.Fatal("Functions() error = nil, want built-in name conflict")
	}
}

func TestPreviewDateLayoutsRendersStableSortedExamples(t *testing.T) {
	t.Parallel()

	previews, err := template.PreviewDateLayouts(map[string]string{
		"wordsOnly":      releaseDayLayout,
		releaseStampName: releaseStampLayout,
	})
	if err != nil {
		t.Fatalf("PreviewDateLayouts() error = %v", err)
	}
	if got, want := len(previews), 2; got != want {
		t.Fatalf("len(previews) = %d, want %d", got, want)
	}
	if got, want := previews[0].Name, releaseStampName; got != want {
		t.Errorf("previews[0].Name = %q, want %q", got, want)
	}
	if got, want := previews[0].Rendered, releaseStampLayout; got != want {
		t.Errorf("previews[0].Rendered = %q, want %q", got, want)
	}
	if got, want := previews[1].Rendered, releaseDayLayout; got != want {
		t.Errorf("previews[1].Rendered = %q, want %q", got, want)
	}
}
