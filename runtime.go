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
		jsii.String(sourceCodeLambda),
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
	hash := sha256.New()
	_, err := hash.Write([]byte(fmt.Sprintf("package: %s %s", sourceCodePackage, sourceCodeLambda)))
	if err != nil {
		panic(err)
	}

	exp, err := regexp.Compile("(.*\\.go$)|(.*\\.(mod|sum)$)")
	if err != nil {
		panic(err)
	}

	log.Printf("==> debug %s\n", filepath.Join(os.Getenv("GOPATH"), "src", sourceCodePackage))
	err = filepath.Walk(
		filepath.Join(os.Getenv("GOPATH"), "src", sourceCodePackage),
		func(path string, info fs.FileInfo, err error) error {
			if exp.MatchString(path) {
				hashfile(hash, path)
			}
			return nil
		},
	)

	if err != nil {
		panic(err)
	}

	v := hash.Sum(nil)
	log.Printf("==> %s %x\n", filepath.Join(sourceCodePackage, sourceCodeLambda), v)
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
	log.Printf("==> go build %s\n", g.sourceCode)

	cmd := exec.Command("go", "build", "-o", filepath.Join(*outputDir, "main"), filepath.Join(g.sourceCode))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = make([]string, 0)

	cmd.Env = append(cmd.Env,
		fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
		fmt.Sprintf("GOPATH=%s", os.Getenv("GOPATH")),
		fmt.Sprintf("GOROOT=%s", os.Getenv("GOROOT")),
		fmt.Sprintf("GOCACHE=%s", "/tmp/go.amd64"),
		fmt.Sprintf("GOOS=%s", "linux"),
		fmt.Sprintf("GOARCH=%s", "amd64"),
	)

	if err := cmd.Run(); err != nil {
		log.Printf("%s", err)
		return jsii.Bool(false)
	}

	return jsii.Bool(true)
}
