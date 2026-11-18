//
// Copyright (C) 2020 - 2025 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/scud
//

package scud

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Hasher is a utility to compute hash of lambda function and its dependencies.
//
// The hash covers:
//   - the content of every source file (Go, Cgo, assembly, embedded files)
//     of every package the function depends on that belongs to the main
//     module or to a module replaced by a local directory;
//   - the path and version of every other module the function depends on;
//   - the Go toolchain version;
//   - the build configuration (GOOS, GOARCH, CGO_ENABLED, ldflags, etc).
//
// Packages are resolved by `go list` in module mode, file names are hashed
// relative to the package import path, so the hash is the same on any
// machine regardless of the checkout location.
type Hasher struct {
	verbose  bool
	dir      string
	env      []string
	settings []string
	tags     string
}

func NewHasher(verbose bool) *Hasher {
	gocc := NewGoCompiler("", "", "", nil)
	return &Hasher{
		verbose:  verbose,
		env:      gocc.cmdEnv(),
		settings: gocc.fingerprint(),
		tags:     goTags,
	}
}

func (h *Hasher) withCompiler(gocc *GoCompiler) *Hasher {
	h.env = gocc.cmdEnv()
	h.settings = gocc.fingerprint()
	return h
}

// package metadata produced by go list
type goPackage struct {
	ImportPath string
	Dir        string
	Standard   bool
	Module     *goModule
	GoFiles    []string
	CgoFiles   []string
	CFiles     []string
	CXXFiles   []string
	HFiles     []string
	SFiles     []string
	SysoFiles  []string
	EmbedFiles []string
	Error      *struct{ Err string }
}

type goModule struct {
	Path    string
	Version string
	Main    bool
	Replace *goModule
}

// local source code is hashed file by file, remote by module version
func (m *goModule) isLocal() bool {
	return m == nil || m.Main || (m.Replace != nil && m.Replace.Version == "")
}

func (m *goModule) id() string {
	if m.Replace != nil {
		return fmt.Sprintf("%s@%s => %s@%s", m.Path, m.Version, m.Replace.Path, m.Replace.Version)
	}
	return m.Path + "@" + m.Version
}

func (h *Hasher) Hash(sourceCodeModule, sourceCodeLambda, sourceCodeVersion string) (string, error) {
	t := time.Now()
	pkg := importPath(sourceCodeModule, sourceCodeLambda)

	hash := sha256.New()

	vsn := ""
	if sourceCodeVersion != "" {
		vsn = "@" + sourceCodeVersion
	}
	if _, err := fmt.Fprintf(hash, "package: %s %s%s\n", sourceCodeModule, sourceCodeLambda, vsn); err != nil {
		return "", err
	}

	goversion, err := h.goVersion()
	if err != nil {
		return "", err
	}
	if _, err := fmt.Fprintf(hash, "go: %s\n", goversion); err != nil {
		return "", err
	}

	for _, s := range h.settings {
		if _, err := fmt.Fprintf(hash, "%s\n", s); err != nil {
			return "", err
		}
	}

	seq, err := h.deps(pkg)
	if err != nil {
		return "", err
	}

	tree := []string{}
	mods := map[string]struct{}{}
	for _, p := range seq {
		if p.Standard {
			continue
		}

		if !p.Module.isLocal() {
			id := p.Module.id()
			if _, has := mods[id]; !has {
				mods[id] = struct{}{}
				if _, err := fmt.Fprintf(hash, "module: %s\n", id); err != nil {
					return "", err
				}
				tree = append(tree, id)
			}
			continue
		}

		if err := h.hashPackage(hash, p); err != nil {
			return "", err
		}
		tree = append(tree, p.ImportPath)
	}

	checksum := fmt.Sprintf("%x", hash.Sum(nil))

	log.Printf("==> checksum %s | %s (%v)\n", checksum[:8], sourceCodeLambda, time.Since(t))
	if h.verbose {
		for i, lib := range tree {
			if i == len(tree)-1 {
				fmt.Fprintf(os.Stderr, "    └─ %s\n", lib)
				continue
			}
			fmt.Fprintf(os.Stderr, "    ├─ %s\n", lib)
		}
	}

	return checksum, nil
}

func (h *Hasher) goVersion() (string, error) {
	out, err := h.run("go", "env", "GOVERSION")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// list the package and all its dependencies using the build environment,
// so that build constraints select the same files as the compiler does.
func (h *Hasher) deps(pkg string) ([]goPackage, error) {
	out, err := h.run("go", "list", "-deps", "-tags", h.tags,
		"-json=ImportPath,Dir,Standard,Module,GoFiles,CgoFiles,CFiles,CXXFiles,HFiles,SFiles,SysoFiles,EmbedFiles,Error",
		pkg,
	)
	if err != nil {
		return nil, err
	}

	seq := []goPackage{}
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var p goPackage
		if err := dec.Decode(&p); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("unable to parse go list output: %w", err)
		}
		if p.Error != nil {
			return nil, fmt.Errorf("package %s: %s", p.ImportPath, p.Error.Err)
		}
		seq = append(seq, p)
	}

	return seq, nil
}

func (h *Hasher) run(name string, args ...string) ([]byte, error) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd := exec.Command(name, args...)
	cmd.Dir = h.dir
	cmd.Env = h.env
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}

	return stdout.Bytes(), nil
}

func (h *Hasher) hashPackage(w io.Writer, p goPackage) error {
	for _, files := range [][]string{
		p.GoFiles, p.CgoFiles, p.CFiles, p.CXXFiles,
		p.HFiles, p.SFiles, p.SysoFiles, p.EmbedFiles,
	} {
		for _, file := range files {
			if err := h.hashFile(w, p.Dir, p.ImportPath, file); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *Hasher) hashFile(w io.Writer, dir, pkg, file string) error {
	f, err := os.Open(filepath.Join(dir, file))
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = fmt.Fprintf(w, "<file name=%s/%s>\n", pkg, filepath.ToSlash(file))
	if err != nil {
		return err
	}

	_, err = io.Copy(w, f)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(w, "</file>\n")
	if err != nil {
		return err
	}

	return nil
}
