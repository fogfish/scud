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
	"path/filepath"
	"strings"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	apigw2 "github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2"
	authorizers "github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2authorizers"
	integrations "github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2integrations"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscertificatemanager"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscognito"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53targets"

	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

type GatewayProps struct {
	*apigw2.HttpApiProps
	Host         *string
	TlsArn       *string
	AllowOrigins *string
}

type Gateway struct {
	constructs.Construct
	RestAPI apigw2.HttpApi
	domain  apigw2.DomainName
	aRecord awsroute53.ARecord
	methods *[]apigw2.HttpMethod
}

// NewGateway creates new instance of Gateway
func NewGateway(scope constructs.Construct, id *string, props *GatewayProps) *Gateway {
	gw := &Gateway{Construct: constructs.NewConstruct(scope, id)}
	if props.HttpApiProps == nil {
		props.HttpApiProps = &apigw2.HttpApiProps{}
	}

	if props.HttpApiProps.ApiName == nil {
		props.ApiName = awscdk.Aws_STACK_NAME()
	}

	if props.HttpApiProps.CorsPreflight == nil && props.AllowOrigins != nil {
		props.CorsPreflight = &apigw2.CorsPreflightOptions{
			AllowOrigins: &[]*string{props.AllowOrigins},
			AllowMethods: &[]apigw2.CorsHttpMethod{apigw2.CorsHttpMethod_ANY},
			AllowHeaders: jsii.Strings(
				"Authorization",
				"Content-Type",
			),
			MaxAge: awscdk.Duration_Minutes(jsii.Number(10)),
		}
	}

	if props.HttpApiProps.CorsPreflight != nil {
		gw.methods = &[]apigw2.HttpMethod{
			apigw2.HttpMethod_GET,
			apigw2.HttpMethod_HEAD,
			apigw2.HttpMethod_PATCH,
			apigw2.HttpMethod_POST,
			apigw2.HttpMethod_PUT,
			apigw2.HttpMethod_DELETE,
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

	gw.RestAPI = apigw2.NewHttpApi(gw.Construct, jsii.String("Gateway"), props.HttpApiProps)

	apigw2.NewHttpStage(gw.Construct, jsii.String("Stage"),
		&apigw2.HttpStageProps{
			AutoDeploy: jsii.Bool(true),
			StageName:  jsii.String("api"),
			HttpApi:    gw.RestAPI,
		},
	)

	if props.Host != nil && props.TlsArn != nil {
		gw.createRoute53(*props.Host)
	}

	return gw
}

func (gw *Gateway) createRoute53(host string) *Gateway {
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

// Creates public (unprotected) integration to authorize incoming requests.
// This integration allows unrestricted access to the associated API endpoints,
// enabling any client to interact with the resources and functionalities
// provided by your Lambda functions without requiring authentication or
// authorization. It is suitable for scenarios where open access is desired,
// such as public APIs or endpoints that do not handle sensitive data.
func (gw *Gateway) NewAuthorizerPublic() *AuthorizerPublic {
	return &AuthorizerPublic{
		RestAPI: gw.RestAPI,
		methods: gw.methods,
	}
}

// Creates integration with Basic Authorizer to authorize incoming requests.
// This integration implements a simple access/secret key validation mechanism,
// allowing you to protect your API endpoints using basic authentication.
// By validating the provided credentials against the configured access and
// secret keys, the library ensures that only authorized clients can access
// your resources, enhancing the security of your API.
func (gw *Gateway) NewAuthorizerBasic(access, secret string, source ...string) *AuthorizerBasic {
	src := "$request.header.Authorization"
	if len(source) > 0 {
		src = source[0]
	}

	f := NewFunctionGo(gw.Construct, jsii.String("AuthorizerBasic"),
		&FunctionGoProps{
			SourceCodeModule: "github.com/fogfish/scud",
			SourceCodeLambda: "internal/cmd/auth",
			FunctionProps: &awslambda.FunctionProps{
				Timeout: awscdk.Duration_Seconds(jsii.Number(5)),
				Environment: &map[string]*string{
					"CONFIG_AUTHORIZER_ACCESS": jsii.String(access),
					"CONFIG_AUTHORIZER_SECRET": jsii.String(secret),
					"CONFIG_AUTHORIZER_SOURCE": jsii.String(src),
				},
			},
		},
	)

	authorizer := authorizers.NewHttpLambdaAuthorizer(jsii.String("LambdaAuthorizer"), f,
		&authorizers.HttpLambdaAuthorizerProps{
			IdentitySource: jsii.Strings(src),
			// Note: enable for debug purposes only
			ResultsCacheTtl: awscdk.Duration_Seconds(jsii.Number(0)),
		},
	)

	return &AuthorizerBasic{
		RestAPI:    gw.RestAPI,
		authorizer: authorizer,
		methods:    gw.methods,
	}
}

// Creates integration with AWS IAM to authorize incoming requests.
// This integration ensures that only authenticated and authorized principals
// can access the resources and functionalities provided by your Lambda
// functions. By leveraging AWS IAM policies and roles, the library enforces
// fine-grained access control, enhancing the security of your API endpoints.
func (gw *Gateway) NewAuthorizerIAM() *AuthorizerIAM {
	return &AuthorizerIAM{
		RestAPI:    gw.RestAPI,
		authorizer: authorizers.NewHttpIamAuthorizer(),
		methods:    gw.methods,
	}
}

// Creates integration with AWS Cognito to authorize incoming requests.
// This integration allows you to manage user authentication and authorization
// seamlessly. By utilizing AWS Cognito, you can implement robust user
// sign-up, sign-in, and access control mechanisms. The library ensures that
// only authenticated users with valid tokens can access your API endpoints,
// providing an additional layer of security and user management capabilities.
func (gw *Gateway) NewAuthorizerCognito(cognitoArn string, clients ...string) *AuthorizerJwt {
	pool := awscognito.UserPool_FromUserPoolArn(
		gw.Construct,
		jsii.String("Cognito"),
		jsii.String(cognitoArn),
	)

	var allowList *[]awscognito.IUserPoolClient
	if len(clients) > 0 {
		al := make([]awscognito.IUserPoolClient, len(clients))
		for i, id := range clients {
			al[i] = awscognito.UserPoolClient_FromUserPoolClientId(
				gw.Construct,
				jsii.String(fmt.Sprintf("Client%d", i)),
				jsii.String(id),
			)
		}
		allowList = &al
	}

	authorizer := authorizers.NewHttpUserPoolAuthorizer(
		jsii.String("Authorizer"),
		pool,
		&authorizers.HttpUserPoolAuthorizerProps{
			IdentitySource:  jsii.Strings("$request.header.Authorization"),
			UserPoolClients: allowList,
		},
	)

	return &AuthorizerJwt{
		RestAPI:    gw.RestAPI,
		authorizer: authorizer,
		methods:    gw.methods,
	}
}

// Creates integration with Single Sign On provider (e.g. Auth0) to authorize
// incoming requests using JWT tokens. This integration allows you to leverage
// external identity providers for user authentication, ensuring secure and
// seamless access to your API endpoints. By validating JWT tokens issued by
// the SSO provider, the library ensures that only authenticated users can
// access your resources. Additionally, this setup can support various SSO
// standards and providers, enhancing flexibility and security in managing
// user identities and permissions.
func (gw *Gateway) NewAuthorizerJwt(iss string, aud ...string) *AuthorizerJwt {
	authorizer := authorizers.NewHttpJwtAuthorizer(
		jsii.String("Authorizer"),
		jsii.String(iss),
		&authorizers.HttpJwtAuthorizerProps{
			JwtAudience: jsii.Strings(aud...),
		},
	)

	return &AuthorizerJwt{
		RestAPI:    gw.RestAPI,
		authorizer: authorizer,
		methods:    gw.methods,
	}
}

//------------------------------------------------------------------------------

type AuthorizerPublic struct {
	RestAPI apigw2.HttpApi
	methods *[]apigw2.HttpMethod
}

// Associate a Lambda function with a REST API path. It uses the specified
// path as a prefix, enabling the association of the Lambda function with
// all subpaths under that prefix.
func (api *AuthorizerPublic) AddResource(
	endpoint string,
	handler awslambda.Function,
) *AuthorizerPublic {
	lambda := integrations.NewHttpLambdaIntegration(
		jsii.String(filepath.Base(endpoint)),
		handler,
		&integrations.HttpLambdaIntegrationProps{
			PayloadFormatVersion: apigw2.PayloadFormatVersion_VERSION_1_0(),
		},
	)

	for _, path := range []string{endpoint, endpoint + "/{any+}"} {
		api.RestAPI.AddRoutes(&apigw2.AddRoutesOptions{
			Path:        jsii.String(path),
			Integration: lambda,
			Methods:     api.methods,
		})
	}

	return api
}

//------------------------------------------------------------------------------

type AuthorizerBasic struct {
	RestAPI    apigw2.HttpApi
	authorizer authorizers.HttpLambdaAuthorizer
	methods    *[]apigw2.HttpMethod
}

// Associate a Lambda function with a REST API path. It uses the specified
// path as a prefix, enabling the association of the Lambda function with
// all subpaths under that prefix.
func (api *AuthorizerBasic) AddResource(
	endpoint string,
	handler awslambda.Function,
) *AuthorizerBasic {
	lambda := integrations.NewHttpLambdaIntegration(
		jsii.String(filepath.Base(endpoint)),
		handler,
		&integrations.HttpLambdaIntegrationProps{
			PayloadFormatVersion: apigw2.PayloadFormatVersion_VERSION_1_0(),
		},
	)

	for _, path := range []string{endpoint, endpoint + "/{any+}"} {
		api.RestAPI.AddRoutes(&apigw2.AddRoutesOptions{
			Path:        jsii.String(path),
			Integration: lambda,
			Authorizer:  api.authorizer,
			Methods:     api.methods,
		})
	}

	return api
}

//------------------------------------------------------------------------------

type AuthorizerIAM struct {
	constructs.Construct
	RestAPI    apigw2.HttpApi
	authorizer apigw2.IHttpRouteAuthorizer
	methods    *[]apigw2.HttpMethod
}

// Associate a Lambda function with a REST API path. It uses the specified
// path as a prefix, enabling the association of the Lambda function with
// all subpaths under that prefix.
//
// Protect access to resource only for AWS IAM principals.
func (api *AuthorizerIAM) AddResource(
	endpoint string,
	handler awslambda.Function,
	grantee awsiam.IGrantable,
) *AuthorizerIAM {
	lambda := integrations.NewHttpLambdaIntegration(
		jsii.String(filepath.Base(endpoint)),
		handler,
		&integrations.HttpLambdaIntegrationProps{
			PayloadFormatVersion: apigw2.PayloadFormatVersion_VERSION_1_0(),
		},
	)

	for _, path := range []string{endpoint, endpoint + "/{any+}"} {
		routes := api.RestAPI.AddRoutes(&apigw2.AddRoutesOptions{
			Path:        jsii.String(path),
			Integration: lambda,
			Authorizer:  api.authorizer,
			Methods:     api.methods,
		})
		(*routes)[0].GrantInvoke(grantee, nil)
	}

	return api
}

//------------------------------------------------------------------------------

type AuthorizerJwt struct {
	constructs.Construct
	RestAPI    apigw2.HttpApi
	authorizer apigw2.IHttpRouteAuthorizer
	methods    *[]apigw2.HttpMethod
}

// Associate a Lambda function with a REST API path. It uses the specified
// path as a prefix, enabling the association of the Lambda function with
// all subpaths under that prefix.
//
// Protect access to resource only for principals with valid JWT token.
func (api *AuthorizerJwt) AddResource(
	endpoint string,
	handler awslambda.Function,
	accessScope ...string,
) *AuthorizerJwt {
	lambda := integrations.NewHttpLambdaIntegration(
		jsii.String(filepath.Base(endpoint)),
		handler,
		&integrations.HttpLambdaIntegrationProps{
			PayloadFormatVersion: apigw2.PayloadFormatVersion_VERSION_1_0(),
		},
	)

	for _, path := range []string{endpoint, endpoint + "/{any+}"} {
		api.RestAPI.AddRoutes(&apigw2.AddRoutesOptions{
			Path:                jsii.String(path),
			Integration:         lambda,
			Authorizer:          api.authorizer,
			AuthorizationScopes: jsii.Strings(accessScope...),
			Methods:             api.methods,
		})
	}

	return api
}

//------------------------------------------------------------------------------

/*
type AuthorizerUniversal struct {
	constructs.Construct
	RestAPI    apigw2.HttpApi
	authorizer apigw2.IHttpRouteAuthorizer
}

func (api *AuthorizerUniversal) AddResource(
	endpoint string,
	handler awslambda.Function,
) *AuthorizerUniversal {
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
		Authorizer:  api.authorizer,
	}

	api.RestAPI.AddRoutes(opts)

	return api
}
*/
