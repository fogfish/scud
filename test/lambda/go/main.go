package main

import (
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

var version = "0.0.0"

func main() {
	lambda.Start(
		func(r events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
			return events.APIGatewayV2HTTPResponse{
				StatusCode:      200,
				IsBase64Encoded: false,
				Headers: map[string]string{
					"Content-Type": "text/plain",
					"X-Version":    version,
				},
				Body: "Hello World!",
			}, nil
		},
	)
}
