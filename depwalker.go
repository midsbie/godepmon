package main

import (
	"fmt"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Deps represents a slice of dependency file paths.
type Deps []string

// depWalker is used to walk the dependencies of a Go module, filtering dependencies based on
// whether they belong to the same module or include external dependencies.
type depWalker struct {
	module              string
	moduleWithSlash     string
	includeExternalDeps bool
}

// NewDepWalker creates a new dependency walker with the specified options.  It returns a *depWalker
// configured according to the provided parameters.
func NewDepWalker(includeExternalDeps bool) *depWalker {
	return &depWalker{
		includeExternalDeps: includeExternalDeps,
	}
}

// List generates a list of dependency file paths for a given directory path. It returns an error if
// the dependencies cannot be determined. If includeExternalDeps is false, only dependencies within
// the same module are included.
func (dw *depWalker) List(path string) (Deps, error) {
	if !dw.includeExternalDeps {
		if gomod, err := NewGoMod(path); err != nil {
			return nil, err
		} else if module, err := gomod.Module(); err != nil {
			return nil, err
		} else {
			dw.module = module
			dw.moduleWithSlash = module + "/"
		}
	}

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedImports | packages.NeedDeps,
		Dir:  path,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("failed to load packages: %s", err)
	}

	imports := make(map[string]*packages.Package)
	dw.visitAll(pkgs, imports)

	deps := []string{}
	for _, pkg := range imports {
		for _, f := range pkg.GoFiles {
			deps = append(deps, f)
		}
	}

	sort.Strings(deps)
	return deps, nil
}

// visitAll recursively visits all packages reachable from the initial set, adding them to the
// imports map if they meet the inclusion criteria defined by isCandidate.
func (dw *depWalker) visitAll(pkgs []*packages.Package, imports map[string]*packages.Package) {
	for _, pkg := range pkgs {
		if _, ok := imports[pkg.PkgPath]; ok {
			continue
		}

		if !dw.isCandidate(pkg.PkgPath) {
			continue
		}

		imports[pkg.PkgPath] = pkg

		pi := make([]*packages.Package, 0, len(pkg.Imports))
		for _, i := range pkg.Imports {
			pi = append(pi, i)
		}

		dw.visitAll(pi, imports)
	}
}

// isCandidate determines whether a package path should be considered for inclusion based on the
// DepWalker's configuration.
func (dw *depWalker) isCandidate(pkgPath string) bool {
	return dw.includeExternalDeps ||
		pkgPath == dw.module ||
		strings.HasPrefix(pkgPath, dw.moduleWithSlash)
}
