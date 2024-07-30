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
	"path/filepath"
	"regexp"
	"time"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3assets"
	"github.com/aws/jsii-runtime-go"
)

type Compiler interface {
	awscdk.ILocalBundling
	SourceCodeModule() string
	SourceCodeLambda() string
	SourceCodeVersion() string
}

// AssetCodeGo bundles lambda function from source code
func AssetCodeGo(compiler Compiler) awslambda.Code {
	hash := hashpkg(compiler)
	return awslambda.NewAssetCode(
		jsii.String("."),
		&awss3assets.AssetOptions{
			AssetHashType: awscdk.AssetHashType_CUSTOM,
			AssetHash:     jsii.String(hash),
			Bundling: &awscdk.BundlingOptions{
				Image: awscdk.DockerImage_FromRegistry(jsii.String("golang")),
				Local: compiler.(awscdk.ILocalBundling),
				// Note: it make no sense to build Golang code inside container
			},
		})
}

func hashpkg(compiler Compiler) string {
	vsn := ""
	if compiler.SourceCodeVersion() != "" {
		vsn = fmt.Sprintf("@%s", compiler.SourceCodeVersion())
	}

	pkg := fmt.Sprintf("package: %s %s%s", compiler.SourceCodeModule(), compiler.SourceCodeLambda(), vsn)
	path := filepath.Join(compiler.SourceCodeModule(), compiler.SourceCodeLambda())

	t := time.Now()
	hash := sha256.New()
	_, err := hash.Write([]byte(pkg))
	if err != nil {
		panic(err)
	}

	exp, err := regexp.Compile(`(.*\.go$)|(.*\.(mod|sum)$)`)
	if err != nil {
		panic(err)
	}

	sourceCode := os.Getenv("GITHUB_WORKSPACE")
	if sourceCode == "" {
		sourceCode = filepath.Join(os.Getenv("GOPATH"), "src", compiler.SourceCodeModule())
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
	log.Printf("==> checksum %x | %s%s (%v)\n", v[:4], path, vsn, d)
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
