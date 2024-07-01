package main

import (
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	lambda.Start(
		func(r events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
			return events.APIGatewayV2HTTPResponse{
				StatusCode:      200,
				IsBase64Encoded: false,
				Headers: map[string]string{
					"Content-Type": "text/plain",
				},
				Body: "Hello World!",
			}, nil
		},
	)
}
