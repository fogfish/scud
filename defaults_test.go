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

func TestFunctionLogging(t *testing.T) {
	json := func(system, application string) map[string]any {
		return map[string]any{
			"LogFormat":           "JSON",
			"SystemLogLevel":      system,
			"ApplicationLogLevel": application,
		}
	}
	text := map[string]any{
		"LogFormat":           "Text",
		"SystemLogLevel":      assertions.Match_Absent(),
		"ApplicationLogLevel": assertions.Match_Absent(),
	}

	for name, tt := range map[string]struct {
		defaults *scud.Defaults
		props    awslambda.FunctionProps
		expected map[string]any
	}{
		"library": {
			expected: json("WARN", "INFO"),
		},
		"scope": {
			defaults: &scud.Defaults{
				SystemLogLevel:      awslambda.SystemLogLevel_INFO,
				ApplicationLogLevel: awslambda.ApplicationLogLevel_ERROR,
			},
			expected: json("INFO", "ERROR"),
		},
		"scope opts out": {
			defaults: &scud.Defaults{LoggingFormat: awslambda.LoggingFormat_TEXT},
			expected: text,
		},
		"function over scope": {
			defaults: &scud.Defaults{SystemLogLevel: awslambda.SystemLogLevel_INFO},
			props: awslambda.FunctionProps{
				SystemLogLevelV2:      awslambda.SystemLogLevel_DEBUG,
				ApplicationLogLevelV2: awslambda.ApplicationLogLevel_TRACE,
			},
			expected: json("DEBUG", "TRACE"),
		},
		"function opts out": {
			props:    awslambda.FunctionProps{LoggingFormat: awslambda.LoggingFormat_TEXT},
			expected: text,
		},
		"function opts in": {
			defaults: &scud.Defaults{LoggingFormat: awslambda.LoggingFormat_TEXT},
			props:    awslambda.FunctionProps{LoggingFormat: awslambda.LoggingFormat_JSON},
			expected: json("WARN", "INFO"),
		},
		"deprecated levels": {
			props: awslambda.FunctionProps{
				//lint:ignore SA1019 deprecated properties are respected
				SystemLogLevel: jsii.String("INFO"),
				//lint:ignore SA1019 deprecated properties are respected
				ApplicationLogLevel: jsii.String("DEBUG"),
			},
			expected: json("INFO", "DEBUG"),
		},
		"deprecated format": {
			//lint:ignore SA1019 deprecated properties are respected
			props:    awslambda.FunctionProps{LogFormat: jsii.String("Text")},
			expected: text,
		},
		"deprecated json format": {
			//lint:ignore SA1019 deprecated properties are respected
			props:    awslambda.FunctionProps{LogFormat: jsii.String("JSON")},
			expected: json("WARN", "INFO"),
		},
	} {
		t.Run(name, func(t *testing.T) {
			stack := newStack(t, nil)
			if tt.defaults != nil {
				scud.SetDefaults(stack, tt.defaults)
			}

			scud.NewFunctionGo(stack, jsii.String("f"),
				&scud.FunctionGoProps{
					SourceCodeModule: "github.com/fogfish/scud",
					SourceCodeLambda: "test/lambda/go",
					FunctionProps:    &tt.props,
				},
			)

			template := assertions.Template_FromStack(stack, nil)
			template.HasResourceProperties(jsii.String("AWS::Lambda::Function"),
				map[string]any{"LoggingConfig": tt.expected},
			)
		})
	}
}

func TestContainerLogging(t *testing.T) {
	for name, tt := range map[string]struct {
		props    awslambda.DockerImageFunctionProps
		expected map[string]any
	}{
		"library": {
			expected: map[string]any{"LogFormat": "JSON", "SystemLogLevel": "WARN", "ApplicationLogLevel": "INFO"},
		},
		"function": {
			props:    awslambda.DockerImageFunctionProps{LoggingFormat: awslambda.LoggingFormat_TEXT},
			expected: map[string]any{"LogFormat": "Text", "SystemLogLevel": assertions.Match_Absent()},
		},
	} {
		t.Run(name, func(t *testing.T) {
			stack := newStack(t, nil)
			scud.NewContainerGo(stack, jsii.String("f"),
				&scud.ContainerGoProps{
					SourceCodeModule:         "github.com/fogfish/scud",
					SourceCodeLambda:         "test/lambda/go",
					DockerImageFunctionProps: &tt.props,
				},
			)

			template := assertions.Template_FromStack(stack, nil)
			template.HasResourceProperties(jsii.String("AWS::Lambda::Function"),
				map[string]any{"LoggingConfig": tt.expected},
			)
		})
	}
}
