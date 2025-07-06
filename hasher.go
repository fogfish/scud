//
// Copyright (C) 2020 - 2025 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/scud
//

package scud

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Hasher is a utility to compute hash of lambda function and its dependencies.
type Hasher struct {
	sourceCodeModule  string
	sourceCodeLambda  string
	sourceCodeVersion string
	fileType          *regexp.Regexp
	verbose           bool
}

func NewHasher(verbose bool) *Hasher {
	return &Hasher{
		fileType: regexp.MustCompile(`(.*\.go$)|(.*\.(mod|sum)$)`),
		verbose:  verbose,
	}
}

func (h *Hasher) Hash(sourceCodeModule, sourceCodeLambda, sourceCodeVersion string) (string, error) {
	t := time.Now()
	h.sourceCodeModule = sourceCodeModule
	h.sourceCodeLambda = sourceCodeLambda
	h.sourceCodeVersion = sourceCodeVersion

	hash := sha256.New()
	if err := h.hashPackage(hash); err != nil {
		return "", err
	}

	seq, err := h.deps()
	if err != nil {
		return "", err
	}

	for _, lib := range seq {
		dir := h.rootSourceCode(lib)
		ent, err := os.ReadDir(dir)
		if err != nil {
			return "", err
		}
		if err := h.hashSubPackage(hash, dir, ent); err != nil {
			return "", err
		}
	}

	checksum := fmt.Sprintf("%x", hash.Sum(nil))

	log.Printf("==> checksum %s | %s (%v)\n", checksum[:8], sourceCodeLambda, time.Since(t))
	if h.verbose {
		for i, lib := range seq[1:] {
			if i == len(seq)-2 {
				fmt.Fprintf(os.Stderr, "    └─ %s\n", lib)
				continue
			}
			fmt.Fprintf(os.Stderr, "    ├─ %s\n", lib)
		}
	}

	return checksum, nil
}

func (h *Hasher) rootSourceCode(sourceCodeModule string) string {
	sourceCode := os.Getenv("GITHUB_WORKSPACE")
	if sourceCode == "" {
		sourceCode = filepath.Join(os.Getenv("GOPATH"), "src", sourceCodeModule)
	}

	return sourceCode
}

func (h *Hasher) deps() ([]string, error) {
	pkg := filepath.Join(h.sourceCodeModule, h.sourceCodeLambda)

	buf := &bytes.Buffer{}
	cmd := exec.Command("go", "list", "-f", `{{join .Deps "\n"}}`, pkg)
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	seq := []string{pkg}
	s := bufio.NewScanner(buf)
	for s.Scan() {
		line := s.Text()
		if strings.HasPrefix(line, h.sourceCodeModule) {
			seq = append(seq, line)
		}
	}

	return seq, nil
}

func (h *Hasher) hashPackage(w io.Writer) error {
	vsn := ""
	if h.sourceCodeVersion != "" {
		vsn = fmt.Sprintf("@%s", h.sourceCodeVersion)
	}

	pkg := fmt.Sprintf("package: %s %s%s\n", h.sourceCodeModule, h.sourceCodeLambda, vsn)

	_, err := w.Write([]byte(pkg))
	if err != nil {
		return err
	}

	return nil
}

func (h *Hasher) hashSubPackage(w io.Writer, dir string, ent []os.DirEntry) error {
	for _, entry := range ent {
		if entry.IsDir() {
			continue
		}
		if h.fileType.MatchString(entry.Name()) {
			if err := h.hashFile(w, filepath.Join(dir, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *Hasher) hashFile(w io.Writer, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = fmt.Fprintf(w, "<file name=%s>\n", path)
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
