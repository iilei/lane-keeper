// Package workflow resolves configured Lane-Keeper workflows into execution context.
package workflow

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/iilei/lane-keeper/internal/config"
)

type (
	// RemoteHeadResolver resolves the default branch for a named Git remote.
	RemoteHeadResolver func(context.Context, string) (string, error)

	// NamedCheck preserves a configured check's workflow ordering and identity.
	NamedCheck struct {
		Name  string
		Check config.Check
	}

	// Resolved is the complete static context required to evaluate one workflow.
	Resolved struct {
		Name                     string
		Description              string
		Remote                   string
		TargetBranch             string
		Checks                   []NamedCheck
		SharedSource             string
		BranchTemplateName       string
		BranchTemplate           config.Template
		MergeRequestTemplateName string
		MergeRequestTemplate     config.Template
		TemplateDateFormats      map[string]string
		Await                    config.AwaitSettings
	}
)

// Resolve validates and resolves one configured workflow.
func Resolve(
	ctx context.Context,
	model *config.Model,
	name string,
	lookupEnv func(string) (string, bool),
	resolveRemoteHead RemoteHeadResolver,
) (Resolved, error) {
	if model == nil {
		return Resolved{}, fmt.Errorf("workflow %q: Lane-Keeper configuration is required", name)
	}
	if errs := model.Validate(); len(errs) > 0 {
		return Resolved{}, fmt.Errorf("invalid Lane-Keeper configuration: %w", errs[0])
	}

	configured, ok := model.Workflows[name]
	if !ok {
		return Resolved{}, fmt.Errorf("unknown workflow %q", name)
	}
	remote := configured.Remote
	if remote == "" {
		remote = model.Defaults.Remote
	}
	targetBranch, err := resolveTargetBranch(ctx, configured.TargetBranch, remote, resolveRemoteHead)
	if err != nil {
		return Resolved{}, fmt.Errorf("workflow %q: %w", name, err)
	}
	await, err := model.ResolveAwaitSettings(name, lookupEnv)
	if err != nil {
		return Resolved{}, fmt.Errorf("workflow %q: %w", name, err)
	}

	checks := make([]NamedCheck, 0, len(configured.Checks))
	for _, checkName := range configured.Checks {
		checks = append(checks, NamedCheck{Name: checkName, Check: model.Checks[checkName]})
	}
	return Resolved{
		Name:                     name,
		Description:              configured.Description,
		Remote:                   remote,
		TargetBranch:             targetBranch,
		Checks:                   checks,
		SharedSource:             model.Shared.Source,
		BranchTemplateName:       configured.BranchTemplate,
		BranchTemplate:           model.Templates[configured.BranchTemplate],
		MergeRequestTemplateName: configured.MergeRequestTemplate,
		MergeRequestTemplate:     model.Templates[configured.MergeRequestTemplate],
		TemplateDateFormats:      model.Defaults.TemplateDateFormats,
		Await:                    await,
	}, nil
}

// GitRemoteHead returns a resolver backed by git symbolic-ref in repositoryRoot.
func GitRemoteHead(repositoryRoot string) RemoteHeadResolver {
	return func(ctx context.Context, remote string) (string, error) {
		ref := "refs/remotes/" + remote + "/HEAD"
		//nolint:gosec // fixed Git operation; repository and ref are separate argv values, not shell input.
		command := exec.CommandContext(ctx, "git", "-C", repositoryRoot, "symbolic-ref", "--quiet", "--short", ref)
		output, err := command.Output()
		if err != nil {
			return "", fmt.Errorf("resolve remote %q HEAD: %w", remote, err)
		}
		shortRef := strings.TrimSpace(string(output))
		prefix := remote + "/"
		branch, found := strings.CutPrefix(shortRef, prefix)
		if !found || branch == "" {
			return "", fmt.Errorf("remote %q HEAD resolved to unexpected ref %q", remote, shortRef)
		}
		return branch, nil
	}
}

func resolveTargetBranch(
	ctx context.Context,
	target config.TargetBranch,
	remote string,
	resolveRemoteHead RemoteHeadResolver,
) (string, error) {
	switch target.Resolve {
	case "literal":
		return target.Value, nil
	case "git-remote-head":
		if resolveRemoteHead == nil {
			return "", fmt.Errorf("target branch resolver %q is unavailable", target.Resolve)
		}
		return resolveRemoteHead(ctx, remote)
	default:
		return "", fmt.Errorf("unknown target branch resolver %q", target.Resolve)
	}
}
