package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/jsii-runtime-go"
	"github.com/fogfish/scud"
)

func main() {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("example"), nil)

	scud.NewFunctionGo(stack, jsii.String("MyFun"),
		&scud.FunctionGoProps{
			SourceCodeModule: "github.com/fogfish/scud",
			SourceCodeLambda: "examples/01_simple_function/lambda",
		},
	)

	app.Synth(nil)
}
