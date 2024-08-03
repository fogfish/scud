//
// Copyright (C) 2020 - 2024 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/scud
//

package scud_test

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/jsii-runtime-go"
	"github.com/fogfish/scud"
)

func TestFunctionGo(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("Test"), nil)

	scud.NewFunctionGo(stack, jsii.String("test"),
		&scud.FunctionGoProps{
			SourceCodeModule: "github.com/fogfish/scud",
			SourceCodeLambda: "test/lambda/go",
		},
	)

	require := map[*string]*float64{
		jsii.String("AWS::IAM::Role"):        jsii.Number(2),
		jsii.String("AWS::Lambda::Function"): jsii.Number(2),
		jsii.String("Custom::LogRetention"):  jsii.Number(1),
	}

	template := assertions.Template_FromStack(stack, nil)
	for key, val := range require {
		template.ResourceCountIs(key, val)
	}
}

func TestFunctionGoArch(t *testing.T) {
	for arch, config := range map[string]string{
		"arm64": "arm64",
		"amd64": "x86_64",
	} {

		app := awscdk.NewApp(nil)
		stack := awscdk.NewStack(app, jsii.String("Test"), nil)

		scud.NewFunctionGo(stack, jsii.String("test"),
			&scud.FunctionGoProps{
				SourceCodeModule:  "github.com/fogfish/scud",
				SourceCodeLambda:  "test/lambda/go",
				SourceCodeVersion: "v1.2.3",
				GoEnv: map[string]string{
					"GOARCH": arch,
				},
				GoVar: map[string]string{
					"main.some": "1.2.3",
				},
			},
		)

		require := map[*string]*float64{
			jsii.String("AWS::IAM::Role"):        jsii.Number(2),
			jsii.String("AWS::Lambda::Function"): jsii.Number(2),
			jsii.String("Custom::LogRetention"):  jsii.Number(1),
		}

		template := assertions.Template_FromStack(stack, nil)
		for key, val := range require {
			template.ResourceCountIs(key, val)
		}

		template.HasResourceProperties(jsii.String("AWS::Lambda::Function"),
			map[string]any{
				"Architectures": []string{config},
			},
		)
	}
}

func TestFunctionGoWithProps(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("Test"), nil)

	scud.NewFunctionGo(stack, jsii.String("test"),
		&scud.FunctionGoProps{
			SourceCodeModule: "github.com/fogfish/scud",
			SourceCodeLambda: "test/lambda/go",
			FunctionProps: &awslambda.FunctionProps{
				FunctionName: jsii.String("test"),
			},
		},
	)

	require := map[*string]*float64{
		jsii.String("AWS::IAM::Role"):        jsii.Number(2),
		jsii.String("AWS::Lambda::Function"): jsii.Number(2),
		jsii.String("Custom::LogRetention"):  jsii.Number(1),
	}

	template := assertions.Template_FromStack(stack, nil)
	for key, val := range require {
		template.ResourceCountIs(key, val)
	}
	template.HasResourceProperties(jsii.String("AWS::Lambda::Function"),
		map[string]interface{}{
			"FunctionName": "test",
		},
	)
}

func TestFunctionGoMany(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("Test"), nil)

	scud.NewFunctionGo(stack, jsii.String("test"),
		&scud.FunctionGoProps{
			SourceCodeModule: "github.com/fogfish/scud",
			SourceCodeLambda: "test/lambda/go",
			FunctionProps: &awslambda.FunctionProps{
				FunctionName: jsii.String("test"),
			},
		},
	)

	scud.NewFunctionGo(stack, jsii.String("another"),
		&scud.FunctionGoProps{
			SourceCodeModule: "github.com/fogfish/scud",
			SourceCodeLambda: "test/lambda/another",
			FunctionProps: &awslambda.FunctionProps{
				FunctionName: jsii.String("another"),
			},
		},
	)
}

func TestFunctionGoContainer(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("Test"), nil)

	scud.NewContainerGo(stack, jsii.String("test"),
		&scud.ContainerGoProps{
			SourceCodeModule: "github.com/fogfish/scud",
			SourceCodeLambda: "test/lambda/go",
			StaticAssets: []string{
				"test/lambda/go/main.go",
			},
		},
	)

	require := map[*string]*float64{
		jsii.String("AWS::IAM::Role"):        jsii.Number(2),
		jsii.String("AWS::Lambda::Function"): jsii.Number(2),
		jsii.String("Custom::LogRetention"):  jsii.Number(1),
	}

	template := assertions.Template_FromStack(stack, nil)
	for key, val := range require {
		template.ResourceCountIs(key, val)
	}
}

func TestCreateGateway(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("Test"), nil)

	scud.NewGateway(stack, jsii.String("GW"),
		&scud.GatewayProps{
			HttpApiProps: &awsapigatewayv2.HttpApiProps{
				ApiName: jsii.String("test"),
			},
		},
	)

	require := map[*string]*float64{
		jsii.String("AWS::ApiGatewayV2::Api"):   jsii.Number(1),
		jsii.String("AWS::ApiGatewayV2::Stage"): jsii.Number(2),
	}

	template := assertions.Template_FromStack(stack, nil)
	for key, val := range require {
		template.ResourceCountIs(key, val)
	}
	template.HasResourceProperties(jsii.String("AWS::ApiGatewayV2::Api"),
		map[string]interface{}{
			"Name": "test",
		},
	)
}

func TestAddResource(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("Test"), nil)

	f := scud.NewFunctionGo(stack, jsii.String("test"),
		&scud.FunctionGoProps{
			SourceCodeModule: "github.com/fogfish/scud",
			SourceCodeLambda: "test/lambda/go",
		},
	)

	gw := scud.NewGateway(stack, jsii.String("GW"), &scud.GatewayProps{})
	gw.AddResource("/test", f)

	require := map[*string]*float64{
		jsii.String("AWS::ApiGatewayV2::Api"):         jsii.Number(1),
		jsii.String("AWS::ApiGatewayV2::Stage"):       jsii.Number(2),
		jsii.String("AWS::ApiGatewayV2::Route"):       jsii.Number(1),
		jsii.String("AWS::ApiGatewayV2::Integration"): jsii.Number(1),
	}

	template := assertions.Template_FromStack(stack, nil)
	for key, val := range require {
		template.ResourceCountIs(key, val)
	}
}

func TestAddResourceDepthPath(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("Test"), nil)

	f := scud.NewFunctionGo(stack, jsii.String("test"),
		&scud.FunctionGoProps{
			SourceCodeModule: "github.com/fogfish/scud",
			SourceCodeLambda: "test/lambda/go",
		},
	)

	gw := scud.NewGateway(stack, jsii.String("GW"), &scud.GatewayProps{})
	gw.AddResource("/test/1", f)
	gw.AddResource("/test/2", f)

	require := map[*string]*float64{
		jsii.String("AWS::ApiGatewayV2::Api"):         jsii.Number(1),
		jsii.String("AWS::ApiGatewayV2::Stage"):       jsii.Number(2),
		jsii.String("AWS::ApiGatewayV2::Route"):       jsii.Number(2),
		jsii.String("AWS::ApiGatewayV2::Integration"): jsii.Number(2),
		jsii.String("AWS::Lambda::Function"):          jsii.Number(2),
	}

	template := assertions.Template_FromStack(stack, nil)
	for key, val := range require {
		template.ResourceCountIs(key, val)
	}
}

func TestAuthorizerIAM(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("Test"), nil)

	f := scud.NewFunctionGo(stack, jsii.String("test"),
		&scud.FunctionGoProps{
			SourceCodeModule: "github.com/fogfish/scud",
			SourceCodeLambda: "test/lambda/go",
		},
	)

	gw := scud.NewGateway(stack, jsii.String("GW"), &scud.GatewayProps{})
	gw.NewAuthorizerIAM().
		AddResource("/test", f, awsiam.NewAccountRootPrincipal())

	require := map[*string]*float64{
		jsii.String("AWS::ApiGatewayV2::Api"): jsii.Number(1),
	}

	template := assertions.Template_FromStack(stack, nil)
	for key, val := range require {
		template.ResourceCountIs(key, val)
	}
}

func TestAuthorizerCognito(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("Test"), nil)

	f := scud.NewFunctionGo(stack, jsii.String("test"),
		&scud.FunctionGoProps{
			SourceCodeModule: "github.com/fogfish/scud",
			SourceCodeLambda: "test/lambda/go",
		},
	)

	gw := scud.NewGateway(stack, jsii.String("GW"), &scud.GatewayProps{})
	gw.NewAuthorizerCognito("arn:aws:cognito-idp:eu-west-1:000000000000:userpool/eu-west-1_XXXXXXXXX").
		AddResource("/test", f, "test")

	require := map[*string]*float64{
		jsii.String("AWS::ApiGatewayV2::Authorizer"): jsii.Number(1),
	}

	template := assertions.Template_FromStack(stack, nil)
	for key, val := range require {
		template.ResourceCountIs(key, val)
	}
}

func TestAuthorizerJwt(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("Test"), nil)

	f := scud.NewFunctionGo(stack, jsii.String("test"),
		&scud.FunctionGoProps{
			SourceCodeModule: "github.com/fogfish/scud",
			SourceCodeLambda: "test/lambda/go",
		},
	)

	gw := scud.NewGateway(stack, jsii.String("GW"), &scud.GatewayProps{})
	gw.NewAuthorizerJwt("iss", "aud").
		AddResource("/test", f)

	require := map[*string]*float64{
		jsii.String("AWS::ApiGatewayV2::Authorizer"): jsii.Number(1),
	}

	template := assertions.Template_FromStack(stack, nil)
	for key, val := range require {
		template.ResourceCountIs(key, val)
	}
}

func TestConfigRoute53(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("Test"),
		&awscdk.StackProps{
			Env: &awscdk.Environment{
				Account: jsii.String("000000000000"),
				Region:  jsii.String("eu-west-1"),
			},
		},
	)

	scud.NewGateway(stack, jsii.String("GW"),
		&scud.GatewayProps{
			Host:   jsii.String("test.example.com"),
			TlsArn: jsii.String("arn:aws:acm:eu-west-1:000000000000:certificate/00000000-0000-0000-0000-000000000000"),
		},
	)

	require := map[*string]*float64{
		jsii.String("AWS::ApiGatewayV2::DomainName"): jsii.Number(1),
		jsii.String("AWS::Route53::RecordSet"):       jsii.Number(1),
	}

	template := assertions.Template_FromStack(stack, nil)
	for key, val := range require {
		template.ResourceCountIs(key, val)
	}
}
