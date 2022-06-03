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
	"log"
	"os"
	"os/exec"
	"path/filepath"

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
			AssetHashType: awscdk.AssetHashType_OUTPUT,
			Bundling: &awscdk.BundlingOptions{
				Image: awscdk.DockerImage_FromRegistry(jsii.String("golang")),
				Local: &gocc{filepath.Join(sourceCodePackage, sourceCodeLambda)},
				// Note: it make no sense to build Golang code inside container
			},
		})
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
