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
	"github.com/aws/aws-cdk-go/awscdk/v2/awscognito"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsssm"

	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

type GatewayProps struct {
	*apigw2.HttpApiProps

	// Custom domain name of the API (e.g. v1.api.example.com), requires TlsArn.
	Host *string

	// ARN of the certificate for the custom domain name.
	TlsArn *string

	// Hosted zone (domain name) where the custom domain record is created.
	// Default is the Host without its first label
	// (e.g. api.example.com for v1.api.example.com).
	// The zone is resolved with Route 53 lookup at synthesis,
	// use HostedZone to avoid the lookup.
	Zone *string

	// Hosted zone where the custom domain record is created
	// (e.g. HostedZone_FromHostedZoneAttributes). It takes precedence over Zone,
	// no lookup is made at synthesis.
	HostedZone awsroute53.IHostedZone

	// Shortcut to enable CORS for the given origin with default headers.
	// Use Cors for full control.
	AllowOrigins *string

	// CORS configuration, ignored if HttpApiProps.CorsPreflight is defined.
	Cors *Cors

	// Access logging of the default stage
	AccessLog *AccessLog

	// Throttling of the default stage (all routes). Route specific throttling
	// is defined by RouteProps.Throttle.
	Throttle *Throttle

	// Name of SSM parameter, the API id is exported to (e.g. /myapp/v1/api).
	// Use it to refer the versioned API from another stack (see NewAlias).
	ApiIdParameter *string
}

// CORS configuration of the HTTP API
type Cors struct {
	// Origins allowed to access the API
	AllowOrigins []string

	// Headers allowed in the request, default is Authorization, Content-Type
	AllowHeaders []string

	// Headers exposed to browser in the response
	ExposeHeaders []string

	// Allow credentials (cookies, authorization headers) in cross-origin requests
	AllowCredentials bool

	// For how long browser caches the preflight response, default is 10 minutes
	MaxAge awscdk.Duration
}

// Throttling of the API
type Throttle struct {
	// The steady-state rate limit (requests per second).
	RateLimit float64
	// The maximum number of concurrent requests.
	BurstLimit float64
}

// Access logging of the API
type AccessLog struct {
	// Log group to write access logs, created if not defined.
	// The retention of created log group follows scud.Defaults.
	LogGroup awslogs.ILogGroup

	// Log format, default is DefaultAccessLogFormat.
	Format *string
}

// DefaultAccessLogFormat is the JSON access log format used by default.
const DefaultAccessLogFormat = `{"requestId":"$context.requestId","ip":"$context.identity.sourceIp","requestTime":"$context.requestTime","httpMethod":"$context.httpMethod","routeKey":"$context.routeKey","path":"$context.path","status":"$context.status","protocol":"$context.protocol","responseLength":"$context.responseLength","latency":"$context.responseLatency","integrationLatency":"$context.integrationLatency","integrationError":"$context.integrationErrorMessage","authorizerError":"$context.authorizer.error"}`

type Gateway struct {
	constructs.Construct
	RestAPI apigw2.HttpApi

	// Custom domain name, nil if not configured
	Domain apigw2.DomainName

	// DNS record of the custom domain name, nil if not configured
	Record awsroute53.ARecord

	methods       *[]apigw2.HttpMethod
	ids           map[string]int
	integrations  map[string]integrations.HttpLambdaIntegration
	routeSettings map[string]any
}

// NewGateway creates new instance of Gateway
func NewGateway(scope constructs.Construct, id *string, props *GatewayProps) *Gateway {
	gw := &Gateway{
		Construct:     constructs.NewConstruct(scope, id),
		ids:           map[string]int{},
		integrations:  map[string]integrations.HttpLambdaIntegration{},
		routeSettings: map[string]any{},
	}
	if props.HttpApiProps == nil {
		props.HttpApiProps = &apigw2.HttpApiProps{}
	}

	if props.HttpApiProps.ApiName == nil {
		props.ApiName = awscdk.Aws_STACK_NAME()
	}

	if props.HttpApiProps.CorsPreflight == nil {
		props.CorsPreflight = corsPreflight(props)
	}

	if props.HttpApiProps.CorsPreflight != nil {
		// OPTIONS is handled by API Gateway (preflight)
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
		gw.Domain = newDomainName(gw.Construct, props.Host, props.TlsArn)

		props.HttpApiProps.DefaultDomainMapping = &apigw2.DomainMappingOptions{
			DomainName: gw.Domain,
		}
	}

	gw.RestAPI = apigw2.NewHttpApi(gw.Construct, jsii.String("Gateway"), props.HttpApiProps)

	if props.Host != nil && props.TlsArn != nil {
		gw.Record = newDomainRecord(gw.Construct, props.Host, props.HostedZone, props.Zone, gw.Domain)
	}

	if props.AccessLog != nil {
		gw.configAccessLog(props.AccessLog)
	}

	if props.Throttle != nil {
		gw.stage().SetDefaultRouteSettings(&apigw2.CfnStage_RouteSettingsProperty{
			ThrottlingRateLimit:  jsii.Number(props.Throttle.RateLimit),
			ThrottlingBurstLimit: jsii.Number(props.Throttle.BurstLimit),
		})
	}

	if props.ApiIdParameter != nil {
		awsssm.NewStringParameter(gw.Construct, jsii.String("ApiId"),
			&awsssm.StringParameterProps{
				ParameterName: props.ApiIdParameter,
				StringValue:   gw.RestAPI.ApiId(),
			},
		)
	}

	return gw
}

func corsPreflight(props *GatewayProps) *apigw2.CorsPreflightOptions {
	cors := props.Cors
	if cors == nil && props.AllowOrigins != nil {
		cors = &Cors{AllowOrigins: []string{*props.AllowOrigins}}
	}

	if cors == nil {
		return nil
	}

	headers := cors.AllowHeaders
	if len(headers) == 0 {
		headers = []string{"Authorization", "Content-Type"}
	}

	maxAge := cors.MaxAge
	if maxAge == nil {
		maxAge = awscdk.Duration_Minutes(jsii.Number(10))
	}

	spec := &apigw2.CorsPreflightOptions{
		AllowOrigins: jsii.Strings(cors.AllowOrigins...),
		AllowMethods: &[]apigw2.CorsHttpMethod{apigw2.CorsHttpMethod_ANY},
		AllowHeaders: jsii.Strings(headers...),
		MaxAge:       maxAge,
	}

	if len(cors.ExposeHeaders) > 0 {
		spec.ExposeHeaders = jsii.Strings(cors.ExposeHeaders...)
	}

	if cors.AllowCredentials {
		spec.AllowCredentials = jsii.Bool(true)
	}

	return spec
}

// ApiId of the HTTP API
func (gw *Gateway) ApiId() *string { return gw.RestAPI.ApiId() }

// L1 construct of the default stage, use it to configure stage settings
// not covered by the library.
func (gw *Gateway) CfnStage() apigw2.CfnStage { return gw.stage() }

func (gw *Gateway) stage() apigw2.CfnStage {
	stage := gw.RestAPI.DefaultStage()
	if stage == nil {
		panic(fmt.Errorf("gateway %s has no default stage", *gw.Node().Path()))
	}

	return stage.Node().DefaultChild().(apigw2.CfnStage)
}

func (gw *Gateway) configAccessLog(spec *AccessLog) {
	logs := spec.LogGroup
	if logs == nil {
		defaults := defaultsOf(gw.Construct)

		retention := defaults.LogRetention
		if retention == "" {
			retention = awslogs.RetentionDays_FIVE_DAYS
		}

		removal := defaults.LogRemovalPolicy
		if removal == "" {
			removal = awscdk.RemovalPolicy_DESTROY
		}

		logs = awslogs.NewLogGroup(gw.Construct, jsii.String("AccessLog"),
			&awslogs.LogGroupProps{
				Retention:     retention,
				RemovalPolicy: removal,
			},
		)
	}

	format := spec.Format
	if format == nil {
		format = jsii.String(DefaultAccessLogFormat)
	}

	gw.stage().SetAccessLogSettings(&apigw2.CfnStage_AccessLogSettingsProperty{
		DestinationArn: logs.LogGroupArn(),
		Format:         format,
	})
}

// uniqueId returns base id for the first use of base, suffixed by sequence
// number for subsequent ones. It keeps construct ids of the first instance
// backward compatible.
func (gw *Gateway) uniqueId(base string) string {
	gw.ids[base]++
	if n := gw.ids[base]; n > 1 {
		return fmt.Sprintf("%s%d", base, n)
	}
	return base
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
		gw:      gw,
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

	// the first instance keeps legacy construct ids and function name
	fid := gw.uniqueId("AuthorizerBasic")
	scope := gw.Construct
	fprops := &awslambda.FunctionProps{
		Timeout: awscdk.Duration_Seconds(jsii.Number(5)),
		Environment: &map[string]*string{
			"CONFIG_AUTHORIZER_ACCESS": jsii.String(access),
			"CONFIG_AUTHORIZER_SECRET": jsii.String(secret),
			"CONFIG_AUTHORIZER_SOURCE": jsii.String(src),
		},
	}

	spec := &FunctionGoProps{
		SourceCodeModule: "github.com/fogfish/scud",
		SourceCodeLambda: "internal/cmd/auth",
		FunctionProps:    fprops,
	}

	if fid != "AuthorizerBasic" {
		scope = constructs.NewConstruct(gw.Construct, jsii.String(fid))
		fprops.FunctionName = jsii.Sprintf("%s-%s%d", *awscdk.Aws_STACK_NAME(), spec.UniqueID(), gw.ids["AuthorizerBasic"])
	}

	f := NewFunctionGo(scope, jsii.String(fid), spec)

	authorizer := authorizers.NewHttpLambdaAuthorizer(jsii.String(gw.uniqueId("LambdaAuthorizer")), f,
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
		gw:         gw,
	}
}

// Properties of Lambda authorizer
type AuthorizerLambdaProps struct {
	// Construct id of the authorizer, it is also the default name of
	// the authorizer. Default is derived from the function's construct id.
	Id string

	// Name of the authorizer, default is Id.
	AuthorizerName *string

	// Identity sources (e.g. "$request.header.Authorization").
	// Empty (default) means no identity source: API Gateway invokes
	// the authorizer for every request, including anonymous ones.
	// API Gateway requires disabled caching (zero ResultsCacheTtl)
	// for authorizers without identity source.
	IdentitySource []string

	// Response types of the authorizer, default is SIMPLE.
	// SIMPLE responses use authorizer payload format 2.0, IAM uses 1.0.
	ResponseTypes []authorizers.HttpLambdaResponseType

	// How long API Gateway caches authorizer results, default 0 (disabled).
	ResultsCacheTtl awscdk.Duration
}

// Creates integration with own Lambda (REQUEST) authorizer.
// Use package github.com/fogfish/scud/authorizer to implement the function.
func (gw *Gateway) NewAuthorizerLambda(handler awslambda.IFunction, props *AuthorizerLambdaProps) *AuthorizerLambda {
	if props == nil {
		props = &AuthorizerLambdaProps{}
	}

	ttl := props.ResultsCacheTtl
	if ttl == nil {
		ttl = awscdk.Duration_Seconds(jsii.Number(0))
	}

	if len(props.IdentitySource) == 0 && !*awscdk.Token_IsUnresolved(ttl) && *ttl.ToSeconds(nil) != 0 {
		panic(fmt.Errorf("lambda authorizer without identity source requires zero ResultsCacheTtl"))
	}

	responseTypes := props.ResponseTypes
	if len(responseTypes) == 0 {
		responseTypes = []authorizers.HttpLambdaResponseType{authorizers.HttpLambdaResponseType_SIMPLE}
	}

	id := props.Id
	if id == "" {
		id = *handler.Node().Id() + "Authorizer"
	}
	id = gw.uniqueId(id)

	authorizer := authorizers.NewHttpLambdaAuthorizer(jsii.String(id), handler,
		&authorizers.HttpLambdaAuthorizerProps{
			AuthorizerName:  props.AuthorizerName,
			IdentitySource:  jsii.Strings(props.IdentitySource...),
			ResponseTypes:   &responseTypes,
			ResultsCacheTtl: ttl,
		},
	)

	return &AuthorizerLambda{
		RestAPI:    gw.RestAPI,
		authorizer: authorizer,
		methods:    gw.methods,
		gw:         gw,
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
		gw:         gw,
	}
}

// Creates integration with AWS Cognito to authorize incoming requests.
// This integration allows you to manage user authentication and authorization
// seamlessly. By utilizing AWS Cognito, you can implement robust user
// sign-up, sign-in, and access control mechanisms. The library ensures that
// only authenticated users with valid tokens can access your API endpoints,
// providing an additional layer of security and user management capabilities.
func (gw *Gateway) NewAuthorizerCognito(cognitoArn string, clients ...string) *AuthorizerJwt {
	cid := gw.uniqueId("Cognito")
	pool := awscognito.UserPool_FromUserPoolArn(
		gw.Construct,
		jsii.String(cid),
		jsii.String(cognitoArn),
	)

	prefix := ""
	if cid != "Cognito" {
		prefix = cid
	}

	var allowList *[]awscognito.IUserPoolClient
	if len(clients) > 0 {
		al := make([]awscognito.IUserPoolClient, len(clients))
		for i, id := range clients {
			al[i] = awscognito.UserPoolClient_FromUserPoolClientId(
				gw.Construct,
				jsii.String(fmt.Sprintf("%sClient%d", prefix, i)),
				jsii.String(id),
			)
		}
		allowList = &al
	}

	authorizer := authorizers.NewHttpUserPoolAuthorizer(
		jsii.String(gw.uniqueId("Authorizer")),
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
		gw:         gw,
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
		jsii.String(gw.uniqueId("Authorizer")),
		jsii.String(iss),
		&authorizers.HttpJwtAuthorizerProps{
			JwtAudience: jsii.Strings(aud...),
		},
	)

	return &AuthorizerJwt{
		RestAPI:    gw.RestAPI,
		authorizer: authorizer,
		methods:    gw.methods,
		gw:         gw,
	}
}

//------------------------------------------------------------------------------

// legacy prefix routes: endpoint and endpoint/{any+}
func prefixRoutes(endpoint string) []string {
	if endpoint == "" || endpoint == "/" {
		return []string{"/", "/{any+}"}
	}
	endpoint = strings.TrimSuffix(endpoint, "/")
	return []string{endpoint, endpoint + "/{any+}"}
}

func prefixIntegrationId(endpoint string) string {
	id := filepath.Base(endpoint)
	if id == "/" || id == "." {
		return "Root"
	}
	return id
}

//------------------------------------------------------------------------------

type AuthorizerPublic struct {
	RestAPI apigw2.HttpApi
	methods *[]apigw2.HttpMethod
	gw      *Gateway
}

// Associate a Lambda function with a REST API path. It uses the specified
// path as a prefix, enabling the association of the Lambda function with
// all subpaths under that prefix.
func (api *AuthorizerPublic) AddResource(
	endpoint string,
	handler awslambda.Function,
) *AuthorizerPublic {
	lambda := integrations.NewHttpLambdaIntegration(
		jsii.String(prefixIntegrationId(endpoint)),
		handler,
		&integrations.HttpLambdaIntegrationProps{
			PayloadFormatVersion: apigw2.PayloadFormatVersion_VERSION_1_0(),
		},
	)

	for _, path := range prefixRoutes(endpoint) {
		api.RestAPI.AddRoutes(&apigw2.AddRoutesOptions{
			Path:        jsii.String(path),
			Integration: lambda,
			Methods:     api.methods,
		})
	}

	return api
}

// Associate a Lambda function with the exact route path (see RouteProps).
// The route is public even if the HTTP API has default authorizer.
func (api *AuthorizerPublic) AddRoute(path string, props *RouteProps) *AuthorizerPublic {
	api.gw.addRoute(path, props, apigw2.NewHttpNoneAuthorizer(), routeAuthNone)
	return api
}

//------------------------------------------------------------------------------

type AuthorizerBasic struct {
	RestAPI    apigw2.HttpApi
	authorizer authorizers.HttpLambdaAuthorizer
	methods    *[]apigw2.HttpMethod
	gw         *Gateway
}

// Associate a Lambda function with a REST API path. It uses the specified
// path as a prefix, enabling the association of the Lambda function with
// all subpaths under that prefix.
func (api *AuthorizerBasic) AddResource(
	endpoint string,
	handler awslambda.Function,
) *AuthorizerBasic {
	lambda := integrations.NewHttpLambdaIntegration(
		jsii.String(prefixIntegrationId(endpoint)),
		handler,
		&integrations.HttpLambdaIntegrationProps{
			PayloadFormatVersion: apigw2.PayloadFormatVersion_VERSION_1_0(),
		},
	)

	for _, path := range prefixRoutes(endpoint) {
		api.RestAPI.AddRoutes(&apigw2.AddRoutesOptions{
			Path:        jsii.String(path),
			Integration: lambda,
			Authorizer:  api.authorizer,
			Methods:     api.methods,
		})
	}

	return api
}

// Associate a Lambda function with the exact route path (see RouteProps).
func (api *AuthorizerBasic) AddRoute(path string, props *RouteProps) *AuthorizerBasic {
	api.gw.addRoute(path, props, api.authorizer, routeAuthLambda)
	return api
}

//------------------------------------------------------------------------------

type AuthorizerLambda struct {
	RestAPI    apigw2.HttpApi
	authorizer authorizers.HttpLambdaAuthorizer
	methods    *[]apigw2.HttpMethod
	gw         *Gateway
}

// Associate a Lambda function with a REST API path. It uses the specified
// path as a prefix, enabling the association of the Lambda function with
// all subpaths under that prefix.
func (api *AuthorizerLambda) AddResource(
	endpoint string,
	handler awslambda.Function,
) *AuthorizerLambda {
	for _, path := range prefixRoutes(endpoint) {
		api.gw.addRoute(path, &RouteProps{Handler: handler}, api.authorizer, routeAuthLambda)
	}

	return api
}

// Associate a Lambda function with the exact route path (see RouteProps).
func (api *AuthorizerLambda) AddRoute(path string, props *RouteProps) *AuthorizerLambda {
	api.gw.addRoute(path, props, api.authorizer, routeAuthLambda)
	return api
}

//------------------------------------------------------------------------------

type AuthorizerIAM struct {
	constructs.Construct
	RestAPI    apigw2.HttpApi
	authorizer apigw2.IHttpRouteAuthorizer
	methods    *[]apigw2.HttpMethod
	gw         *Gateway
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
		jsii.String(prefixIntegrationId(endpoint)),
		handler,
		&integrations.HttpLambdaIntegrationProps{
			PayloadFormatVersion: apigw2.PayloadFormatVersion_VERSION_1_0(),
		},
	)

	for _, path := range prefixRoutes(endpoint) {
		routes := api.RestAPI.AddRoutes(&apigw2.AddRoutesOptions{
			Path:        jsii.String(path),
			Integration: lambda,
			Authorizer:  api.authorizer,
			Methods:     api.methods,
		})
		for _, route := range *routes {
			route.GrantInvoke(grantee, nil)
		}
	}

	return api
}

// Associate a Lambda function with the exact route path (see RouteProps).
// RouteProps.Grantee is granted to invoke the route.
func (api *AuthorizerIAM) AddRoute(path string, props *RouteProps) *AuthorizerIAM {
	api.gw.addRoute(path, props, api.authorizer, routeAuthIAM)
	return api
}

//------------------------------------------------------------------------------

type AuthorizerJwt struct {
	constructs.Construct
	RestAPI    apigw2.HttpApi
	authorizer apigw2.IHttpRouteAuthorizer
	methods    *[]apigw2.HttpMethod
	gw         *Gateway
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
		jsii.String(prefixIntegrationId(endpoint)),
		handler,
		&integrations.HttpLambdaIntegrationProps{
			PayloadFormatVersion: apigw2.PayloadFormatVersion_VERSION_1_0(),
		},
	)

	for _, path := range prefixRoutes(endpoint) {
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

// Associate a Lambda function with the exact route path (see RouteProps).
// RouteProps.AuthorizationScopes are required from the token.
func (api *AuthorizerJwt) AddRoute(path string, props *RouteProps) *AuthorizerJwt {
	api.gw.addRoute(path, props, api.authorizer, routeAuthJwt)
	return api
}
