//
// Copyright (C) 2020 - 2025 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/scud
//

package scud

import (
	"fmt"
	"strings"

	apigw2 "github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2"
	integrations "github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2integrations"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/jsii-runtime-go"
)

// RouteProps defines the exact route of HTTP API.
//
// Unlike AddResource, the route matches the path exactly. The path may
// contain path parameters (e.g. /{instance}/facts), greedy path parameter
// (e.g. /files/{proxy+}) or be the root "/".
type RouteProps struct {
	// Function handling requests of the route (required)
	Handler awslambda.IFunction

	// HTTP methods of the route. The default is every method except OPTIONS
	// if CORS is enabled at the gateway (preflight is answered by
	// API Gateway), otherwise ANY.
	Methods []apigw2.HttpMethod

	// Construct id of the route. If the route has multiple methods, the id is
	// suffixed with the method. The default is derived from the method and
	// the whole path (e.g. GET/{instance}/facts), as HttpApi.AddRoutes does.
	Id string

	// Payload format of the Lambda integration, default is 1.0.
	//
	//   - 1.0: the function receives events.APIGatewayProxyRequest, the context
	//     of Lambda authorizer is at RequestContext.Authorizer
	//   - 2.0: the function receives events.APIGatewayV2HTTPRequest, the context
	//     of Lambda authorizer is at RequestContext.Authorizer.Lambda
	//
	// One integration per function and payload format is shared by all routes.
	PayloadFormatVersion apigw2.PayloadFormatVersion

	// Throttling of the route, overrides the gateway's throttling.
	Throttle *Throttle

	// OAuth scopes required by the route (JWT and Cognito authorizers only).
	AuthorizationScopes []string

	// Principal granted to invoke the route (IAM authorizer only).
	Grantee awsiam.IGrantable
}

type routeAuthKind int

const (
	routeAuthNone routeAuthKind = iota
	routeAuthLambda
	routeAuthIAM
	routeAuthJwt
)

func (gw *Gateway) addRoute(
	path string,
	props *RouteProps,
	authorizer apigw2.IHttpRouteAuthorizer,
	kind routeAuthKind,
) []apigw2.HttpRoute {
	if props == nil || props.Handler == nil {
		panic(fmt.Errorf("route %s: handler is required", path))
	}

	if !strings.HasPrefix(path, "/") {
		panic(fmt.Errorf("route %s: path must start with /", path))
	}

	if len(props.AuthorizationScopes) > 0 && kind != routeAuthJwt {
		panic(fmt.Errorf("route %s: authorization scopes are supported by JWT and Cognito authorizers only", path))
	}

	if props.Grantee != nil && kind != routeAuthIAM {
		panic(fmt.Errorf("route %s: grantee is supported by IAM authorizer only", path))
	}

	methods := props.Methods
	if len(methods) == 0 {
		if gw.methods != nil {
			methods = *gw.methods
		} else {
			methods = []apigw2.HttpMethod{apigw2.HttpMethod_ANY}
		}
	}

	integration := gw.integration(props.Handler, props.PayloadFormatVersion)

	var scopes *[]*string
	if len(props.AuthorizationScopes) > 0 {
		scopes = jsii.Strings(props.AuthorizationScopes...)
	}

	routes := make([]apigw2.HttpRoute, 0, len(methods))
	for _, method := range methods {
		id := string(method) + path
		if props.Id != "" {
			id = props.Id
			if len(methods) > 1 {
				id = props.Id + string(method)
			}
		}

		route := apigw2.NewHttpRoute(gw.RestAPI, jsii.String(id),
			&apigw2.HttpRouteProps{
				HttpApi:             gw.RestAPI,
				RouteKey:            apigw2.HttpRouteKey_With(jsii.String(path), method),
				Integration:         integration,
				Authorizer:          authorizer,
				AuthorizationScopes: scopes,
			},
		)

		if props.Grantee != nil {
			route.GrantInvoke(props.Grantee, nil)
		}

		if props.Throttle != nil {
			gw.throttleRoute(route, string(method)+" "+path, props.Throttle)
		}

		routes = append(routes, route)
	}

	return routes
}

// one integration per function and payload format version
func (gw *Gateway) integration(
	handler awslambda.IFunction,
	version apigw2.PayloadFormatVersion,
) integrations.HttpLambdaIntegration {
	if version == nil {
		version = apigw2.PayloadFormatVersion_VERSION_1_0()
	}

	key := *handler.Node().Addr() + "@" + *version.Version()
	if lambda, has := gw.integrations[key]; has {
		return lambda
	}

	id := *handler.Node().Id()
	if *version.Version() != "1.0" {
		id = id + "V" + strings.ReplaceAll(*version.Version(), ".", "")
	}

	lambda := integrations.NewHttpLambdaIntegration(
		jsii.String(id),
		handler,
		&integrations.HttpLambdaIntegrationProps{
			PayloadFormatVersion: version,
		},
	)
	gw.integrations[key] = lambda

	return lambda
}

func (gw *Gateway) throttleRoute(route apigw2.HttpRoute, routeKey string, throttle *Throttle) {
	gw.routeSettings[routeKey] = map[string]any{
		"ThrottlingRateLimit":  throttle.RateLimit,
		"ThrottlingBurstLimit": throttle.BurstLimit,
	}

	stage := gw.stage()
	stage.SetRouteSettings(gw.routeSettings)

	// stage settings refer the route by its key, it must exists
	stage.Node().AddDependency(route)
}
