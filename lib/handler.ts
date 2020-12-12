//
// Copyright (C) 2020 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/scud
//
import * as cdk from '@aws-cdk/core'
import * as lambda from '@aws-cdk/aws-lambda'
import * as path from 'path'
import * as logs from '@aws-cdk/aws-logs'
import { AssetCodeGo } from './runtime'

/*

Props is a type of handler function
*/
export type Props = Partial<Omit<lambda.FunctionProps, "code">> & {
  code: string
}

/*

Go(lang) handler function
*/
export const Go = (props: Props): lambda.FunctionProps => ({
  ...props,

  code: AssetCodeGo(props.code),
  handler: 'main',
  runtime: lambda.Runtime.GO_1_X,
  functionName: `${cdk.Aws.STACK_NAME}-${path.basename(props.code)}`,

  logRetention: props.logRetention || logs.RetentionDays.FIVE_DAYS,
  timeout: props.timeout || cdk.Duration.minutes(1),
})
