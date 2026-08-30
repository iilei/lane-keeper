// Package template defines the public context available to repository templates.
package template

import (
	"fmt"
	"strings"
	texttemplate "text/template"
	"time"
)

// Context contains values supplied while rendering repository templates.
// Its Values method deliberately maps Go field names to the lower-camel template API.
type Context struct {
	Ticket           string
	Environment      string
	Version          string
	SHA              string
	ShortSHA         string
	TargetBranch     string
	CommitAuthorDate time.Time
	CommitDate       time.Time
}

// Values returns the complete template-facing context.
func (context *Context) Values() map[string]any {
	return map[string]any{
		"ticket":           context.Ticket,
		"environment":      context.Environment,
		"version":          context.Version,
		"sha":              context.SHA,
		"shortSha":         context.ShortSHA,
		"targetBranch":     context.TargetBranch,
		"commitAuthorDate": context.CommitAuthorDate,
		"commitDate":       context.CommitDate,
	}
}

// Render parses and executes source as a Go template against context, using
// the given custom date layouts to extend the built-in "date" function.
func (context *Context) Render(name, source string, customDateLayouts map[string]string) (string, error) {
	functions, err := Functions(customDateLayouts)
	if err != nil {
		return "", err
	}
	renderer, err := texttemplate.New(name).Funcs(functions).Parse(source)
	if err != nil {
		return "", fmt.Errorf("parse template %q: %w", name, err)
	}
	var output strings.Builder
	if err := renderer.Execute(&output, context.Values()); err != nil {
		return "", fmt.Errorf("render template %q: %w", name, err)
	}
	return output.String(), nil
}
