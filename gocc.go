package scud

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/jsii-runtime-go"
)

type GoCompiler struct {
	sourceCode        string
	sourceCodePackage string
	sourceCodeLambda  string
	env               map[string]string
}

func NewGoCompiler(
	sourceCodePackage string,
	sourceCodeLambda string,
	env map[string]string,
) *GoCompiler {
	if env == nil {
		env = map[string]string{}
	}

	return &GoCompiler{
		sourceCode:        filepath.Join(sourceCodePackage, sourceCodeLambda),
		sourceCodePackage: sourceCodePackage,
		sourceCodeLambda:  sourceCodeLambda,
		env:               env,
	}
}

func (g *GoCompiler) SourceCodePackage() string { return g.sourceCodePackage }
func (g *GoCompiler) SourceCodeLambda() string  { return g.sourceCodeLambda }

func (g *GoCompiler) TryBundle(outputDir *string, options *awscdk.BundlingOptions) *bool {
	t := time.Now()

	cmd := exec.Command("go", "build", "-o", filepath.Join(*outputDir, "main"), filepath.Join(g.sourceCode))
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
		if _, exists := g.env[envvar]; !exists {
			g.env[envvar] = os.Getenv(envvar)
		}
	}

	for envvar, defval := range map[string]string{
		"GOOS":        "linux",
		"GOARCH":      "amd64",
		"CGO_ENABLED": "0",
	} {
		if _, exists := g.env[envvar]; !exists {
			g.env[envvar] = defval
		}
	}

	for envvar, gen := range map[string]func() string{
		"GOCACHE": g.goCache,
	} {
		if _, exists := g.env[envvar]; !exists {
			g.env[envvar] = gen()
		}
	}

	env := make([]string, 0)
	for key, val := range g.env {
		env = append(env, key+"="+val)
	}
	return env
}
