package scud

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/jsii-runtime-go"
)

type GoCompiler struct {
	sourceCode        string
	sourceCodePackage string
	sourceCodeLambda  string
	sourceCodeVersion string
	govar             map[string]string
	goenv             map[string]string
}

const goBinary = "bootstrap"

func NewGoCompiler(
	sourceCodePackage string,
	sourceCodeLambda string,
	sourceCodeVersion string,
	govar map[string]string,
	goenv map[string]string,
) *GoCompiler {
	if goenv == nil {
		goenv = map[string]string{}
	}

	if govar == nil {
		govar = map[string]string{}
	}

	return &GoCompiler{
		sourceCode:        filepath.Join(sourceCodePackage, sourceCodeLambda),
		sourceCodePackage: sourceCodePackage,
		sourceCodeLambda:  sourceCodeLambda,
		sourceCodeVersion: sourceCodeVersion,
		govar:             govar,
		goenv:             goenv,
	}
}

func (g *GoCompiler) SourceCodeModule() string  { return g.sourceCodePackage }
func (g *GoCompiler) SourceCodeLambda() string  { return g.sourceCodeLambda }
func (g *GoCompiler) SourceCodeVersion() string { return g.sourceCodeVersion }

func (g *GoCompiler) TryBundle(outputDir *string, options *awscdk.BundlingOptions) *bool {
	t := time.Now()

	target := filepath.Join(*outputDir, goBinary)
	goflags := []string{"build", "-tags", "lambda.norpc"}

	ldflags := []string{"-s", "-w"}
	if g.sourceCodeVersion != "" {
		ldflags = append(ldflags, fmt.Sprintf("-X main.version=%s", g.sourceCodeVersion))
	}

	for name, value := range g.govar {
		ldflags = append(ldflags, fmt.Sprintf("-X %s=%s", name, value))
	}
	goflags = append(goflags, "-ldflags", strings.Join(ldflags, " "))
	goflags = append(goflags, "-o", target)
	goflags = append(goflags, filepath.Join(g.sourceCode))

	cmd := exec.Command("go", goflags...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = g.cmdEnv()

	if err := cmd.Run(); err != nil {
		log.Printf("%s", err)
		return jsii.Bool(false)
	}

	log.Printf("==> go build %s (%v)\n", g.sourceCode, time.Since(t))

	if os.Getenv("SCUD_COMPRESS_UPX") == "1" {
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

func (g *GoCompiler) goCache() string {
	gha := os.Getenv("GITHUB_ACTION")
	gocache := os.Getenv("GOCACHE")

	if gha != "" && gocache != "" {
		return gocache
	}

	return "/tmp/go.amd64"
}

func (g *GoCompiler) cmdEnv() []string {
	for _, envvar := range []string{
		"PATH",
		"GOPATH",
		"GOROOT",
		"GOMODCACHE",
	} {
		if _, exists := g.goenv[envvar]; !exists {
			g.goenv[envvar] = os.Getenv(envvar)
		}
	}

	for envvar, defval := range map[string]string{
		"GOOS":        "linux",
		"GOARCH":      "arm64",
		"CGO_ENABLED": "0",
	} {
		if _, exists := g.goenv[envvar]; !exists {
			g.goenv[envvar] = defval
		}
	}

	for envvar, gen := range map[string]func() string{
		"GOCACHE": g.goCache,
	} {
		if _, exists := g.goenv[envvar]; !exists {
			g.goenv[envvar] = gen()
		}
	}

	env := make([]string, 0)
	for key, val := range g.goenv {
		env = append(env, key+"="+val)
	}
	return env
}
