package template

import (
	"fmt"
	"maps"
	"slices"
	texttemplate "text/template"
	"time"
)

var builtinDateLayouts = map[string]string{
	"yyMMdd":   "060102",
	"yyyyMMdd": "20060102",
	"HHmm":     "1504",
	"isoDate":  "2006-01-02",
	"rfc3339":  time.RFC3339,
}

// DateLayoutPreview describes a custom date layout rendered with Go's reference time.
type DateLayoutPreview struct {
	Name     string
	Layout   string
	Rendered string
}

// Functions returns template functions with built-in and additive custom date layouts.
func Functions(customDateLayouts map[string]string) (texttemplate.FuncMap, error) {
	layouts, err := dateLayouts(customDateLayouts)
	if err != nil {
		return nil, err
	}

	return texttemplate.FuncMap{
		"date": func(name string, timestamp time.Time) (string, error) {
			layout, ok := layouts[name]
			if !ok {
				return "", fmt.Errorf("unknown date layout %q", name)
			}
			return timestamp.Format(layout), nil
		},
	}, nil
}

// PreviewDateLayouts validates and renders custom layouts with Go's reference time.
func PreviewDateLayouts(customDateLayouts map[string]string) ([]DateLayoutPreview, error) {
	if _, err := dateLayouts(customDateLayouts); err != nil {
		return nil, err
	}

	names := slices.Sorted(maps.Keys(customDateLayouts))
	const referenceZoneOffsetSeconds = -7 * 60 * 60
	referenceTime := time.Date(2006, time.January, 2, 15, 4, 5, 0, time.FixedZone("MST", referenceZoneOffsetSeconds))
	previews := make([]DateLayoutPreview, 0, len(names))
	for _, name := range names {
		layout := customDateLayouts[name]
		rendered := referenceTime.Format(layout)
		previews = append(previews, DateLayoutPreview{
			Name:     name,
			Layout:   layout,
			Rendered: rendered,
		})
	}
	return previews, nil
}

func dateLayouts(customDateLayouts map[string]string) (map[string]string, error) {
	layouts := maps.Clone(builtinDateLayouts)
	for name, layout := range customDateLayouts {
		if _, builtIn := layouts[name]; builtIn {
			return nil, fmt.Errorf("custom date layout %q conflicts with a built-in layout", name)
		}
		if layout == "" {
			return nil, fmt.Errorf("custom date layout %q is empty", name)
		}
		layouts[name] = layout
	}
	return layouts, nil
}
