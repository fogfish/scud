//
// Copyright (C) 2020 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/scud
//

package scud

import (
	"path/filepath"
	"strings"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigateway"
	apigw2 "github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2"
	authorizers "github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2authorizers"
	integrations "github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2integrations"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscertificatemanager"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscognito"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53targets"

	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// Gateway is RESTful API Gateway Construct pattern
type Gateway interface {
	constructs.Construct
	RestApi() apigw2.HttpApi
	WithAuthorizerIAM()
	WithAuthorizerCognito(cognitoArn string)
	AddResource(resourceRootPath string, resourceHandler awslambda.Function, requiredAccessScope ...string)
}

type GatewayProps struct {
	*apigw2.HttpApiProps
	Host   *string
	TlsArn *string
}

type gateway struct {
	constructs.Construct
	restapi      apigw2.HttpApi
	domain       apigw2.DomainName
	authorizer   apigw2.IHttpRouteAuthorizer
	enableScopes bool
	aRecord      awsroute53.ARecord
}

// NewGateway creates new instance of Gateway
func NewGateway(scope constructs.Construct, id *string, props *GatewayProps) Gateway {
	gw := &gateway{Construct: constructs.NewConstruct(scope, id)}
	if props.HttpApiProps == nil {
		props.HttpApiProps = &apigw2.HttpApiProps{}
	}

	if props.HttpApiProps.ApiName == nil {
		props.ApiName = awscdk.Aws_STACK_NAME()
	}

	if props.HttpApiProps.CorsPreflight == nil {
		props.CorsPreflight = &apigw2.CorsPreflightOptions{
			AllowOrigins: awsapigateway.Cors_ALL_ORIGINS(),
			MaxAge:       awscdk.Duration_Minutes(jsii.Number(10)),
		}
	}

	if props.Host != nil && props.TlsArn != nil {
		gw.domain = apigw2.NewDomainName(gw.Construct, jsii.String("DomainName"),
			&apigw2.DomainNameProps{
				EndpointType: apigw2.EndpointType_REGIONAL,
				DomainName:   props.Host,
				Certificate:  awscertificatemanager.Certificate_FromCertificateArn(gw.Construct, jsii.String("X509"), props.TlsArn),
			},
		)

		props.HttpApiProps.DefaultDomainMapping = &apigw2.DomainMappingOptions{
			DomainName: gw.domain,
		}
	}

	gw.restapi = apigw2.NewHttpApi(gw.Construct, jsii.String("Gateway"), props.HttpApiProps)

	apigw2.NewHttpStage(gw.Construct, jsii.String("Stage"),
		&apigw2.HttpStageProps{
			AutoDeploy: jsii.Bool(true),
			StageName:  jsii.String("api"),
			HttpApi:    gw.restapi,
		},
	)

	if props.Host != nil && props.TlsArn != nil {
		gw.createRoute53(*props.Host)
	}

	return gw
}

func (gw *gateway) RestApi() apigw2.HttpApi { return gw.restapi }

func (gw *gateway) createRoute53(host string) *gateway {
	domain := strings.Join(strings.Split(host, ".")[1:], ".")
	zone := awsroute53.HostedZone_FromLookup(gw.Construct, jsii.String("HZone"),
		&awsroute53.HostedZoneProviderProps{
			DomainName: jsii.String(domain),
		},
	)

	gw.aRecord = awsroute53.NewARecord(gw.Construct, jsii.String("ARecord"),
		&awsroute53.ARecordProps{
			RecordName: jsii.String(host),
			Target: awsroute53.RecordTarget_FromAlias(
				awsroute53targets.NewApiGatewayv2DomainProperties(gw.domain.RegionalDomainName(), gw.domain.RegionalHostedZoneId()),
			),
			Ttl:  awscdk.Duration_Seconds(jsii.Number(60)),
			Zone: zone,
		},
	)

	return gw
}

// Enable IAM Authorizer
func (gw *gateway) WithAuthorizerIAM() {
	gw.enableScopes = false
	gw.authorizer = authorizers.NewHttpIamAuthorizer()
}

// Enable AWS Cognito Authorizer
func (gw *gateway) WithAuthorizerCognito(cognitoArn string) {
	pool := awscognito.UserPool_FromUserPoolArn(
		gw.Construct,
		jsii.String("Cognito"),
		jsii.String(cognitoArn),
	)

	gw.enableScopes = true
	gw.authorizer = authorizers.NewHttpUserPoolAuthorizer(
		jsii.String("Authorizer"),
		pool,
		&authorizers.HttpUserPoolAuthorizerProps{
			IdentitySource: jsii.Strings("$request.header.Authorization"),
		},
	)
}

// Add resource
func (gw *gateway) AddResource(
	endpoint string,
	handler awslambda.Function,
	accessScope ...string,
) {
	lambda := integrations.NewHttpLambdaIntegration(
		jsii.String(filepath.Base(endpoint)),
		handler,
		&integrations.HttpLambdaIntegrationProps{
			PayloadFormatVersion: apigw2.PayloadFormatVersion_VERSION_1_0(),
		},
	)

	opts := &apigw2.AddRoutesOptions{
		Path:        jsii.String(endpoint + "/{any+}"),
		Integration: lambda,
	}
	if gw.authorizer != nil {
		opts.Authorizer = gw.authorizer
		if gw.enableScopes {
			opts.AuthorizationScopes = jsii.Strings(accessScope...)
		}
	}

	gw.restapi.AddRoutes(opts)
}
