//
// Copyright (C) 2020 - 2024 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/scud
//

package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/jsii-runtime-go"
	"github.com/fogfish/scud"
)

func main() {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("example-api-auth-iam"), nil)

	gw := scud.NewGateway(stack, jsii.String("Gateway"),
		&scud.GatewayProps{},
	)

	f := scud.NewFunctionGo(stack, jsii.String("test"),
		&scud.FunctionGoProps{
			SourceCodeModule: "github.com/fogfish/scud",
			SourceCodeLambda: "test/lambda/go",
		},
	)

	// Public endpoint
	gw.AddResource("/public", f)

	// IAM authorization
	gw.NewAuthorizerIAM().
		AddResource("/private/iam/hw", f, awsiam.NewAccountRootPrincipal())

	// Auth0 authorization
	gw.NewAuthorizerJwt("https://tenant.eu.auth0.com/", "https://audience.com").
		AddResource("/private/jwt/hw", f, "xx", "yy")

	app.Synth(nil)
}
