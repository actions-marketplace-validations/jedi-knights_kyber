// Package parser is the Go AST adapter for kyber. It reads .go files using
// go/parser, builds a domain.Package per source directory, and returns one
// domain.Function per top-level FuncDecl. This is the only file in kyber
// that touches go/parser directly; everything downstream works on the
// already-parsed AST.
package parser

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/jedi-knights/kyber/internal/domain"
)

// GoAST implements ports.SourceParser by parsing each input file with go/parser.
type GoAST struct{}

// New constructs a Go AST parser.
func New() *GoAST { return &GoAST{} }

// ParseFiles parses the given file paths. Files in the same directory share a
// domain.Package value, so metrics can resolve named interfaces and globals
// across sibling files without re-parsing.
func (p *GoAST) ParseFiles(ctx context.Context, paths []string) ([]*domain.Function, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	byDir, err := groupByDir(paths)
	if err != nil {
		return nil, err
	}

	var out []*domain.Function
	for dir, dirPaths := range byDir {
		funcs, err := p.parseDir(ctx, dir, dirPaths)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", dir, err)
		}
		out = append(out, funcs...)
	}
	return out, nil
}

func (p *GoAST) parseDir(ctx context.Context, dir string, paths []string) ([]*domain.Function, error) {
	fset := token.NewFileSet()
	files := make([]*ast.File, 0, len(paths))
	sourceByFile := make(map[string][]byte, len(paths))

	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}
		files = append(files, file)
		sourceByFile[path] = src
	}

	pkg := &domain.Package{
		Name:       packageName(files),
		ImportPath: dir,
		Files:      files,
		FileSet:    fset,
		Interfaces: extractInterfaces(files),
		Globals:    extractGlobals(files),
	}

	var out []*domain.Function
	for _, file := range files {
		path := fset.Position(file.Pos()).Filename
		src := sourceByFile[path]
		lines := splitLines(src)
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			out = append(out, buildFunction(fn, path, fset, pkg, lines))
		}
	}
	return out, nil
}

func buildFunction(fn *ast.FuncDecl, path string, fset *token.FileSet, pkg *domain.Package, lines []string) *domain.Function {
	recv := receiverName(fn)
	name := fn.Name.Name
	if recv != "" {
		name = recv + "." + name
	}
	doc := ""
	if fn.Doc != nil {
		doc = strings.TrimSpace(fn.Doc.Text())
	}
	start := fset.Position(fn.Pos()).Line
	end := fset.Position(fn.End()).Line
	body := []string{}
	if start >= 1 && end <= len(lines) {
		body = lines[start-1 : end]
	}
	return &domain.Function{
		Name:        name,
		Receiver:    recv,
		Package:     pkg,
		File:        path,
		FuncDecl:    fn,
		FileSet:     fset,
		SourceLines: body,
		Doc:         doc,
	}
}

func receiverName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	expr := fn.Recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

func extractInterfaces(files []*ast.File) map[string]*ast.InterfaceType {
	out := make(map[string]*ast.InterfaceType)
	for _, file := range files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if iface, ok := ts.Type.(*ast.InterfaceType); ok {
					out[ts.Name.Name] = iface
				}
			}
		}
	}
	return out
}

func extractGlobals(files []*ast.File) map[string]token.Pos {
	out := make(map[string]token.Pos)
	for _, file := range files {
		collectGlobalsFromFile(file, out)
	}
	return out
}

func collectGlobalsFromFile(file *ast.File, out map[string]token.Pos) {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || (gen.Tok != token.VAR && gen.Tok != token.CONST) {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, name := range vs.Names {
				out[name.Name] = name.Pos()
			}
		}
	}
}

func packageName(files []*ast.File) string {
	if len(files) == 0 {
		return ""
	}
	return files[0].Name.Name
}

func splitLines(src []byte) []string {
	return strings.Split(string(src), "\n")
}

func groupByDir(paths []string) (map[string][]string, error) {
	out := make(map[string][]string)
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, fmt.Errorf("resolving %s: %w", p, err)
		}
		dir := filepath.Dir(abs)
		out[dir] = append(out[dir], abs)
	}
	return out, nil
}
