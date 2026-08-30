// Package template defines the public context available to repository templates.
package template

import "time"

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
	}
}
