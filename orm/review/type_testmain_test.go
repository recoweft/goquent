package review

import (
	"context"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/tools/go/packages"
)

func TestNativeTestExecutableRoles(t *testing.T) {
	t.Setenv("GOPACKAGESDRIVER", "off")
	root := t.TempDir()
	for p, s := range map[string]string{"go.mod": "module fixture.example/roles\ngo 1.25.0\n", "a.go": "package fixture\n", "a_test.go": "package fixture\nimport \"testing\"\nfunc TestOne(t *testing.T) {}\n", "external_test.go": "package fixture_test\nimport \"testing\"\nfunc TestExternal(t *testing.T) {}\n"} {
		if os.WriteFile(filepath.Join(root, p), []byte(s), 0600) != nil {
			t.Fatal("fixture write")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cfg := &packages.Config{Context: ctx, Dir: root, Mode: packages.LoadSyntax | packages.NeedForTest, Tests: true}
	loaded, e := packages.Load(cfg, ".")
	if e != nil {
		t.Fatal("syntax load")
	}
	cfg.Mode = packages.LoadImports | packages.NeedDeps | packages.NeedForTest
	meta, e := packages.Load(cfg, ".")
	if e != nil {
		t.Fatal("metadata load")
	}
	graph := map[string]*packages.Package{}
	incoming := map[string]bool{}
	var visit func(*packages.Package)
	visit = func(p *packages.Package) {
		if graph[p.ID] != nil {
			return
		}
		graph[p.ID] = p
		for _, q := range p.Imports {
			incoming[q.ID] = true
			visit(q)
		}
	}
	for _, p := range meta {
		visit(p)
	}
	var candidate *packages.Package
	for _, p := range loaded {
		if unusedTestExecutable(p, graph, incoming, root) {
			if candidate != nil {
				t.Fatal("multiple candidates")
			}
			candidate = p
		}
	}
	if candidate == nil {
		t.Fatal("native executable not recognized")
	}
	file := candidate.CompiledGoFiles[0]
	original := graph["fixture.example/roles"]
	cases := map[string]func(*packages.Package){
		"ordinary_name":      func(p *packages.Package) { p.Name = "fixture" },
		"ordinary_identity":  func(p *packages.Package) { p.ID = "fixture.example/main.test"; p.PkgPath = p.ID },
		"test_variant":       func(p *packages.Package) { p.ForTest = original.ID },
		"missing_types":      func(p *packages.Package) { p.Types = nil },
		"missing_info":       func(p *packages.Package) { p.TypesInfo = nil },
		"missing_syntax":     func(p *packages.Package) { p.Syntax = nil },
		"missing_files":      func(p *packages.Package) { p.CompiledGoFiles = nil },
		"inconsistent_files": func(p *packages.Package) { p.GoFiles = []string{filepath.Join(root, "_testmain.go")} },
		"multiple_files":     func(p *packages.Package) { p.CompiledGoFiles = append([]string{file}, file) },
		"ill_typed":          func(p *packages.Package) { p.IllTyped = true },
		"package_errors":     func(p *packages.Package) { p.Errors = []packages.Error{{Msg: "PRIVATE_ERROR"}} },
		"missing_imports":    func(p *packages.Package) { p.Imports = nil },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			p := *candidate
			change(&p)
			if unusedTestExecutable(&p, graph, incoming, root) {
				t.Fatal("unknown role accepted")
			}
		})
	}
	incoming[candidate.ID] = true
	if unusedTestExecutable(candidate, graph, incoming, root) {
		t.Fatal("incoming dependency accepted")
	}
	delete(incoming, candidate.ID)
	roles := newTestMainRoles()
	roles.add(loaded, meta, root)
	parsed := map[string][]byte{file: []byte("captured")}
	if !roles.unused(nil, nil, parsed)[file] {
		t.Fatal("native nonuse proof lost")
	}
	if roles.unused(map[string]*typedFile{file: {}}, nil, parsed)[file] {
		t.Fatal("explicit source excluded")
	}
	if roles.unused(nil, parsed, parsed)[file] {
		t.Fatal("overlay source excluded")
	}
	if roles.unused(nil, nil, nil)[file] {
		t.Fatal("uncaptured source excluded")
	}
	consumer := types.NewPackage("fixture.example/consumer", "consumer")
	consumer.SetImports([]*types.Package{candidate.Types})
	for name, tf := range map[string]*typedFile{
		"ast": {file: candidate.Syntax[0]}, "info": {info: candidate.TypesInfo}, "types": {pkg: candidate.Types}, "transitive": {pkg: consumer},
		"helper": {helpers: map[*types.Func]*ast.FuncDecl{types.NewFunc(0, candidate.Types, "helper", nil): nil}},
	} {
		t.Run(name, func(t *testing.T) {
			if roles.unused(map[string]*typedFile{"consumer.go": tf}, nil, parsed)[file] {
				t.Fatal("consumer contamination accepted")
			}
		})
	}
	// A dependency's ordinary file role vetoes a synthetic role, across loads too.
	original.GoFiles = append(original.GoFiles, file)
	shared := newTestMainRoles()
	shared.add(loaded, meta, root)
	if shared.unused(nil, nil, parsed)[file] {
		t.Fatal("shared file excluded")
	}
	roles.retained[file] = true
	if roles.unused(nil, nil, parsed)[file] {
		t.Fatal("cross-root shared role excluded")
	}
	roles.retained[file] = false
	roles.valid = false
	if roles.unused(nil, nil, parsed)[file] {
		t.Fatal("invalid metadata accepted")
	}
}

func TestNativeDriverBoundary(t *testing.T) {
	t.Setenv("GOPACKAGESDRIVER", "off")
	if !nativePackageDriver() {
		t.Fatal("native disabled")
	}
	t.Setenv("GOPACKAGESDRIVER", "PRIVATE_UNKNOWN_DRIVER")
	if nativePackageDriver() {
		t.Fatal("unknown driver accepted")
	}
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("GOPACKAGESDRIVER", "")
	if !nativePackageDriver() {
		t.Fatal("absent driver")
	}
	if os.WriteFile(filepath.Join(dir, "gopackagesdriver"), []byte("#!/bin/sh\nexit 1\n"), 0700) != nil {
		t.Fatal("write")
	}
	if nativePackageDriver() {
		t.Fatal("implicit external driver accepted")
	}
}
