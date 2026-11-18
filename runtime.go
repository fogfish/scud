//
// Copyright (C) 2020 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/scud
//

package scud

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"sync"

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
	if gocc, ok := compiler.(*GoCompiler); ok {
		hash = hash.withCompiler(gocc)
	}

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

var (
	moduleDirLock  sync.Mutex
	moduleDirCache = map[string]string{}
)

// moduleDir resolves the root directory of Go module in module mode.
// The module is either the main module or one of its dependencies.
func moduleDir(sourceCodeModule string) (string, error) {
	moduleDirLock.Lock()
	defer moduleDirLock.Unlock()

	if dir, has := moduleDirCache[sourceCodeModule]; has {
		return dir, nil
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", sourceCodeModule)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("unable to resolve module %s: %w: %s", sourceCodeModule, err, strings.TrimSpace(stderr.String()))
	}

	dir := strings.TrimSpace(stdout.String())
	if dir == "" {
		return "", fmt.Errorf("unable to resolve module %s: source code is not available, run go mod download", sourceCodeModule)
	}

	moduleDirCache[sourceCodeModule] = dir
	return dir, nil
}

func rootSourceCode(sourceCodeModule string) string {
	dir, err := moduleDir(sourceCodeModule)
	if err != nil {
		panic(err)
	}
	return dir
}
