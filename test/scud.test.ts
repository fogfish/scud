//
// Copyright (C) 2020 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/scud
//
import * as assert from '@aws-cdk/assert'
import * as api from '@aws-cdk/aws-apigateway'
import * as lambda from '@aws-cdk/aws-lambda'
import * as scud from '../lib'
import * as cdk from '@aws-cdk/core'
import * as pure from 'aws-cdk-pure'

const Gateway = (): api.RestApiProps => scud.Gateway({
  restApiName: 'test',
})

const Handler = (): lambda.FunctionProps =>
  scud.handler.Go({
    sourceCodePackage: './test/lambda/go',
    sourceCodeLambda: '.',
  })

//
//
it('create REST API gateway', () => {
  const stack = new cdk.Stack()

  pure.join(stack,
    scud.mkService(Gateway),
  )

  const requires: {[key: string]: number} = {
    'AWS::ApiGateway::RestApi': 1,
    'AWS::ApiGateway::Deployment': 1,
    'AWS::ApiGateway::Stage': 1,
    'AWS::ApiGateway::Method': 1,
  }

  Object.keys(requires).forEach(
    key => assert.expect(stack).to(
      assert.countResources(key, requires[key])
    )
  )
})

//
//
it('enables cognito based authorizer', () => {
  const stack = new cdk.Stack()

  pure.join(stack,
    scud.mkService(Gateway)
      .enableOAuth2([
        "arn:aws:cognito-idp:eu-west-1:000000000000:userpool/eu-west-1_XXXXXXXXX",
      ])
  )

  const requires: {[key: string]: number} = {
    'AWS::ApiGateway::Authorizer': 1,
  }

  Object.keys(requires).forEach(
    key => assert.expect(stack).to(
      assert.countResources(key, requires[key])
    )
  )
})

//
//
it('builds lambda function', () => {
  const stack = new cdk.Stack()

  pure.join(stack,
    scud.aws.Lambda(Handler)
  )

  const requires: {[key: string]: number} = {
    'AWS::IAM::Role': 2,
    'AWS::Lambda::Function': 2,
    'Custom::LogRetention': 1,
  }

  Object.keys(requires).forEach(
    key => assert.expect(stack).to(
      assert.countResources(key, requires[key])
    )
  )
})

//
//
it('create REST API resource', () => {
  const stack = new cdk.Stack()

  pure.join(stack,
    scud.mkService(Gateway)
      .addResource('test', scud.aws.Lambda(Handler))
  )

  const requires: {[key: string]: number} = {
    'AWS::ApiGateway::RestApi': 1,
    'AWS::ApiGateway::Deployment': 1,
    'AWS::ApiGateway::Stage': 1,
    'AWS::ApiGateway::Method': 5,
    'AWS::ApiGateway::Resource': 2,
    'AWS::Lambda::Function': 2,
  }

  Object.keys(requires).forEach(
    key => assert.expect(stack).to(
      assert.countResources(key, requires[key])
    )
  )
})
