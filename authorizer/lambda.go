//
// Copyright (C) 2020 - 2025 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/scud
//

package authorizer

import (
	"encoding/base64"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

// Request is the event received by Lambda authorizer of HTTP API configured
// with SIMPLE response type (authorizer payload format 2.0), which is
// the default of scud.Gateway.NewAuthorizerLambda.
//
//	lambda.Start(func(req authorizer.Request) (authorizer.Response, error) {
//	  key, ok := authorizer.Bearer(req)
//	  if !ok {
//	    return authorizer.Deny(), nil
//	  }
//	  ...
//	  return authorizer.Allow(map[string]any{"sub": "..."}), nil
//	})
//
// The context of the response is delivered to the integration at
// requestContext.authorizer (integration payload format 1.0) or at
// requestContext.authorizer.lambda (integration payload format 2.0).
type Request = events.APIGatewayV2CustomAuthorizerV2Request

// Response is the simple response of Lambda authorizer.
type Response = events.APIGatewayV2CustomAuthorizerSimpleResponse

// Allow the request, the context is passed to the integration.
func Allow(context map[string]any) Response {
	return Response{IsAuthorized: true, Context: context}
}

// Deny the request, API Gateway answers 403 Forbidden to the client.
//
// Return Deny with nil error to refuse the request. A returned error
// is answered 500 Internal Server Error by API Gateway.
func Deny() Response {
	return Response{IsAuthorized: false}
}

// Header returns value of the request header, the name is case-insensitive.
func Header(req Request, name string) (string, bool) {
	if val, has := req.Headers[strings.ToLower(name)]; has {
		return val, true
	}

	for key, val := range req.Headers {
		if strings.EqualFold(key, name) {
			return val, true
		}
	}

	return "", false
}

// Query returns value of the query string parameter.
func Query(req Request, name string) (string, bool) {
	val, has := req.QueryStringParameters[name]
	return val, has
}

// Authorization returns the scheme and credentials of Authorization header.
// The scheme is case-insensitive, it is returned as given by the client.
func Authorization(req Request) (scheme, credentials string, ok bool) {
	val, has := Header(req, "Authorization")
	if !has {
		return "", "", false
	}

	scheme, credentials, ok = strings.Cut(strings.TrimSpace(val), " ")
	if !ok {
		return "", "", false
	}

	credentials = strings.TrimSpace(credentials)
	if credentials == "" {
		return "", "", false
	}

	return scheme, credentials, true
}

// Bearer returns token of Authorization: Bearer <token> header.
func Bearer(req Request) (string, bool) {
	scheme, token, ok := Authorization(req)
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	return token, true
}

// BasicCredentials returns user and password of Authorization: Basic <base64> header.
// Both padded (RFC 7617) and unpadded base64 are accepted.
func BasicCredentials(req Request) (user, password string, ok bool) {
	scheme, credentials, ok := Authorization(req)
	if !ok || !strings.EqualFold(scheme, "Basic") {
		return "", "", false
	}

	c, err := base64.StdEncoding.DecodeString(credentials)
	if err != nil {
		c, err = base64.RawStdEncoding.DecodeString(credentials)
		if err != nil {
			return "", "", false
		}
	}

	return strings.Cut(string(c), ":")
}
