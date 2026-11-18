//
// Copyright (C) 2020 - 2025 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/scud
//

package scud_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	apigw2 "github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2"
	authorizers "github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2authorizers"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
	"github.com/fogfish/scud"
)

func newStack(t *testing.T, context map[string]any) awscdk.Stack {
	t.Helper()

	var props *awscdk.AppProps
	if context != nil {
		props = &awscdk.AppProps{Context: &context}
	}

	app := awscdk.NewApp(props)
	return awscdk.NewStack(app, jsii.String("Test"),
		&awscdk.StackProps{
			Env: &awscdk.Environment{
				Account: jsii.String("000000000000"),
				Region:  jsii.String("eu-west-1"),
			},
		},
	)
}

func newFunction(scope constructs.Construct, id string) awslambda.Function {
	return scud.NewFunctionGo(scope, jsii.String(id),
		&scud.FunctionGoProps{
			SourceCodeModule: "github.com/fogfish/scud",
			SourceCodeLambda: "test/lambda/go",
			FunctionProps: &awslambda.FunctionProps{
				FunctionName: jsii.String(id),
				LogGroup:     awslogs.LogGroup_FromLogGroupName(scope, jsii.String("Logs"+id), jsii.String(id)),
			},
		},
	)
}

func routeKeys(template assertions.Template) []string {
	keys := []string{}
	for _, r := range *template.FindResources(jsii.String("AWS::ApiGatewayV2::Route"), nil) {
		spec := (*r)["Properties"].(map[string]any)
		keys = append(keys, spec["RouteKey"].(string))
	}
	return keys
}

func hasAll(t *testing.T, seq []string, expected ...string) {
	t.Helper()
	set := map[string]struct{}{}
	for _, x := range seq {
		set[x] = struct{}{}
	}
	for _, x := range expected {
		if _, has := set[x]; !has {
			t.Errorf("%q not found in %v", x, seq)
		}
	}
	if len(seq) != len(expected) {
		t.Errorf("expected %d items, got %d: %v", len(expected), len(seq), seq)
	}
}

func mustPanic(t *testing.T, contains string, f func()) {
	t.Helper()
	defer func() {
		t.Helper()
		err := recover()
		if err == nil {
			t.Errorf("expected panic %q", contains)
			return
		}
		if !strings.Contains(fmt.Sprint(err), contains) {
			t.Errorf("expected panic %q, got %v", contains, err)
		}
	}()
	f()
}

//------------------------------------------------------------------------------

func TestGatewayDefaultStageOnly(t *testing.T) {
	stack := newStack(t, nil)
	scud.NewGateway(stack, jsii.String("GW"), &scud.GatewayProps{})

	template := assertions.Template_FromStack(stack, nil)
	template.ResourceCountIs(jsii.String("AWS::ApiGatewayV2::Stage"), jsii.Number(1))
	template.HasResourceProperties(jsii.String("AWS::ApiGatewayV2::Stage"),
		map[string]any{"StageName": "$default", "AutoDeploy": true},
	)
}

func TestAddRouteExact(t *testing.T) {
	stack := newStack(t, nil)
	read := newFunction(stack, "read")
	write := newFunction(stack, "write")

	gw := scud.NewGateway(stack, jsii.String("GW"), &scud.GatewayProps{})
	gw.NewAuthorizerPublic().
		AddRoute("/", &scud.RouteProps{Handler: read, Methods: []apigw2.HttpMethod{apigw2.HttpMethod_GET}}).
		AddRoute("/{instance}", &scud.RouteProps{Handler: read, Methods: []apigw2.HttpMethod{apigw2.HttpMethod_GET}}).
		AddRoute("/{instance}/facts", &scud.RouteProps{Handler: read, Methods: []apigw2.HttpMethod{apigw2.HttpMethod_GET}}).
		AddRoute("/{instance}/facts", &scud.RouteProps{Handler: write, Methods: []apigw2.HttpMethod{apigw2.HttpMethod_PUT, apigw2.HttpMethod_DELETE}}).
		AddRoute("/admin/facts", &scud.RouteProps{Handler: write, Id: "AdminFacts"})

	template := assertions.Template_FromStack(stack, nil)

	hasAll(t, routeKeys(template),
		"GET /",
		"GET /{instance}",
		"GET /{instance}/facts",
		"PUT /{instance}/facts",
		"DELETE /{instance}/facts",
		"ANY /admin/facts",
	)

	// one integration per function
	template.ResourceCountIs(jsii.String("AWS::ApiGatewayV2::Integration"), jsii.Number(2))
	template.HasResourceProperties(jsii.String("AWS::ApiGatewayV2::Integration"),
		map[string]any{"PayloadFormatVersion": "1.0"},
	)
	template.HasResourceProperties(jsii.String("AWS::ApiGatewayV2::Route"),
		map[string]any{"RouteKey": "GET /", "AuthorizationType": "NONE"},
	)
}

func TestAddRouteCorsMethods(t *testing.T) {
	stack := newStack(t, nil)
	f := newFunction(stack, "f")

	gw := scud.NewGateway(stack, jsii.String("GW"),
		&scud.GatewayProps{
			Cors: &scud.Cors{
				AllowOrigins:  []string{"https://example.com"},
				AllowHeaders:  []string{"Authorization", "Content-Type", "Mcp-Session-Id"},
				ExposeHeaders: []string{"Mcp-Session-Id"},
			},
		},
	)
	gw.NewAuthorizerPublic().AddRoute("/mcp", &scud.RouteProps{Handler: f})

	template := assertions.Template_FromStack(stack, nil)
	hasAll(t, routeKeys(template),
		"GET /mcp", "HEAD /mcp", "PATCH /mcp", "POST /mcp", "PUT /mcp", "DELETE /mcp",
	)
	template.HasResourceProperties(jsii.String("AWS::ApiGatewayV2::Api"),
		map[string]any{
			"CorsConfiguration": map[string]any{
				"AllowOrigins":  []string{"https://example.com"},
				"AllowHeaders":  []string{"Authorization", "Content-Type", "Mcp-Session-Id"},
				"ExposeHeaders": []string{"Mcp-Session-Id"},
				"AllowMethods":  []string{"*"},
				"MaxAge":        600,
			},
		},
	)
}

func TestAddRoutePayloadFormat(t *testing.T) {
	stack := newStack(t, nil)
	f := newFunction(stack, "f")

	gw := scud.NewGateway(stack, jsii.String("GW"), &scud.GatewayProps{})
	gw.NewAuthorizerPublic().
		AddRoute("/v1", &scud.RouteProps{Handler: f}).
		AddRoute("/v2", &scud.RouteProps{Handler: f, PayloadFormatVersion: apigw2.PayloadFormatVersion_VERSION_2_0()}).
		AddRoute("/v2/{id}", &scud.RouteProps{Handler: f, PayloadFormatVersion: apigw2.PayloadFormatVersion_VERSION_2_0()})

	template := assertions.Template_FromStack(stack, nil)
	template.ResourceCountIs(jsii.String("AWS::ApiGatewayV2::Integration"), jsii.Number(2))
	template.HasResourceProperties(jsii.String("AWS::ApiGatewayV2::Integration"),
		map[string]any{"PayloadFormatVersion": "1.0"},
	)
	template.HasResourceProperties(jsii.String("AWS::ApiGatewayV2::Integration"),
		map[string]any{"PayloadFormatVersion": "2.0"},
	)
}

func TestAddRouteValidation(t *testing.T) {
	stack := newStack(t, nil)
	f := newFunction(stack, "f")
	gw := scud.NewGateway(stack, jsii.String("GW"), &scud.GatewayProps{})

	mustPanic(t, "handler is required", func() {
		gw.NewAuthorizerPublic().AddRoute("/a", &scud.RouteProps{})
	})

	mustPanic(t, "must start with /", func() {
		gw.NewAuthorizerPublic().AddRoute("a", &scud.RouteProps{Handler: f})
	})

	mustPanic(t, "authorization scopes", func() {
		gw.NewAuthorizerPublic().AddRoute("/a", &scud.RouteProps{Handler: f, AuthorizationScopes: []string{"x"}})
	})

	mustPanic(t, "grantee", func() {
		gw.NewAuthorizerPublic().AddRoute("/a", &scud.RouteProps{Handler: f, Grantee: awsiam.NewAccountRootPrincipal()})
	})
}

func TestAddResourceRoot(t *testing.T) {
	stack := newStack(t, nil)
	f := newFunction(stack, "f")

	gw := scud.NewGateway(stack, jsii.String("GW"), &scud.GatewayProps{})
	gw.NewAuthorizerPublic().
		AddResource("/", f).
		AddResource("/a/x", f).
		AddResource("/b/x", f)

	template := assertions.Template_FromStack(stack, nil)
	hasAll(t, routeKeys(template),
		"ANY /", "ANY /{any+}",
		"ANY /a/x", "ANY /a/x/{any+}",
		"ANY /b/x", "ANY /b/x/{any+}",
	)
}

func TestAuthorizerLambdaNoIdentitySource(t *testing.T) {
	stack := newStack(t, nil)
	f := newFunction(stack, "f")
	auth := newFunction(stack, "auth")

	gw := scud.NewGateway(stack, jsii.String("GW"), &scud.GatewayProps{})
	gw.NewAuthorizerLambda(auth, nil).
		AddRoute("/{instance}", &scud.RouteProps{Handler: f, Methods: []apigw2.HttpMethod{apigw2.HttpMethod_GET}})

	template := assertions.Template_FromStack(stack, nil)
	template.ResourceCountIs(jsii.String("AWS::ApiGatewayV2::Authorizer"), jsii.Number(1))
	template.HasResourceProperties(jsii.String("AWS::ApiGatewayV2::Authorizer"),
		map[string]any{
			"AuthorizerType":                 "REQUEST",
			"IdentitySource":                 []string{},
			"AuthorizerResultTtlInSeconds":   0,
			"EnableSimpleResponses":          true,
			"AuthorizerPayloadFormatVersion": "2.0",
			"Name":                           "authAuthorizer",
		},
	)
	template.HasResourceProperties(jsii.String("AWS::ApiGatewayV2::Route"),
		map[string]any{"RouteKey": "GET /{instance}", "AuthorizationType": "CUSTOM"},
	)
}

func TestAuthorizerLambdaIdentitySource(t *testing.T) {
	stack := newStack(t, nil)
	f := newFunction(stack, "f")
	auth := newFunction(stack, "auth")

	gw := scud.NewGateway(stack, jsii.String("GW"), &scud.GatewayProps{})
	gw.NewAuthorizerLambda(auth,
		&scud.AuthorizerLambdaProps{
			Id:              "Keys",
			IdentitySource:  []string{"$request.header.Authorization"},
			ResponseTypes:   []authorizers.HttpLambdaResponseType{authorizers.HttpLambdaResponseType_IAM},
			ResultsCacheTtl: awscdk.Duration_Seconds(jsii.Number(60)),
		},
	).AddResource("/keys", f)

	template := assertions.Template_FromStack(stack, nil)
	template.HasResourceProperties(jsii.String("AWS::ApiGatewayV2::Authorizer"),
		map[string]any{
			"Name":                           "Keys",
			"IdentitySource":                 []string{"$request.header.Authorization"},
			"AuthorizerResultTtlInSeconds":   60,
			"AuthorizerPayloadFormatVersion": "1.0",
		},
	)
	hasAll(t, routeKeys(template), "ANY /keys", "ANY /keys/{any+}")
}

func TestAuthorizerLambdaCacheValidation(t *testing.T) {
	stack := newStack(t, nil)
	auth := newFunction(stack, "auth")
	gw := scud.NewGateway(stack, jsii.String("GW"), &scud.GatewayProps{})

	mustPanic(t, "zero ResultsCacheTtl", func() {
		gw.NewAuthorizerLambda(auth,
			&scud.AuthorizerLambdaProps{
				ResultsCacheTtl: awscdk.Duration_Minutes(jsii.Number(5)),
			},
		)
	})
}

func TestAuthorizerManyOfKind(t *testing.T) {
	stack := newStack(t, nil)
	f := newFunction(stack, "f")
	auth := newFunction(stack, "auth")

	gw := scud.NewGateway(stack, jsii.String("GW"), &scud.GatewayProps{})
	gw.NewAuthorizerJwt("https://a.example.com/", "a").AddRoute("/a", &scud.RouteProps{Handler: f, AuthorizationScopes: []string{"read"}})
	gw.NewAuthorizerJwt("https://b.example.com/", "b").AddRoute("/b", &scud.RouteProps{Handler: f})
	gw.NewAuthorizerCognito("arn:aws:cognito-idp:eu-west-1:000000000000:userpool/eu-west-1_A", "x").AddResource("/c", f)
	gw.NewAuthorizerCognito("arn:aws:cognito-idp:eu-west-1:000000000000:userpool/eu-west-1_B", "y").AddResource("/d", f)
	gw.NewAuthorizerBasic("u1", "p1").AddResource("/e", f)
	gw.NewAuthorizerBasic("u2", "p2").AddRoute("/f", &scud.RouteProps{Handler: f})
	gw.NewAuthorizerLambda(auth, nil).AddRoute("/g", &scud.RouteProps{Handler: f})
	gw.NewAuthorizerLambda(auth, nil).AddRoute("/h", &scud.RouteProps{Handler: f})

	template := assertions.Template_FromStack(stack, nil)
	template.ResourceCountIs(jsii.String("AWS::ApiGatewayV2::Authorizer"), jsii.Number(8))
	template.HasResourceProperties(jsii.String("AWS::ApiGatewayV2::Route"),
		map[string]any{"RouteKey": "ANY /a", "AuthorizationType": "JWT", "AuthorizationScopes": []string{"read"}},
	)

	// legacy names of the first instances are kept
	for _, name := range []string{"Authorizer", "Authorizer2", "Authorizer3", "Authorizer4", "LambdaAuthorizer", "LambdaAuthorizer2", "authAuthorizer", "authAuthorizer2"} {
		template.HasResourceProperties(jsii.String("AWS::ApiGatewayV2::Authorizer"),
			map[string]any{"Name": name},
		)
	}
}

func TestAuthorizerIAMRoute(t *testing.T) {
	stack := newStack(t, nil)
	f := newFunction(stack, "f")
	role := awsiam.NewRole(stack, jsii.String("Role"),
		&awsiam.RoleProps{AssumedBy: awsiam.NewAccountRootPrincipal()},
	)

	gw := scud.NewGateway(stack, jsii.String("GW"), &scud.GatewayProps{})
	gw.NewAuthorizerIAM().AddRoute("/iam", &scud.RouteProps{
		Handler: f,
		Methods: []apigw2.HttpMethod{apigw2.HttpMethod_GET, apigw2.HttpMethod_POST},
		Grantee: role,
	})

	template := assertions.Template_FromStack(stack, nil)
	hasAll(t, routeKeys(template), "GET /iam", "POST /iam")
	template.HasResourceProperties(jsii.String("AWS::ApiGatewayV2::Route"),
		map[string]any{"RouteKey": "GET /iam", "AuthorizationType": "AWS_IAM"},
	)
	template.ResourceCountIs(jsii.String("AWS::IAM::Policy"), jsii.Number(1))
}

func TestStageThrottleAndAccessLog(t *testing.T) {
	stack := newStack(t, nil)
	f := newFunction(stack, "f")

	gw := scud.NewGateway(stack, jsii.String("GW"),
		&scud.GatewayProps{
			Throttle:  &scud.Throttle{RateLimit: 100, BurstLimit: 50},
			AccessLog: &scud.AccessLog{},
		},
	)
	gw.NewAuthorizerPublic().
		AddRoute("/{instance}/facts", &scud.RouteProps{
			Handler:  f,
			Methods:  []apigw2.HttpMethod{apigw2.HttpMethod_PUT},
			Throttle: &scud.Throttle{RateLimit: 0, BurstLimit: 0},
		}).
		AddRoute("/{instance}", &scud.RouteProps{
			Handler:  f,
			Methods:  []apigw2.HttpMethod{apigw2.HttpMethod_GET},
			Throttle: &scud.Throttle{RateLimit: 10, BurstLimit: 5},
		})

	template := assertions.Template_FromStack(stack, nil)
	template.ResourceCountIs(jsii.String("AWS::ApiGatewayV2::Stage"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::Logs::LogGroup"), jsii.Number(1))
	template.HasResourceProperties(jsii.String("AWS::ApiGatewayV2::Stage"),
		map[string]any{
			"StageName": "$default",
			"DefaultRouteSettings": map[string]any{
				"ThrottlingRateLimit":  100,
				"ThrottlingBurstLimit": 50,
			},
			"RouteSettings": map[string]any{
				"PUT /{instance}/facts": map[string]any{
					"ThrottlingRateLimit":  0,
					"ThrottlingBurstLimit": 0,
				},
				"GET /{instance}": map[string]any{
					"ThrottlingRateLimit":  10,
					"ThrottlingBurstLimit": 5,
				},
			},
			"AccessLogSettings": map[string]any{
				"Format": scud.DefaultAccessLogFormat,
			},
		},
	)

	stages := *template.FindResources(jsii.String("AWS::ApiGatewayV2::Stage"), nil)
	for _, stage := range stages {
		deps, _ := (*stage)["DependsOn"].([]any)
		if len(deps) < 2 {
			t.Errorf("stage must depend on throttled routes: %v", deps)
		}
	}
}

func TestDomainThreeLabels(t *testing.T) {
	stack := newStack(t, map[string]any{
		"hosted-zone:account=000000000000:domainName=api.example.com:region=eu-west-1": map[string]any{
			"Id":   "/hostedzone/ZAPI",
			"Name": "api.example.com.",
		},
	})

	gw := scud.NewGateway(stack, jsii.String("GW"),
		&scud.GatewayProps{
			Host:           jsii.String("v1.api.example.com"),
			TlsArn:         jsii.String("arn:aws:acm:eu-west-1:000000000000:certificate/00000000-0000-0000-0000-000000000000"),
			ApiIdParameter: jsii.String("/example/v1/api"),
		},
	)
	if gw.Domain == nil || gw.Record == nil || gw.ApiId() == nil {
		t.Errorf("domain, record and api id must be exposed")
	}

	template := assertions.Template_FromStack(stack, nil)
	template.HasResourceProperties(jsii.String("AWS::ApiGatewayV2::DomainName"),
		map[string]any{"DomainName": "v1.api.example.com"},
	)
	template.HasResourceProperties(jsii.String("AWS::Route53::RecordSet"),
		map[string]any{"Name": "v1.api.example.com.", "Type": "A", "HostedZoneId": "ZAPI"},
	)
	template.HasResourceProperties(jsii.String("AWS::SSM::Parameter"),
		map[string]any{"Name": "/example/v1/api"},
	)
}

func TestAlias(t *testing.T) {
	stack := newStack(t, map[string]any{
		"hosted-zone:account=000000000000:domainName=api.example.com:region=eu-west-1": map[string]any{
			"Id":   "/hostedzone/ZAPI",
			"Name": "api.example.com.",
		},
	})

	scud.NewAlias(stack, jsii.String("Alias"),
		&scud.AliasProps{
			Host:   jsii.String("api.example.com"),
			TlsArn: jsii.String("arn:aws:acm:eu-west-1:000000000000:certificate/00000000-0000-0000-0000-000000000000"),
			ApiId:  jsii.String("abc123"),
		},
	)

	template := assertions.Template_FromStack(stack, nil)
	template.HasResourceProperties(jsii.String("AWS::ApiGatewayV2::DomainName"),
		map[string]any{"DomainName": "api.example.com"},
	)
	template.HasResourceProperties(jsii.String("AWS::ApiGatewayV2::ApiMapping"),
		map[string]any{"ApiId": "abc123", "Stage": "$default"},
	)
	template.HasResourceProperties(jsii.String("AWS::Route53::RecordSet"),
		map[string]any{"Name": "api.example.com.", "Type": "A", "HostedZoneId": "ZAPI"},
	)
}

func TestAliasFromParameter(t *testing.T) {
	stack := newStack(t, map[string]any{
		"hosted-zone:account=000000000000:domainName=example.com:region=eu-west-1": map[string]any{
			"Id":   "/hostedzone/ZEX",
			"Name": "example.com.",
		},
	})

	scud.NewAlias(stack, jsii.String("Alias"),
		&scud.AliasProps{
			Host:           jsii.String("api.example.com"),
			Zone:           jsii.String("example.com"),
			TlsArn:         jsii.String("arn:aws:acm:eu-west-1:000000000000:certificate/00000000-0000-0000-0000-000000000000"),
			ApiIdParameter: jsii.String("/example/v1/api"),
		},
	)

	template := assertions.Template_FromStack(stack, nil)
	template.ResourceCountIs(jsii.String("AWS::ApiGatewayV2::ApiMapping"), jsii.Number(1))
	template.HasResourceProperties(jsii.String("AWS::Route53::RecordSet"),
		map[string]any{"Name": "api.example.com.", "HostedZoneId": "ZEX"},
	)
}
