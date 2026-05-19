// Package walker implements the filesystem-based FileWalker port. It walks
// one or more root directories, returns every .go file that survives the
// include/exclude filters, and resolves "./..." style paths into directory
// trees.
package walker

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jedi-knights/kyber/internal/ports"
)

// FS implements ports.FileWalker over the real filesystem.
type FS struct{}

// New constructs a filesystem walker.
func New() *FS { return &FS{} }

// Walk resolves each root and returns the union of matching .go files.
// Paths like "./...", "./pkg/...", or trailing-slash directories are walked
// recursively; a path that names a file is included directly if it survives
// the filters.
func (w *FS) Walk(ctx context.Context, roots []string, opts ports.WalkOptions) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(opts.ExcludeGlobs) == 0 {
		opts.ExcludeGlobs = ports.DefaultExcludes()
	}

	seen := make(map[string]struct{})
	var out []string
	for _, root := range roots {
		files, err := walkOne(ctx, root, opts)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			if _, dup := seen[f]; dup {
				continue
			}
			seen[f] = struct{}{}
			out = append(out, f)
		}
	}
	return out, nil
}

func walkOne(ctx context.Context, root string, opts ports.WalkOptions) ([]string, error) {
	dir, recursive := resolveRoot(root)
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", root, err)
	}

	if !info.IsDir() {
		if shouldInclude(dir, dir, opts) {
			return []string{dir}, nil
		}
		return nil, nil
	}

	var out []string
	walkFn := makeWalkFn(ctx, dir, recursive, opts, &out)
	if err := filepath.WalkDir(dir, walkFn); err != nil {
		return nil, err
	}
	return out, nil
}

func makeWalkFn(ctx context.Context, dir string, recursive bool, opts ports.WalkOptions, out *[]string) fs.WalkDirFunc {
	return func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if d.IsDir() {
			return walkDirEntry(path, dir, recursive, opts)
		}
		if shouldInclude(path, dir, opts) {
			*out = append(*out, path)
		}
		return nil
	}
}

func walkDirEntry(path, dir string, recursive bool, opts ports.WalkOptions) error {
	if !recursive && path != dir {
		return filepath.SkipDir
	}
	if dirExcluded(path, dir, opts.ExcludeGlobs) {
		return filepath.SkipDir
	}
	return nil
}

// resolveRoot strips a trailing "/..." (or "..." alone) and returns the
// directory plus whether recursion is requested.
func resolveRoot(root string) (string, bool) {
	switch {
	case root == "./..." || root == "...":
		return ".", true
	case strings.HasSuffix(root, "/..."):
		return strings.TrimSuffix(root, "/..."), true
	default:
		return root, false
	}
}

func shouldInclude(path, base string, opts ports.WalkOptions) bool {
	if filepath.Ext(path) != ".go" {
		return false
	}
	if !opts.IncludeTests && strings.HasSuffix(path, "_test.go") {
		return false
	}
	rel, err := filepath.Rel(base, path)
	if err != nil {
		rel = path
	}
	for _, glob := range opts.ExcludeGlobs {
		if matchGlob(glob, rel) {
			return false
		}
	}
	return true
}

func dirExcluded(path, base string, globs []string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil || rel == "." {
		return false
	}
	for _, glob := range globs {
		if matchGlob(glob, rel) {
			return true
		}
	}
	return false
}

// matchGlob matches a glob with "**" support (any number of path components).
// The match is anchored at both ends, like filepath.Match.
func matchGlob(pattern, name string) bool {
	pattern = filepath.ToSlash(pattern)
	name = filepath.ToSlash(name)

	// Pattern "vendor/**" should also match the bare "vendor" directory entry
	// so WalkDir can skip it before descending.
	if prefix, ok := strings.CutSuffix(pattern, "/**"); ok {
		if name == prefix || strings.HasPrefix(name, prefix+"/") {
			return true
		}
	}

	return globMatch(pattern, name)
}

// globMatch implements a minimal glob with "**". Other metacharacters fall
// through to filepath.Match's semantics per path segment.
func globMatch(pattern, name string) bool {
	patParts := strings.Split(pattern, "/")
	nameParts := strings.Split(name, "/")
	return matchParts(patParts, nameParts)
}

func matchParts(pattern, name []string) bool {
	for len(pattern) > 0 {
		if pattern[0] == "**" {
			return matchDoubleStar(pattern[1:], name)
		}
		if !matchSegment(pattern[0], name) {
			return false
		}
		pattern = pattern[1:]
		name = name[1:]
	}
	return len(name) == 0
}

func matchDoubleStar(rest, name []string) bool {
	if len(rest) == 0 {
		return true
	}
	for i := 0; i <= len(name); i++ {
		if matchParts(rest, name[i:]) {
			return true
		}
	}
	return false
}

func matchSegment(pat string, name []string) bool {
	if len(name) == 0 {
		return false
	}
	ok, err := filepath.Match(pat, name[0])
	return err == nil && ok
}
