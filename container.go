//
// Copyright (C) 2020 - 2024 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/scud
//

package scud

import (
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecrassets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

type ContainerGoProps struct {
	*awslambda.DockerImageFunctionProps

	// Canonical name of Golang module that containing the function
	//	SourceCodeModule: "github.com/fogfish/scud",
	SourceCodeModule string

	// Path to lambda function relative to the module
	//	SourceCodeLambda:  "test/lambda/go"
	SourceCodeLambda string

	// The version of software asset passed as linker flag
	//	-ldflags '-X main.version=...'
	SourceCodeVersion string

	// Variables and its values passed as linker flags
	//	-ldflags '-X key1=val1 -X key2=val2 ...'
	GoVar map[string]string

	// Go environment, default includes
	//	GOOS=linux
	//	GOARCH=arm64
	//	CGO_ENABLED=0
	GoEnv map[string]string

	// Static files included into container, the path is relative to module
	StaticAssets []string

	// Linux Alpine Packages (apk) to be installed within the container
	Packages []string
}

func (*ContainerGoProps) HKT1(awslambda.Function) {}

func (props *ContainerGoProps) UniqueID() string {
	return funcName(props.SourceCodeModule, props.SourceCodeLambda)
}

func NewContainerGo(scope constructs.Construct, id *string, spec *ContainerGoProps) awslambda.Function {
	var props awslambda.DockerImageFunctionProps
	if spec.DockerImageFunctionProps != nil {
		props = *spec.DockerImageFunctionProps
	}

	if props.Timeout == nil {
		props.Timeout = awscdk.Duration_Minutes(jsii.Number(1))
	}

	if props.FunctionName == nil {
		props.FunctionName = jsii.Sprintf("%s-%s", *awscdk.Aws_STACK_NAME(), spec.UniqueID())
	}

	if props.LogGroup == nil {
		// See: https://docs.aws.amazon.com/lambda/latest/dg/monitoring-cloudwatchlogs-loggroups.html
		props.LogGroup = awslogs.NewLogGroup(scope, jsii.String("LogGroup"),
			&awslogs.LogGroupProps{
				LogGroupName:  props.FunctionName,
				Retention:     awslogs.RetentionDays_FIVE_DAYS,
				RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
			},
		)
	}

	// arm64 is default deployment
	platContainer := "linux/arm64"
	platCode := awsecrassets.Platform_LINUX_ARM64()
	props.Architecture = awslambda.Architecture_ARM_64()
	if spec.GoEnv != nil {
		switch spec.GoEnv["GOARCH"] {
		case "amd64":
			platContainer = "linux/amd64"
			props.Architecture = awslambda.Architecture_X86_64()
			platCode = awsecrassets.Platform_LINUX_AMD64()
		case "arm64":
			platContainer = "linux/arm64"
			props.Architecture = awslambda.Architecture_ARM_64()
			platCode = awsecrassets.Platform_LINUX_ARM64()
		}
	}

	gocc := NewGoCompiler(
		spec.SourceCodeModule,
		spec.SourceCodeLambda,
		spec.SourceCodeVersion,
		spec.GoVar,
		spec.GoEnv,
	)

	path := filepath.Join(os.TempDir(), spec.SourceCodeModule, spec.SourceCodeLambda)
	if err := os.MkdirAll(path, 0775); err != nil {
		panic(err)
	}

	isBuild := gocc.TryBundle(jsii.String(path), nil)
	if !*isBuild {
		panic(fmt.Errorf("unable to build %s/%s", spec.SourceCodeModule, spec.SourceCodeLambda))
	}

	docker := fmt.Sprintf(`
FROM %s
%s
%s
ADD bootstrap /bin/bootstrap

CMD ["/bin/bootstrap"]
	`, dockerBaseImage(spec), dockerPackages(spec), dockerAssets(path, spec))

	err := os.WriteFile(filepath.Join(path, "Dockerfile"), []byte(docker), 0664)
	if err != nil {
		panic(err)
	}

	props.Code = awslambda.DockerImageCode_FromImageAsset(
		jsii.String(path),
		&awslambda.AssetImageCodeProps{
			Platform: platCode,
			BuildArgs: &map[string]*string{
				"platform": jsii.String(platContainer),
			},
		},
	)

	return awslambda.NewDockerImageFunction(scope, id, &props)
}

func dockerBaseImage(spec *ContainerGoProps) string {
	if len(spec.Packages) == 0 {
		return "scratch"

	}
	return "alpine"
}

func dockerPackages(spec *ContainerGoProps) string {
	if len(spec.Packages) == 0 {
		return ""
	}

	return fmt.Sprintf("RUN apk --no-cache add --update %s\n",
		strings.Join(spec.Packages, " "),
	)
}

func dockerAssets(path string, spec *ContainerGoProps) string {
	root := rootSourceCode(spec.SourceCodeModule)

	assets := []string{}
	for _, asset := range spec.StaticAssets {
		source := filepath.Join(root, asset)
		target := filepath.Join(path, asset)
		if err := os.MkdirAll(filepath.Dir(target), 0775); err != nil {
			panic(err)
		}

		log.Printf("==> copy %s\n", asset)
		if err := copy(source, target); err != nil {
			panic(err)
		}

		assets = append(assets, fmt.Sprintf("ADD %s /opt/%s", asset, asset))
	}

	return strings.Join(assets, "\n")
}

func copy(source, target string) (err error) {
	r, err := os.Open(source)
	if err != nil {
		return err
	}
	defer r.Close()

	w, err := os.Create(target)
	if err != nil {
		return err
	}
	defer func() { err = w.Close() }()

	if _, err := io.Copy(w, r); err != nil {
		return &fs.PathError{Op: "copy", Path: target, Err: err}
	}

	return nil
}
