//
// Copyright (C) 2020 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/scud
//

package scud

import (
	"fmt"
	"os"
	"path/filepath"

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
	hash := NewHasher(false)
	checksum, err := hash.Hash(
		compiler.SourceCodeModule(),
		compiler.SourceCodeLambda(),
		compiler.SourceCodeVersion(),
	)
	if err != nil {
		panic(fmt.Errorf("failed to compute hash of the source code: %w", err))
	}

	return awslambda.NewAssetCode(
		jsii.String("."),
		&awss3assets.AssetOptions{
			AssetHashType: awscdk.AssetHashType_CUSTOM,
			AssetHash:     jsii.String(checksum),
			Bundling: &awscdk.BundlingOptions{
				Image: awscdk.DockerImage_FromRegistry(jsii.String("golang")),
				Local: compiler.(awscdk.ILocalBundling),
				// Note: it make no sense to build Golang code inside container
			},
		})
}

func rootSourceCode(sourceCodeModule string) string {
	sourceCode := os.Getenv("GITHUB_WORKSPACE")
	if sourceCode == "" {
		sourceCode = filepath.Join(os.Getenv("GOPATH"), "src", sourceCodeModule)
	}

	return sourceCode
}
