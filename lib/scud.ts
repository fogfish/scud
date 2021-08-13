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
import * as dns from '@aws-cdk/aws-route53'
import * as target from '@aws-cdk/aws-route53-targets'
import * as acm from '@aws-cdk/aws-certificatemanager'

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

HostedZone is helper component
*/
const HostedZone = (domainName: string): pure.IPure<dns.IHostedZone> => {
  const awscdkIssue4592 = (parent: cdk.Construct, id: string, props: dns.HostedZoneProviderProps): dns.IHostedZone => (
    dns.HostedZone.fromLookup(parent, id, props)
  )
  const iaac = pure.include(awscdkIssue4592) // dns.HostedZone.fromLookup
  const SiteHostedZone = (): dns.HostedZoneProviderProps => ({ domainName })
  return iaac(SiteHostedZone)
}

/*

Certificate is helper component
*/
const Certificate = (arn: string): pure.IPure<acm.ICertificate> => {
  const wrap = pure.include(acm.Certificate.fromCertificateArn)
  const SiteTLS = (): string => arn
  return wrap(SiteTLS)
}

/*

Service ...
*/
type Effect = pure.IEffect<{
  gateway: api.RestApi
  authorizer?: api.CfnAuthorizer
  dns?: dns.ARecord
}>

export type Service = Effect & {
  configOAuth2(cognitoUserPools: string[]): Service
  configRoute53(host: string, tlsArn: string): Service
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

  service.configOAuth2 = (
    cognitoUserPools: string[],
  ): Service =>
    effectService(
      eff.flatMap(
        ({ gateway }) => ({
          authorizer: OAuth2(gateway, cognitoUserPools),
        })
      )
    )

  service.configRoute53 = (
    host: string,
    tlsArn: string,
  ): Service =>
    effectService(
      eff.flatMap(
        ({ gateway }) => ({
          dns: Certificate(tlsArn)
            .effect((tls) => gateway.addDomainName('DName', { domainName: host, certificate: tls }))
            .flatMap(_ => HostedZone(host.split('.').splice(1).join('.')))
            .flatMap(zone => {
              const DNS  = (): dns.ARecordProps => ({
                recordName: host,
                target: { aliasTarget: new target.ApiGateway(gateway) },
                ttl: cdk.Duration.seconds(60),
                zone,
              })
              return pure.iaac(dns.ARecord)(DNS)
            })
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
        () => ({ h: pure.wrap(api.LambdaIntegration)(resourceHandler) })
      ).effect( ({ gateway, authorizer, h }) => {
        const require = (authorizer && resourceScopes) && requireOAuth2(authorizer.ref, resourceScopes)

        const root = gateway.root.addResource(resourceRootPath)
        root.addMethod('ANY', h, require)
        root.addResource('{any+}').addMethod('ANY', h, require)
      })
    )

  return service
}
