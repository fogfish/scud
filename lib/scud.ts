//
// Copyright (C) 2020 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/scud
//
import * as cdk from '@aws-cdk/core'
import * as lambda from '@aws-cdk/aws-lambda'
import * as pure from 'aws-cdk-pure'
import * as api from '@aws-cdk/aws-apigateway'

/*

REST API Gateway
*/
export const Gateway = (
  props: Partial<api.RestApiProps>
): api.RestApiProps => ({
  ...props,

  deploy: true,
  deployOptions: { stageName: 'api' },
  endpointTypes: [api.EndpointType.REGIONAL],
  failOnWarnings: true,
  defaultCorsPreflightOptions: {
    allowOrigins: api.Cors.ALL_ORIGINS,
    maxAge: cdk.Duration.minutes(10),
  },
})

/*

OAuth2 enables authorization of service requests
*/
const OAuth2 = (
  gateway: api.RestApi,
  cognitoUserPools: string[],
): pure.IPure<api.CfnAuthorizer> => {
  const Authorizer = () : api.CfnAuthorizerProps => ({
    type: api.AuthorizationType.COGNITO,
    name: `${cdk.Aws.STACK_NAME}-oauth2`,
    identitySource: "method.request.header.Authorization",
    providerArns: cognitoUserPools,
    restApiId: gateway.restApiId,
  })
  return pure.iaac(api.CfnAuthorizer)(Authorizer)
}

/*

requireOAuth2 enforces a policy to REST API endpoint
*/
const requireOAuth2 = (
  authorizerId: string,
  scopes: string[]
): api.MethodOptions => ({
  authorizer: { authorizerId },
  authorizationType: api.AuthorizationType.COGNITO,
  requestParameters: {
    "method.request.header.Authorization": true,
  },
  authorizationScopes: scopes
})


/*

Service ...
*/
type Effect = pure.IEffect<{
  gateway: api.RestApi
  authorizer?: api.CfnAuthorizer
}>

export type Service = Effect & {
  enableOAuth2(cognitoUserPools: string[]): Service
  addResource(path: string, handler: pure.IPure<lambda.Function>, scopes?: string[]): Service
}

export const mkService = (
  gateway: () => api.RestApiProps,
): Service =>
  effectService(
    pure.use({
      gateway: pure.iaac(api.RestApi)(gateway)
    })
  )

function effectService(eff: Effect): Service {
  const service = eff as Service

  service.enableOAuth2 = (
    cognitoUserPools: string[],
  ): Service =>
    effectService(
      eff.flatMap(
        ({ gateway }) => ({
          authorizer: OAuth2(gateway, cognitoUserPools),
        })
      )
    )

  service.addResource = (
    resourceRootPath: string,
    resourceHandler: pure.IPure<lambda.Function>,
    resourceScopes?: string[],
  ): Service =>
    effectService(
      eff.flatMap(
        _ => ({ h: pure.wrap(api.LambdaIntegration)(resourceHandler) })
      ).effect( ({ gateway, authorizer, h }) => {
        const require = (authorizer && resourceScopes) && requireOAuth2(authorizer.ref, resourceScopes)

        const root = gateway.root.addResource(resourceRootPath)
        root.addMethod('ANY', h, require)
        root.addResource('{any+}').addMethod('ANY', h, require)
      })
    )

  return service
}
