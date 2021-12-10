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
	"strings"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigateway"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscertificatemanager"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscognito"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53targets"

	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

/*

Gateway is RESTful API Gateway Construct pattern
*/
type Gateway interface {
	ConfigRoute53(host string, tlsArn string) Gateway
	ConfigAuthorizer(cognitoUserPools ...string) Gateway
	AddResource(resourceRootPath string, resourceHandler awslambda.Function, requiredAccessScope ...string) Gateway
}

type gateway struct {
	constructs.Construct
	restapi    awsapigateway.RestApi
	authorizer awsapigateway.IAuthorizer
	x509       awscertificatemanager.ICertificate
	aRecord    awsroute53.ARecord
}

/*

NewGateway creates new instance of Gateway
*/
func NewGateway(scope constructs.Construct, id *string) Gateway {
	gw := &gateway{Construct: constructs.NewConstruct(scope, id)}
	return gw.mkGateway()
}

func (gw *gateway) mkGateway() Gateway {
	id := jsii.String("Gateway")
	gw.restapi = awsapigateway.NewRestApi(gw.Construct, id,
		&awsapigateway.RestApiProps{
			Deploy: jsii.Bool(true),
			DeployOptions: &awsapigateway.StageOptions{
				StageName: jsii.String("api"),
			},
			EndpointTypes:  &[]awsapigateway.EndpointType{awsapigateway.EndpointType_REGIONAL},
			FailOnWarnings: jsii.Bool(true),

			DefaultCorsPreflightOptions: &awsapigateway.CorsOptions{
				AllowOrigins: awsapigateway.Cors_ALL_ORIGINS(),
				MaxAge:       awscdk.Duration_Minutes(jsii.Number(10)),
			},
		},
	)

	return gw
}

/*

ConfigRoute53 deploys custom domain name for gateway
*/
func (gw *gateway) ConfigRoute53(host string, tlsArn string) Gateway {
	return gw.
		mkX509(tlsArn).
		mkDomainName(host).
		mkRoute53(host)
}

func (gw *gateway) mkX509(tlsArn string) *gateway {
	id := jsii.String("X509")
	gw.x509 = awscertificatemanager.Certificate_FromCertificateArn(gw.Construct, id, jsii.String(tlsArn))

	return gw
}

func (gw *gateway) mkDomainName(host string) *gateway {
	gw.restapi.AddDomainName(jsii.String("HostName"),
		&awsapigateway.DomainNameOptions{
			DomainName:  jsii.String(host),
			Certificate: gw.x509,
		},
	)

	return gw
}

func (gw *gateway) mkRoute53(host string) *gateway {
	domain := strings.Join(strings.Split(host, ".")[1:], ".")
	zone := awsroute53.HostedZone_FromLookup(gw.Construct, jsii.String("HZone"),
		&awsroute53.HostedZoneProviderProps{
			DomainName: jsii.String(domain),
		},
	)

	gw.aRecord = awsroute53.NewARecord(gw.Construct, jsii.String("ARecord"),
		&awsroute53.ARecordProps{
			RecordName: jsii.String(host),
			Target:     awsroute53.NewRecordTarget(nil, awsroute53targets.NewApiGateway(gw.restapi)),
			Ttl:        awscdk.Duration_Seconds(jsii.Number(60)),
			Zone:       zone,
		},
	)

	return gw
}

/*

ConfigAuthorizer deploys AWS Cognito authorizer for gateway
*/
func (gw *gateway) ConfigAuthorizer(cognitoUserPools ...string) Gateway {
	pools := make([]awscognito.IUserPool, 0)
	for i, pool := range cognitoUserPools {
		pools = append(pools,
			awscognito.UserPool_FromUserPoolArn(
				gw.Construct,
				jsii.String(fmt.Sprintf("AuthPool%d", i)),
				jsii.String(pool),
			),
		)
	}

	gw.authorizer = awsapigateway.NewCognitoUserPoolsAuthorizer(gw.Construct, jsii.String("Auth"),
		&awsapigateway.CognitoUserPoolsAuthorizerProps{
			CognitoUserPools: &pools,
			IdentitySource:   jsii.String("method.request.header.Authorization"),
		},
	)

	return gw
}

/*

AddResource creates a new handler to gateway
*/
func (gw *gateway) AddResource(
	endpoint string,
	handler awslambda.Function,
	accessScope ...string,
) Gateway {
	lambda := awsapigateway.NewLambdaIntegration(handler, nil)

	opts := &awsapigateway.MethodOptions{}
	if gw.authorizer != nil && len(accessScope) > 0 {
		opts.AuthorizationType = awsapigateway.AuthorizationType_COGNITO
		opts.RequestParameters = &map[string]*bool{
			"method.request.header.Authorization": jsii.Bool(true),
		}
		opts.AuthorizationScopes = jsii.Strings(accessScope...)
		opts.Authorizer = gw.authorizer
	}

	rsc := gw.restapi.Root().AddResource(jsii.String(endpoint), nil)
	rsc.AddMethod(jsii.String("ANY"), lambda, opts)

	sub := rsc.AddResource(jsii.String("{any+}"), nil)
	sub.AddMethod(jsii.String("ANY"), lambda, opts)

	return gw
}
