//
// Copyright (C) 2020 - 2025 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/scud
//

package scud

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/jsii-runtime-go"
)

// Configure Go compiler and Linter
type Toolchain struct {
	// Go environment, default includes
	//	GOOS=linux
	//	GOARCH=arm64
	//	CGO_ENABLED=0
	//
	// The compiler inherits the environment of the process (HOME, GOPATH,
	// GOCACHE, GOPROXY, GOPRIVATE, GOFLAGS, etc), variables defined here
	// override inherited ones.
	GoEnv map[string]string

	// Linker flags passed to the Go linker (-ldflags).
	LDFlags []string

	// Go variables injected at link time using -X key=value.
	// Example: -ldflags "-X main.version=1.0.0"
	LDVars map[string]string
}

type GoCompiler struct {
	sourceCode        string
	sourceCodePackage string
	sourceCodeLambda  string
	sourceCodeVersion string
	config            *Toolchain
}

const (
	goBinary = "bootstrap"
	goTags   = "lambda.norpc"
)

func NewGoCompiler(
	sourceCodePackage string,
	sourceCodeLambda string,
	sourceCodeVersion string,
	config *Toolchain,
) *GoCompiler {
	if config == nil {
		config = &Toolchain{}
	}

	return &GoCompiler{
		sourceCode:        importPath(sourceCodePackage, sourceCodeLambda),
		sourceCodePackage: sourceCodePackage,
		sourceCodeLambda:  sourceCodeLambda,
		sourceCodeVersion: sourceCodeVersion,
		config:            config,
	}
}

// importPath builds Go import path of the package from module path and
// relative path of the package within module.
func importPath(module, pkg string) string {
	return path.Join(module, filepath.ToSlash(pkg))
}

func (g *GoCompiler) SourceCodeModule() string  { return g.sourceCodePackage }
func (g *GoCompiler) SourceCodeLambda() string  { return g.sourceCodeLambda }
func (g *GoCompiler) SourceCodeVersion() string { return g.sourceCodeVersion }

func (g *GoCompiler) TryBundle(outputDir *string, options *awscdk.BundlingOptions) *bool {
	t := time.Now()

	target := filepath.Join(*outputDir, goBinary)
	goflags := []string{"build", "-tags", goTags}
	goflags = append(goflags, "-ldflags", strings.Join(g.ldflags(), " "))
	goflags = append(goflags, "-o", target)
	goflags = append(goflags, g.sourceCode)

	cmd := exec.Command("go", goflags...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = g.cmdEnv()

	if err := cmd.Run(); err != nil {
		log.Printf("%s", err)
		return jsii.Bool(false)
	}

	log.Printf("==> go build %s (%v)\n", g.sourceCode, time.Since(t))

	if compressUPX() {
		t := time.Now()
		cmd = exec.Command("upx", "--best", "-q", "--lzma", target)
		cmd.Stdout = io.Discard
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Printf("%s", err)
			return jsii.Bool(false)
		}
		log.Printf("==> compress %s (%v)\n", g.sourceCode, time.Since(t))
	}

	return jsii.Bool(true)
}

func compressUPX() bool { return os.Getenv("SCUD_COMPRESS_UPX") == "1" }

// linker flags, deterministic order
func (g *GoCompiler) ldflags() []string {
	ldflags := []string{"-s", "-w"}
	if len(g.config.LDFlags) > 0 {
		ldflags = append(ldflags, g.config.LDFlags...)
	}
	if g.sourceCodeVersion != "" {
		ldflags = append(ldflags, fmt.Sprintf("-X main.version=%s", g.sourceCodeVersion))
	}

	vars := make([]string, 0, len(g.config.LDVars))
	for name, value := range g.config.LDVars {
		vars = append(vars, fmt.Sprintf("-X %s=%s", name, value))
	}
	sort.Strings(vars)

	return append(ldflags, vars...)
}

// goEnv returns Go environment explicitly defined for the build:
// defaults overridden by the toolchain config. It does not include
// the inherited process environment.
func (g *GoCompiler) goEnv() map[string]string {
	env := map[string]string{
		"GOOS":        "linux",
		"GOARCH":      "arm64",
		"CGO_ENABLED": "0",
	}
	for key, val := range g.config.GoEnv {
		env[key] = val
	}
	return env
}

// cmdEnv returns the environment of go command: the process environment
// overridden by the build environment.
func (g *GoCompiler) cmdEnv() []string {
	env := g.goEnv()
	seq := make([]string, 0, len(env))
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if _, has := env[key]; !has {
			seq = append(seq, kv)
		}
	}

	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		seq = append(seq, key+"="+env[key])
	}

	return seq
}

// machine local variables, they do not change the output of compiler
var goEnvLocal = map[string]struct{}{
	"PATH": {}, "HOME": {}, "TMPDIR": {}, "GOTMPDIR": {},
	"GOPATH": {}, "GOROOT": {}, "GOMODCACHE": {}, "GOCACHE": {}, "GOENV": {},
	"GOPROXY": {}, "GOPRIVATE": {}, "GONOPROXY": {}, "GONOSUMDB": {}, "GOSUMDB": {},
	"GOINSECURE": {}, "GOVCS": {}, "GOAUTH": {}, "GOTELEMETRY": {},
}

// fingerprint of build configuration, which affects the binary
func (g *GoCompiler) fingerprint() []string {
	env := g.goEnv()
	seq := make([]string, 0, len(env)+4)
	for key, val := range env {
		if _, local := goEnvLocal[key]; !local {
			seq = append(seq, "env: "+key+"="+val)
		}
	}
	sort.Strings(seq)

	seq = append(seq, "tags: "+goTags)
	seq = append(seq, "ldflags: "+strings.Join(g.ldflags(), " "))
	if compressUPX() {
		seq = append(seq, "upx: --best --lzma")
	}

	return seq
}
