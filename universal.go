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

	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/constructs-go/constructs/v10"
)

type FunctionProps interface {
	HKT1(awslambda.Function)
	UniqueID() string
}

func NewFunction(scope constructs.Construct, id *string, spec FunctionProps) awslambda.Function {
	switch prop := spec.(type) {
	case *FunctionGoProps:
		return NewFunctionGo(scope, id, prop)
	case *ContainerGoProps:
		return NewContainerGo(scope, id, prop)
	}

	panic(fmt.Errorf("not supported type %T", spec))
}
