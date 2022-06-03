//
// Copyright (C) 2020 Dmitry Kolesnikov
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
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigateway"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/jsii-runtime-go"
	"github.com/fogfish/scud"
)

func TestFunctionGo(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("Test"), nil)

	scud.NewFunctionGo(stack, jsii.String("test"),
		&scud.FunctionGoProps{
			SourceCodePackage: "github.com/fogfish/scud",
			SourceCodeLambda:  "test/lambda/go",
		},
	)

	require := map[*string]*float64{
		jsii.String("AWS::IAM::Role"):        jsii.Number(2),
		jsii.String("AWS::Lambda::Function"): jsii.Number(2),
		jsii.String("Custom::LogRetention"):  jsii.Number(1),
	}

	template := assertions.Template_FromStack(stack)
	for key, val := range require {
		template.ResourceCountIs(key, val)
	}
}

func TestFunctionGoWithProps(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("Test"), nil)

	scud.NewFunctionGo(stack, jsii.String("test"),
		&scud.FunctionGoProps{
			SourceCodePackage: "github.com/fogfish/scud",
			SourceCodeLambda:  "test/lambda/go",
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

	template := assertions.Template_FromStack(stack)
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
			SourceCodePackage: "github.com/fogfish/scud",
			SourceCodeLambda:  "test/lambda/go",
			FunctionProps: &awslambda.FunctionProps{
				FunctionName: jsii.String("test"),
			},
		},
	)

	scud.NewFunctionGo(stack, jsii.String("another"),
		&scud.FunctionGoProps{
			SourceCodePackage: "github.com/fogfish/scud",
			SourceCodeLambda:  "test/lambda/another",
			FunctionProps: &awslambda.FunctionProps{
				FunctionName: jsii.String("another"),
			},
		},
	)
}

func TestCreateGateway(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("Test"), nil)

	scud.NewGateway(stack, jsii.String("GW"),
		&awsapigateway.RestApiProps{
			RestApiName: jsii.String("test"),
		},
	)

	require := map[*string]*float64{
		jsii.String("AWS::ApiGateway::RestApi"):    jsii.Number(1),
		jsii.String("AWS::ApiGateway::Deployment"): jsii.Number(1),
		jsii.String("AWS::ApiGateway::Stage"):      jsii.Number(1),
		jsii.String("AWS::ApiGateway::Method"):     jsii.Number(1),
	}

	template := assertions.Template_FromStack(stack)
	for key, val := range require {
		template.ResourceCountIs(key, val)
	}
	template.HasResourceProperties(jsii.String("AWS::ApiGateway::RestApi"),
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
			SourceCodePackage: "github.com/fogfish/scud",
			SourceCodeLambda:  "test/lambda/go",
		},
	)

	scud.NewGateway(stack, jsii.String("GW"), nil).
		AddResource("test", f)

	require := map[*string]*float64{
		jsii.String("AWS::ApiGateway::RestApi"):    jsii.Number(1),
		jsii.String("AWS::ApiGateway::Deployment"): jsii.Number(1),
		jsii.String("AWS::ApiGateway::Stage"):      jsii.Number(1),
		jsii.String("AWS::ApiGateway::Method"):     jsii.Number(5),
		jsii.String("AWS::ApiGateway::Resource"):   jsii.Number(2),
		jsii.String("AWS::Lambda::Function"):       jsii.Number(2),
	}

	template := assertions.Template_FromStack(stack)
	for key, val := range require {
		template.ResourceCountIs(key, val)
	}

}

func TestConfigAuthorizer(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("Test"), nil)

	f := scud.NewFunctionGo(stack, jsii.String("test"),
		&scud.FunctionGoProps{
			SourceCodePackage: "github.com/fogfish/scud",
			SourceCodeLambda:  "test/lambda/go",
		},
	)

	scud.NewGateway(stack, jsii.String("GW"), nil).
		ConfigAuthorizer("arn:aws:cognito-idp:eu-west-1:000000000000:userpool/eu-west-1_XXXXXXXXX").
		AddResource("test", f, "test")

	require := map[*string]*float64{
		jsii.String("AWS::ApiGateway::Authorizer"): jsii.Number(1),
	}

	template := assertions.Template_FromStack(stack)
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

	scud.NewGateway(stack, jsii.String("GW"), nil).
		ConfigRoute53("test.example.com", "arn:aws:acm:eu-west-1:000000000000:certificate/00000000-0000-0000-0000-000000000000")

	require := map[*string]*float64{
		jsii.String("AWS::ApiGateway::DomainName"): jsii.Number(1),
		jsii.String("AWS::Route53::RecordSet"):     jsii.Number(1),
	}

	template := assertions.Template_FromStack(stack)
	for key, val := range require {
		template.ResourceCountIs(key, val)
	}
}
