//
// Copyright (C) 2020 - 2024 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/scud
//

package scud

import (
	"path/filepath"
	"strings"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// FunctionGoProps is properties of the function
type FunctionGoProps struct {
	*awslambda.FunctionProps

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
}

func (*FunctionGoProps) HKT1(awslambda.Function) {}

func (props *FunctionGoProps) UniqueID() string {
	return funcName(props.SourceCodeModule, props.SourceCodeLambda)
}

// NewFunctionGo creates Golang Lambda Function from "inline" code
func NewFunctionGo(scope constructs.Construct, id *string, spec *FunctionGoProps) awslambda.Function {
	var props awslambda.FunctionProps
	if spec.FunctionProps != nil {
		props = *spec.FunctionProps
	}

	if props.Timeout == nil {
		props.Timeout = awscdk.Duration_Minutes(jsii.Number(1))
	}

	if props.FunctionName == nil {
		props.FunctionName = jsii.Sprintf("%s-%s", *awscdk.Aws_STACK_NAME(), spec.UniqueID())
	}

	if props.LogGroup == nil {
		// See: https://docs.aws.amazon.com/lambda/latest/dg/monitoring-cloudwatchlogs-loggroups.html
		props.LogGroup = awslogs.NewLogGroup(scope, jsii.String("Logs"+spec.UniqueID()),
			&awslogs.LogGroupProps{
				LogGroupName:  props.FunctionName,
				Retention:     awslogs.RetentionDays_FIVE_DAYS,
				RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
			},
		)
	}

	// arm64 is default deployment
	props.Architecture = awslambda.Architecture_ARM_64()
	if spec.GoEnv != nil {
		switch spec.GoEnv["GOARCH"] {
		case "amd64":
			props.Architecture = awslambda.Architecture_X86_64()
		case "arm64":
			props.Architecture = awslambda.Architecture_ARM_64()
		}
	}

	gocc := NewGoCompiler(
		spec.SourceCodeModule,
		spec.SourceCodeLambda,
		spec.SourceCodeVersion,
		spec.GoVar,
		spec.GoEnv,
	)
	props.Code = AssetCodeGo(gocc)
	props.Handler = jsii.String(goBinary)
	props.Runtime = awslambda.Runtime_PROVIDED_AL2()

	return awslambda.NewFunction(scope, id, &props)
}

func funcName(scModule, scLambda string) string {
	return shorten(filepath.Join(scModule, scLambda))
}

func shorten(path string) string {
	seq := strings.Split(path, string(filepath.Separator))
	if len(seq) == 1 {
		return path
	}

	for i := 0; i < len(seq)-1; i++ {
		seq[i] = shortenSegment(seq[i])
	}
	return strings.Join(seq, "")
}

// shortenSegment heuristically shortens a path segment
func shortenSegment(segment string) string {
	length := len(segment)
	switch {
	case length <= 3:
		return segment // Keep short names unchanged
	case length <= 6:
		return removeVowels(segment) // Remove vowels from medium-length names
	default:
		short := removeVowels(segment)
		if len(short) > 4 {
			return short[:4]
		}
		return short
	}
}

func removeVowels(segment string) string {
	vowels := "aeiouAEIOU"
	keep := []rune{}
	for i, r := range segment {
		if i == 0 || !strings.ContainsRune(vowels, r) {
			keep = append(keep, r)
		}
	}
	if len(keep) < 2 { // Ensure at least 2 characters remain
		return segment
	}
	return string(keep)
}
