package main

import (
	"context"
	"time"

	gitinspect "github.com/iilei/lane-keeper/internal/git"
)

// commitContext holds source-commit facts shared by branch and merge-request rendering.
type commitContext struct {
	sha        string
	shortSHA   string
	authorDate time.Time
	commitDate time.Time
}

func resolveCommitContext(ctx context.Context, inspector *gitinspect.Inspector) (commitContext, error) {
	sha, err := inspector.Resolve(ctx, "HEAD")
	if err != nil {
		return commitContext{}, err
	}
	shortSHA, err := inspector.ShortSHA(ctx, "HEAD")
	if err != nil {
		return commitContext{}, err
	}
	authorDate, err := inspector.AuthorDate(ctx, "HEAD")
	if err != nil {
		return commitContext{}, err
	}
	commitDate, err := inspector.CommitDate(ctx, "HEAD")
	if err != nil {
		return commitContext{}, err
	}
	return commitContext{sha: sha, shortSHA: shortSHA, authorDate: authorDate, commitDate: commitDate}, nil
}
