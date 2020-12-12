//
// Copyright (C) 2020 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/scud
//
import * as lambda from '@aws-cdk/aws-lambda'
import * as pure from 'aws-cdk-pure'

export const Lambda = pure.iaac(lambda.Function)
