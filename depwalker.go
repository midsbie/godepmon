package main

import (
	"fmt"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

type Deps []string

type DepWalker struct {
	module              string
	moduleWithSlash     string
	includeExternalDeps bool
}

func (dw *DepWalker) List(path string) (Deps, error) {
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

func (dw *DepWalker) visitAll(pkgs []*packages.Package, imports map[string]*packages.Package) {
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

func (dw *DepWalker) isCandidate(pkgPath string) bool {
	return dw.includeExternalDeps ||
		pkgPath == dw.module ||
		strings.HasPrefix(pkgPath, dw.moduleWithSlash)
}
