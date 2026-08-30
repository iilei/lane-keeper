package policy

import (
	"errors"
	"fmt"

	"github.com/canonical/starlark/starlark"
	"github.com/canonical/starlark/starlarkstruct"
)

const (
	diffFunctionName = "diff"
	diffFilesAttr    = "files"
	diffIsEmptyAttr  = "is_empty"
)

type diffValue struct {
	files   *starlark.List
	isEmpty starlark.Bool
}

func newGitValue(thread *starlark.Thread, inspector GitInspector) (starlark.Value, error) {
	return starlarkstruct.SafeFromStringDict(thread, starlark.String("git"), starlark.StringDict{
		"resolve":        gitBuiltin("git.resolve", inspector, resolveGitRef),
		"short_sha":      gitBuiltin("git.short_sha", inspector, shortGitSHA),
		"latest_tag":     gitBuiltin("git.latest_tag", inspector, latestGitTag),
		diffFunctionName: gitBuiltin("git.diff", inspector, diffGitRefs),
	})
}

func gitBuiltin(
	name string,
	inspector GitInspector,
	implementation func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple, GitInspector) (starlark.Value, error),
) *starlark.Builtin {
	return starlark.NewBuiltinWithSafety(
		name,
		requiredSafety,
		func(thread *starlark.Thread, function *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			return implementation(thread, function, args, kwargs, inspector)
		},
	)
}

func resolveGitRef(
	thread *starlark.Thread,
	function *starlark.Builtin,
	args starlark.Tuple,
	kwargs []starlark.Tuple,
	inspector GitInspector,
) (starlark.Value, error) {
	ref, err := unpackRef(function, args, kwargs)
	if err != nil {
		return nil, err
	}
	value, err := inspector.Resolve(thread.Context(), ref)
	if err != nil {
		return nil, err
	}
	return accountedString(thread, value)
}

func shortGitSHA(
	thread *starlark.Thread,
	function *starlark.Builtin,
	args starlark.Tuple,
	kwargs []starlark.Tuple,
	inspector GitInspector,
) (starlark.Value, error) {
	ref, err := unpackRef(function, args, kwargs)
	if err != nil {
		return nil, err
	}
	value, err := inspector.ShortSHA(thread.Context(), ref)
	if err != nil {
		return nil, err
	}
	return accountedString(thread, value)
}

func latestGitTag(
	thread *starlark.Thread,
	function *starlark.Builtin,
	args starlark.Tuple,
	kwargs []starlark.Tuple,
	inspector GitInspector,
) (starlark.Value, error) {
	ref, err := unpackRef(function, args, kwargs)
	if err != nil {
		return nil, err
	}
	result, err := inspector.LatestTag(thread.Context(), ref)
	if err != nil {
		return nil, err
	}
	if !result.Found {
		return starlark.None, nil
	}
	return accountedString(thread, result.Name)
}

func diffGitRefs(
	thread *starlark.Thread,
	function *starlark.Builtin,
	args starlark.Tuple,
	kwargs []starlark.Tuple,
	inspector GitInspector,
) (starlark.Value, error) {
	var fromRef, toRef string
	if err := starlark.UnpackArgs(function.Name(), args, kwargs, "from_ref", &fromRef, "to_ref", &toRef); err != nil {
		return nil, err
	}
	result, err := inspector.Diff(thread.Context(), fromRef, toRef)
	if err != nil {
		return nil, err
	}
	return newDiffValue(thread, result.Files)
}

func unpackRef(function *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (string, error) {
	var ref string
	if err := starlark.UnpackArgs(function.Name(), args, kwargs, "ref", &ref); err != nil {
		return "", err
	}
	return ref, nil
}

func accountedString(thread *starlark.Thread, value string) (starlark.Value, error) {
	result := starlark.String(value)
	if err := thread.AddSteps(starlark.SafeInt(len(value) + 1)); err != nil {
		return nil, err
	}
	if err := thread.AddAllocs(starlark.EstimateSize(result)); err != nil {
		return nil, err
	}
	return result, nil
}

func newDiffValue(thread *starlark.Thread, files []string) (starlark.Value, error) {
	allocation := starlark.SafeAdd(
		starlark.EstimateSize(&diffValue{}),
		starlark.EstimateMakeSize([]starlark.Value{}, starlark.SafeInt(len(files))),
	)
	steps := starlark.SafeInt(1)
	for _, file := range files {
		allocation = starlark.SafeAdd(allocation, starlark.EstimateSize(starlark.String(file)))
		steps = starlark.SafeAdd(steps, starlark.SafeInt(len(file)+1))
	}
	if err := thread.AddSteps(steps); err != nil {
		return nil, err
	}
	if err := thread.AddAllocs(allocation); err != nil {
		return nil, err
	}

	values := make([]starlark.Value, 0, len(files))
	for _, file := range files {
		values = append(values, starlark.String(file))
	}
	list := starlark.NewList(values)
	list.Freeze()
	return &diffValue{files: list, isEmpty: starlark.Bool(len(files) == 0)}, nil
}

func (value *diffValue) String() string               { return "diff" }
func (value *diffValue) Type() string                 { return "git.diff" }
func (value *diffValue) Freeze()                      { value.files.Freeze() }
func (value *diffValue) Truth() starlark.Bool         { return starlark.True }
func (value *diffValue) Safety() starlark.SafetyFlags { return requiredSafety }
func (value *diffValue) Hash() (uint32, error)        { return 0, errors.New("unhashable: git.diff") }

func (value *diffValue) AttrNames() []string { return []string{diffFilesAttr, diffIsEmptyAttr} }

func (value *diffValue) Attr(name string) (starlark.Value, error) {
	return value.attr(name)
}

func (value *diffValue) SafeAttr(thread *starlark.Thread, name string) (starlark.Value, error) {
	if err := starlark.CheckSafety(thread, requiredSafety); err != nil {
		return nil, err
	}
	if err := thread.AddSteps(starlark.SafeInt(1)); err != nil {
		return nil, err
	}
	return value.attr(name)
}

func (value *diffValue) attr(name string) (starlark.Value, error) {
	switch name {
	case diffFilesAttr:
		return value.files, nil
	case diffIsEmptyAttr:
		return value.isEmpty, nil
	default:
		return nil, starlark.NoSuchAttrError(fmt.Sprintf("git.diff has no attribute %q", name))
	}
}
