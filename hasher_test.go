//
// Copyright (C) 2020 - 2025 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/scud
//

package scud

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, root, file, content string) {
	t.Helper()
	path := filepath.Join(root, file)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const coreGo = "package core\n\nimport _ \"embed\"\n\n//go:embed data.txt\nvar data string\n\n"

// temporary module outside of GOPATH with function depending on sub-package
func newTestModule(t *testing.T) string {
	t.Helper()
	t.Setenv("GOWORK", "off")
	t.Setenv("GOFLAGS", "-mod=mod")

	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/app\n\ngo 1.22\n")
	writeFile(t, root, "cmd/fn/main.go", "package main\n\nimport \"example.com/app/internal/core\"\n\nfunc main() { core.Run() }\n")
	writeFile(t, root, "internal/core/core.go", coreGo+"func Run() { _ = data }\n")
	writeFile(t, root, "internal/core/data.txt", "v1")
	writeFile(t, root, "internal/core/core_test.go", "package core\n")
	writeFile(t, root, "README.md", "readme")

	return root
}

func testHash(t *testing.T, dir string, gocc *GoCompiler) string {
	t.Helper()
	h := NewHasher(false)
	if gocc != nil {
		h = h.withCompiler(gocc)
	}
	h.dir = dir

	hash, err := h.Hash("example.com/app", "cmd/fn", "")
	if err != nil {
		t.Fatal(err)
	}
	return hash
}

func TestHasherStable(t *testing.T) {
	root := newTestModule(t)
	if a, b := testHash(t, root, nil), testHash(t, root, nil); a != b {
		t.Errorf("hash is not stable: %s != %s", a, b)
	}
}

func TestHasherLocationIndependent(t *testing.T) {
	a := testHash(t, newTestModule(t), nil)
	b := testHash(t, newTestModule(t), nil)
	if a != b {
		t.Errorf("hash depends on checkout location: %s != %s", a, b)
	}
}

func TestHasherInvalidation(t *testing.T) {
	for name, edit := range map[string]func(root string){
		"SubPackage": func(root string) {
			writeFile(t, root, "internal/core/core.go", coreGo+"func Run() { println(data) }\n")
		},
		"NewFile": func(root string) {
			writeFile(t, root, "internal/core/more.go", "package core\n\nconst X = 1\n")
		},
		"EmbedFile": func(root string) {
			writeFile(t, root, "internal/core/data.txt", "v2")
		},
		"Main": func(root string) {
			writeFile(t, root, "cmd/fn/main.go", "package main\n\nimport \"example.com/app/internal/core\"\n\nfunc main() { core.Run(); core.Run() }\n")
		},
	} {
		t.Run(name, func(t *testing.T) {
			root := newTestModule(t)
			before := testHash(t, root, nil)
			edit(root)
			after := testHash(t, root, nil)
			if before == after {
				t.Errorf("hash is not changed")
			}
		})
	}
}

func TestHasherIgnoresUnrelatedFiles(t *testing.T) {
	root := newTestModule(t)
	before := testHash(t, root, nil)

	writeFile(t, root, "README.md", "changed")
	writeFile(t, root, "internal/core/core_test.go", "package core\n\nconst Y = 2\n")
	writeFile(t, root, "other/other.go", "package other\n")

	if after := testHash(t, root, nil); before != after {
		t.Errorf("hash depends on files outside of the build")
	}
}

func TestHasherToolchain(t *testing.T) {
	root := newTestModule(t)
	arm := testHash(t, root, NewGoCompiler("example.com/app", "cmd/fn", "", nil))
	amd := testHash(t, root, NewGoCompiler("example.com/app", "cmd/fn", "", &Toolchain{GoEnv: map[string]string{"GOARCH": "amd64"}}))
	vars := testHash(t, root, NewGoCompiler("example.com/app", "cmd/fn", "", &Toolchain{LDVars: map[string]string{"main.x": "1"}}))
	cache := testHash(t, root, NewGoCompiler("example.com/app", "cmd/fn", "", &Toolchain{GoEnv: map[string]string{"GOCACHE": t.TempDir()}}))

	if arm == amd {
		t.Errorf("hash does not depend on GOARCH")
	}
	if arm == vars {
		t.Errorf("hash does not depend on linker variables")
	}
	if arm != cache {
		t.Errorf("hash depends on machine local variables")
	}
}

func TestModuleDir(t *testing.T) {
	dir, err := moduleDir("github.com/fogfish/scud")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		t.Errorf("module dir %s is invalid: %v", dir, err)
	}

	if _, err := moduleDir("example.com/not/exists"); err == nil {
		t.Errorf("unknown module must fail")
	}
}

func TestGoCompilerEnv(t *testing.T) {
	t.Setenv("GOARCH", "amd64")
	t.Setenv("SCUD_TEST_VAR", "1")

	config := &Toolchain{GoEnv: map[string]string{"CGO_ENABLED": "1"}}
	gocc := NewGoCompiler("example.com/app", "cmd/fn", "", config)

	env := map[string]string{}
	for _, kv := range gocc.cmdEnv() {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				env[kv[:i]] = kv[i+1:]
				break
			}
		}
	}

	if env["GOARCH"] != "arm64" || env["GOOS"] != "linux" || env["CGO_ENABLED"] != "1" {
		t.Errorf("invalid build env: %v", env)
	}
	if env["SCUD_TEST_VAR"] != "1" {
		t.Errorf("process environment is not inherited")
	}
	if len(config.GoEnv) != 1 {
		t.Errorf("toolchain config is mutated: %v", config.GoEnv)
	}
	if gocc.sourceCode != "example.com/app/cmd/fn" {
		t.Errorf("invalid import path %s", gocc.sourceCode)
	}
}
