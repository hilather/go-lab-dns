// Package goparse is a test-only replacement for the deprecated go/parser.ParseDir.
//
// It lives outside package testutil so the production binary does not import go/parser.
package goparse

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// Package mirrors the (deprecated) go/ast.Package shape that
// go/parser.ParseDir returned, without re-exposing the deprecated type.
type Package struct {
	Name  string
	Files map[string]*ast.File
}

// ParseDir parses every .go file in dir (no recursion, no build-tag
// filtering) and groups them by declared package name.
func ParseDir(fset *token.FileSet, dir string, mode parser.Mode) (map[string]*Package, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	pkgs := map[string]*Package{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		path := filepath.Join(dir, name)
		f, err := parser.ParseFile(fset, path, nil, mode)
		if err != nil {
			return nil, err
		}
		pkg, ok := pkgs[f.Name.Name]
		if !ok {
			pkg = &Package{Name: f.Name.Name, Files: map[string]*ast.File{}}
			pkgs[f.Name.Name] = pkg
		}
		pkg.Files[path] = f
	}
	return pkgs, nil
}
