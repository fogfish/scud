//
// Copyright (C) 2020 - 2025 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/scud
//

package scud_test

import (
	"strings"
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
	"github.com/fogfish/scud"
)

func TestFunctionDefaultsLibrary(t *testing.T) {
	stack := newStack(t, nil)
	scud.NewFunctionGo(stack, jsii.String("f"),
		&scud.FunctionGoProps{
			SourceCodeModule: "github.com/fogfish/scud",
			SourceCodeLambda: "test/lambda/go",
		},
	)

	template := assertions.Template_FromStack(stack, nil)
	template.HasResourceProperties(jsii.String("AWS::Lambda::Function"),
		map[string]any{"Timeout": 60},
	)
	template.HasResourceProperties(jsii.String("AWS::Logs::LogGroup"),
		map[string]any{"RetentionInDays": 5},
	)
}

func TestFunctionDefaultsScope(t *testing.T) {
	stack := newStack(t, nil)
	scud.SetDefaults(stack, &scud.Defaults{
		Timeout:        awscdk.Duration_Seconds(jsii.Number(25)),
		MemorySize:     jsii.Number(256),
		LogGroupPrefix: "/app/test",
		LogRetention:   awslogs.RetentionDays_ONE_MONTH,
	})

	// defaults of nearest ancestor apply
	nested := constructs.NewConstruct(stack, jsii.String("Nested"))
	scud.SetDefaults(nested, &scud.Defaults{
		Timeout: awscdk.Duration_Seconds(jsii.Number(10)),
	})

	scud.NewFunctionGo(stack, jsii.String("f"),
		&scud.FunctionGoProps{
			SourceCodeModule: "github.com/fogfish/scud",
			SourceCodeLambda: "test/lambda/go",
			FunctionProps: &awslambda.FunctionProps{
				FunctionName: jsii.String("f"),
			},
		},
	)

	scud.NewFunctionGo(nested, jsii.String("g"),
		&scud.FunctionGoProps{
			SourceCodeModule: "github.com/fogfish/scud",
			SourceCodeLambda: "test/lambda/another",
			FunctionProps: &awslambda.FunctionProps{
				FunctionName: jsii.String("g"),
			},
		},
	)

	template := assertions.Template_FromStack(stack, nil)
	template.HasResourceProperties(jsii.String("AWS::Lambda::Function"),
		map[string]any{"FunctionName": "f", "Timeout": 25, "MemorySize": 256},
	)
	template.HasResourceProperties(jsii.String("AWS::Logs::LogGroup"),
		map[string]any{"LogGroupName": "/app/test/f", "RetentionInDays": 30},
	)
	template.HasResourceProperties(jsii.String("AWS::Lambda::Function"),
		map[string]any{"FunctionName": "g", "Timeout": 10},
	)
	template.HasResourceProperties(jsii.String("AWS::Logs::LogGroup"),
		map[string]any{"LogGroupName": "g", "RetentionInDays": 5},
	)
}

func TestFunctionDefaultsSharedLogGroup(t *testing.T) {
	stack := newStack(t, nil)
	logs := awslogs.NewLogGroup(stack, jsii.String("AppLogs"),
		&awslogs.LogGroupProps{LogGroupName: jsii.String("/app/test")},
	)
	scud.SetDefaults(stack, &scud.Defaults{LogGroup: logs})

	scud.NewFunctionGo(stack, jsii.String("f"),
		&scud.FunctionGoProps{
			SourceCodeModule: "github.com/fogfish/scud",
			SourceCodeLambda: "test/lambda/go",
		},
	)
	scud.NewFunctionGo(stack, jsii.String("g"),
		&scud.FunctionGoProps{
			SourceCodeModule: "github.com/fogfish/scud",
			SourceCodeLambda: "test/lambda/another",
		},
	)

	template := assertions.Template_FromStack(stack, nil)
	template.ResourceCountIs(jsii.String("AWS::Logs::LogGroup"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::Lambda::Function"), jsii.Number(2))
}

func TestFunctionNameLength(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("a-very-long-stack-name-of-the-application-for-test"), nil)

	mustPanic(t, "AWS Lambda allows at most 64", func() {
		scud.NewFunctionGo(stack, jsii.String("f"),
			&scud.FunctionGoProps{
				SourceCodeModule: "github.com/fogfish/scud",
				SourceCodeLambda: "test/lambda/go",
			},
		)
	})

	mustPanic(t, "AWS Lambda allows at most 64", func() {
		scud.NewFunctionGo(stack, jsii.String("g"),
			&scud.FunctionGoProps{
				SourceCodeModule: "github.com/fogfish/scud",
				SourceCodeLambda: "test/lambda/go",
				FunctionProps: &awslambda.FunctionProps{
					FunctionName: jsii.String(strings.Repeat("x", 65)),
				},
			},
		)
	})
}

func TestFunctionTenantIsolation(t *testing.T) {
	stack := newStack(t, nil)
	scud.NewFunctionGo(stack, jsii.String("f"),
		&scud.FunctionGoProps{
			SourceCodeModule: "github.com/fogfish/scud",
			SourceCodeLambda: "test/lambda/go",
			FunctionProps: &awslambda.FunctionProps{
				TenancyConfig: awslambda.TenancyConfig_PER_TENANT(),
			},
		},
	)

	template := assertions.Template_FromStack(stack, nil)
	template.HasResourceProperties(jsii.String("AWS::Lambda::Function"),
		map[string]any{"TenancyConfig": map[string]any{"TenantIsolationMode": "PER_TENANT"}},
	)
}
