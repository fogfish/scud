//
// Copyright (C) 2021 - 2025 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the Apache License Version 2.0. See the LICENSE file for details.
// https://github.com/fogfish/swarm
//

package main

import (
	"log/slog"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/fogfish/scud/authorizer"
)

var (
	None = events.APIGatewayCustomAuthorizerResponse{}
)

func main() {
	access := os.Getenv("CONFIG_AUTHORIZER_ACCESS")
	secret := os.Getenv("CONFIG_AUTHORIZER_SECRET")
	source := os.Getenv("CONFIG_AUTHORIZER_SOURCE")
	basic, err := authorizer.NewBasic(access, secret)
	if err != nil {
		slog.Warn("Basic Auth disabled.")
		basic = nil
	}

	lambda.Start(
		func(evt events.APIGatewayV2CustomAuthorizerV1Request) (events.APIGatewayCustomAuthorizerResponse, error) {
			var apikey string

			switch source {
			case "$request.header.Authorization":
				apikey = evt.Headers["authorization"]
				if !strings.HasPrefix(apikey, "Basic ") {
					return None, authorizer.ErrForbidden
				}
				apikey = strings.TrimPrefix(apikey, "Basic ")
			case "$request.querystring.apikey":
				apikey = evt.QueryStringParameters["apikey"]
			default:
				slog.Error("unsupported identity source.")
				return None, authorizer.ErrForbidden
			}

			if basic != nil {
				principal, context, err := basic.Validate(apikey)
				if err != nil {
					return None, authorizer.ErrForbidden
				}

				return AccessPolicy(principal, evt.MethodArn, context), nil
			}

			return None, authorizer.ErrForbidden
		},
	)
}

//------------------------------------------------------------------------------

// Grant the access to WebSocket with the policy
func AccessPolicy(principal, method string, context map[string]any) events.APIGatewayCustomAuthorizerResponse {
	return events.APIGatewayCustomAuthorizerResponse{
		PrincipalID: principal,
		PolicyDocument: events.APIGatewayCustomAuthorizerPolicy{
			Version: "2012-10-17",
			Statement: []events.IAMPolicyStatement{
				{
					Action:   []string{"execute-api:*"},
					Effect:   "Allow",
					Resource: []string{method},
				},
			},
		},
		Context: context,
	}
}
