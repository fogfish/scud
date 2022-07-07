//
// Copyright (C) 2020 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/scud
//

package scud

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3assets"
	"github.com/aws/jsii-runtime-go"
)

/*

AssetCodeGo bundles lambda function from source code
*/
func AssetCodeGo(sourceCodePackage, sourceCodeLambda string) awslambda.Code {
	return awslambda.NewAssetCode(
		jsii.String(""),
		&awss3assets.AssetOptions{
			AssetHashType: awscdk.AssetHashType_CUSTOM,
			AssetHash:     jsii.String(hashpkg(sourceCodePackage, sourceCodeLambda)),
			Bundling: &awscdk.BundlingOptions{
				Image: awscdk.DockerImage_FromRegistry(jsii.String("golang")),
				Local: &gocc{filepath.Join(sourceCodePackage, sourceCodeLambda)},
				// Note: it make no sense to build Golang code inside container
			},
		})
}

func hashpkg(sourceCodePackage, sourceCodeLambda string) string {
	t := time.Now()
	hash := sha256.New()
	_, err := hash.Write([]byte(fmt.Sprintf("package: %s %s", sourceCodePackage, sourceCodeLambda)))
	if err != nil {
		panic(err)
	}

	exp, err := regexp.Compile(`(.*\.go$)|(.*\.(mod|sum)$)`)
	if err != nil {
		panic(err)
	}

	sourceCode := os.Getenv("GITHUB_WORKSPACE")
	if sourceCode == "" {
		sourceCode = filepath.Join(os.Getenv("GOPATH"), "src", sourceCodePackage)
	}

	err = filepath.Walk(
		sourceCode,
		func(path string, info fs.FileInfo, err error) error {
			if exp.MatchString(path) {
				if err := hashfile(hash, path); err != nil {
					return err
				}
			}
			return nil
		},
	)

	if err != nil {
		panic(err)
	}

	v := hash.Sum(nil)
	d := time.Since(t)
	log.Printf("==> checksum %s %x (%v)\n", filepath.Join(sourceCodePackage, sourceCodeLambda), v[:4], d)
	return fmt.Sprintf("%x", v)
}

func hashfile(hash hash.Hash, file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = hash.Write([]byte(fmt.Sprintf("<file name=%s}>", file)))
	if err != nil {
		return err
	}

	_, err = io.Copy(hash, f)
	if err != nil {
		return err
	}

	_, err = hash.Write([]byte("</file>"))
	if err != nil {
		return err
	}

	return nil
}

type gocc struct {
	sourceCode string
}

func (g gocc) TryBundle(outputDir *string, options *awscdk.BundlingOptions) *bool {
	t := time.Now()

	cmd := exec.Command("go", "build", "-o", filepath.Join(*outputDir, "main"), filepath.Join(g.sourceCode))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = make([]string, 0)

	cmd.Env = append(cmd.Env,
		fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
		fmt.Sprintf("GOPATH=%s", os.Getenv("GOPATH")),
		fmt.Sprintf("GOROOT=%s", os.Getenv("GOROOT")),
		fmt.Sprintf("GOMODCACHE=%s", os.Getenv("GOMODCACHE")),
		fmt.Sprintf("GOCACHE=%s", g.goCache()),
		fmt.Sprintf("GOOS=%s", "linux"),
		fmt.Sprintf("GOARCH=%s", "amd64"),
	)

	if err := cmd.Run(); err != nil {
		log.Printf("%s", err)
		return jsii.Bool(false)
	}

	d := time.Since(t)
	log.Printf("==> go build %s (%v)\n", g.sourceCode, d)
	return jsii.Bool(true)
}

func (g gocc) goCache() string {
	gha := os.Getenv("GITHUB_ACTION")
	gocache := os.Getenv("GOCACHE")

	if gha != "" && gocache != "" {
		return gocache
	}

	return "/tmp/go.amd64"
}
