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
	"path/filepath"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

/*

FunctionGoProps is properties of the function
*/
type FunctionGoProps struct {
	*awslambda.FunctionProps
	SourceCodePackage string
	SourceCodeLambda  string
}

/*

NewFunctionGo creates Golang Lambda Function from "inline" code
*/
func NewFunctionGo(scope constructs.Construct, id *string, props *FunctionGoProps) awslambda.Function {
	var lprops awslambda.FunctionProps
	if props.FunctionProps != nil {
		lprops = *props.FunctionProps
	}

	if lprops.Timeout == nil {
		lprops.Timeout = awscdk.Duration_Minutes(jsii.Number(1))
	}

	if lprops.LogRetention == "" {
		lprops.LogRetention = awslogs.RetentionDays_FIVE_DAYS
	}

	lprops.Code = AssetCodeGo(props.SourceCodePackage, props.SourceCodeLambda)
	lprops.Handler = jsii.String("main")
	lprops.Runtime = awslambda.Runtime_GO_1_X()
	lprops.FunctionName = jsii.String(fmt.Sprintf("%s%s", *awscdk.Aws_STACK_ID(), filepath.Base(props.SourceCodeLambda)))

	return awslambda.NewFunction(scope, id, &lprops)
}
