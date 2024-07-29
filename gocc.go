package scud

import (
	"fmt"
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
	govar             map[string]string
	goenv             map[string]string
}

const goBinary = "bootstrap"

func NewGoCompiler(
	sourceCodePackage string,
	sourceCodeLambda string,
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
		govar:             govar,
		goenv:             goenv,
	}
}

func (g *GoCompiler) SourceCodePackage() string { return g.sourceCodePackage }
func (g *GoCompiler) SourceCodeLambda() string  { return g.sourceCodeLambda }

func (g *GoCompiler) TryBundle(outputDir *string, options *awscdk.BundlingOptions) *bool {
	t := time.Now()

	goflags := []string{"build", "-tags", "lambda.norpc"}

	ldflags := []string{"-s", "-w"}
	for name, value := range g.govar {
		ldflags = append(ldflags, fmt.Sprintf("-X %s=%s", name, value))
	}
	goflags = append(goflags, "-ldflags", strings.Join(ldflags, " "))
	goflags = append(goflags, "-o", filepath.Join(*outputDir, goBinary))
	goflags = append(goflags, filepath.Join(g.sourceCode))

	cmd := exec.Command("go", goflags...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = g.cmdEnv()

	if err := cmd.Run(); err != nil {
		log.Printf("%s", err)
		return jsii.Bool(false)
	}

	d := time.Since(t)
	log.Printf("==> go build %s (%v)\n", g.sourceCode, d)
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
