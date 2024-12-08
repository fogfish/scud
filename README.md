<p align="center">
  <img src="./doc/scud-logo.png" height="240" />
  <h3 align="center">scud</h3>
  <p align="center"><strong>simplified serverless api gateway (AWS CDK L3)</strong></p>

  <p align="center">
    <!-- Version -->
    <a href="https://github.com/fogfish/scud/releases">
      <img src="https://img.shields.io/github/v/tag/fogfish/scud?label=version" />
    </a>
    <!-- Documentation -->
    <a href="https://pkg.go.dev/github.com/fogfish/scud">
      <img src="https://pkg.go.dev/badge/github.com/fogfish/scud" />
    </a>
    <!-- Build Status  -->
    <a href="https://github.com/fogfish/scud/actions/">
      <img src="https://github.com/fogfish/scud/workflows/build/badge.svg" />
    </a>
    <!-- GitHub -->
    <a href="http://github.com/fogfish/scud">
      <img src="https://img.shields.io/github/last-commit/fogfish/scud.svg" />
    </a>
    <!-- Coverage -->
    <a href="https://coveralls.io/github/fogfish/scud?branch=main">
      <img src="https://coveralls.io/repos/github/fogfish/scud/badge.svg?branch=main" />
    </a>
    <!-- Go Card -->
    <a href="https://goreportcard.com/report/github.com/fogfish/scud">
      <img src="https://goreportcard.com/badge/github.com/fogfish/scud" />
    </a>
  </p>
</p>

--- 

`scud` is a Simple Cloud Usable Daemon (API Gateway) designed for serverless RESTful API development. This library is an AWS CDK L3 pattern that handles the infrastructure boilerplate, allowing you to focus on developing application logic.


## Inspiration

AWS API Gateway and AWS Lambda is a perfect approach for quick prototyping or production development of microservice on Amazon Web Services. Unfortunately, it requires a boilerplate AWS CDK code to bootstrap the development. This library implements a high-order components on top of AWS CDK that hardens the api pattern

![RESTful API Pattern](./doc/scud.excalidraw.svg "RESTful API Pattern")

The library aids in building Lambda functions by:
* Integrating the "compilation" of Golang serverless functions ("assets") within CDK workflows;
* Providing validation of OAuth2 Bearer tokens for each API endpoint, using various identity provides (IAM, JWT tokens and AWS Cognito).

- [Inspiration](#inspiration)
- [Getting started](#getting-started)
  - [Quick Start](#quick-start)
- [User Guide](#user-guide)
  - [Serverless functions](#serverless-functions)
  - [Serverless functions (arch amd64)](#serverless-functions-arch-amd64)
  - [Serverless function (Docker container)](#serverless-function-docker-container)
  - [API Gateway](#api-gateway)
  - [API Gateway (Domain Name)](#api-gateway-domain-name)
  - [API Gateway (Resources)](#api-gateway-resources)
  - [Authorizer IAM](#authorizer-iam)
  - [Authorizer AWS Cognito](#authorizer-aws-cognito)
  - [Authorizer JWT](#authorizer-jwt)
- [HowTo Contribute](#howto-contribute)
- [License](#license)
- [References](#references)

## Getting started

The latest version of the library is available at its `main` branch. All development, including new features and bug fixes, take place on the `main` branch using forking and pull requests as described in contribution guidelines. The stable version is available via Golang modules.

Use `go get` to retrieve the library and add it as dependency to your application.

```bash
go get -u github.com/fogfish/scud
```

### Quick Start

```go
package main

import (
  "github.com/aws/aws-cdk-go/awscdk/v2"
  "github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2"
  "github.com/aws/jsii-runtime-go"
  "github.com/fogfish/scud"
)

func main() {
  app := awscdk.NewApp(nil)
  stack := awscdk.NewStack(app, jsii.String("example-api"), nil)

  // API Gateway
  api := scud.NewGateway(stack, jsii.String("Gateway"), &scud.GatewayProps{})

  // Handler Function
  fun := scud.NewFunctionGo(stack, jsii.String("Handler"),
    &scud.FunctionGoProps{
      SourceCodeModule: "github.com/fogfish/scud",
      SourceCodeLambda:  "test/lambda/go",
    },
  )

  // Example endpoint
  api.AddResource("/example", fun)

  app.Synth(nil)
}
```

See advanced example as [the service blueprint](https://github.com/fogfish/blueprint-serverless-golang) 


## User Guide

### Serverless functions

AWS CDK supports a bundling feature that streamlines the process of creating assets for Lambda functions from source code. This feature is particularly useful for compiling and packaging your code in languages such as Golang, which need to be converted from source files into executable binaries. The library simplify bundling thought built in presets for assembling ARM64 lambdas for Amazon Linux 2.

```go
scud.NewFunctionGo(stack, jsii.String("Handler"),
  &scud.FunctionGoProps{
    // Golang module that containing the function
    SourceCodeModule: "github.com/fogfish/scud",
    // Path to lambda function within the module 
    SourceCodeLambda:  "test/lambda/go",
    // Lambda properties
    FunctionProps: &awslambda.FunctionProps{},
  },
)
```

### Serverless functions (arch amd64)

ARM64 is default architecture for lambda function. Use `GoEnv` property to build lambda for other architecture.

```go
scud.NewFunctionGo(scope, jsii.String("test"),
  &scud.FunctionGoProps{
    SourceCodeModule: "github.com/fogfish/scud",
    SourceCodeLambda:  "test/lambda/go",
    GoEnv: map[string]string{"GOARCH": "amd64"},
  },
)
```

### Serverless function (Docker container)

Zip files is default distribution method for lambda function. The library support building it from containers.

```go
scud.NewContainerGo(stack, jsii.String("test"),
  &scud.ContainerGoProps{
    SourceCodeModule: "github.com/fogfish/scud",
    SourceCodeLambda: "test/lambda/go",
    StaticAssets: []string{
      // list of files to be include into container
      // path is relative to SourceCodeModule
      // For example 
      "test/lambda/go/main.go"
    },
  },
)
```


### API Gateway 

The library defined presets for AWS API Gateway V2. 

```go
scud.NewGateway(stack, jsii.String("Gateway"),
  &scud.GatewayProps{}
)
```

### API Gateway (Domain Name)

The library deploy gateway using default AWS host naming convention `https://{uid}.execute-api.{region}.amazonaws.com`. Supply custom domain name and Certificate ARN for custom naming.

```go
scud.NewGateway(stack, jsii.String("Gateway"),
  &scud.GatewayProps{
    Host: jsii.String("test.example.com"),
    TlsArn: jsii.String("arn:aws:acm:eu-west-1:000000000000:certificate/00000000-0000-0000-0000-000000000000"),
  },
)
```

### API Gateway (Resources)

The Gateway construct implements the `AddResource` function to associate a Lambda function with a REST API path. It uses the specified path as a prefix, enabling the association of the Lambda function with all subpaths under that prefix. 

```go
gateway.AddResource("/example", handler)
```

### Authorizer IAM

The library supports integration with AWS IAM to authorize incoming requests. This integration ensures that only authenticated and authorized principals  can access the resources and functionalities provided by your Lambda functions. By leveraging AWS IAM policies and roles, the library enforces fine-grained access control, enhancing the security of your API endpoints.

```go
api := scud.NewGateway(stack, jsii.String("Gateway"),
  &scud.GatewayProps{}
)

// Using the IAM authorizer requires specifying a principal or role. 
role := awsiam.NewRole(/* ... */)

api.NewAuthorizerIAM().
  AddResource("/example", handler, role)
```

You can still access API with curl even with IAM authorizer is used.

```bash
curl https://example.com/petshop/pets \
  -XGET \
  -H "Accept: application/json" \
  -H "x-amz-security-token: $AWS_SESSION_TOKEN" \
  --aws-sigv4 "aws:amz:eu-west-1:execute-api" \
  --user "$AWS_ACCESS_KEY_ID:$AWS_SECRET_ACCESS_KEY"
```

### Authorizer AWS Cognito

The library supports integration with AWS Cognito to authorize incoming requests. This integration allows you to manage user authentication and authorization seamlessly. By utilizing AWS Cognito, you can implement robust user sign-up, sign-in, and access control mechanisms. The library ensures that only authenticated users with valid tokens can access your API endpoints, providing an additional layer of security and user management capabilities. The integration of AWS API Gateway and AWS Cognito is well documented in [the official documentation](https://docs.aws.amazon.com/apigateway/latest/developerguide/apigateway-integrate-with-cognito.html). This pattern facilitates the deployment of this configuration by simply providing the ARN of the user pool and specifying scopes to protect your endpoints.

```go
api := scud.NewGateway(stack, jsii.String("Gateway"),
  &scud.GatewayProps{}
)

// Cognito pool has to be pre-defined.
// Supply list of allowed clients to control the access.
api.NewAuthorizerCognito("arn:aws:cognito-idp:...", /* ... */).
  AddResource("/example", handler, "my/scope")
```

### Authorizer JWT

The library supports integration with Single Sign On provider (e.g. Auth0) to authorize incoming requests using JWT tokens. This integration allows you to leverage external identity providers for user authentication, ensuring secure and seamless access to your API endpoints. By validating JWT tokens issued by the SSO provider, the library ensures that only authenticated users can access your resources. Additionally, this setup can support various SSO standards and providers, enhancing flexibility and security in managing user identities and permissions.

```go
api := scud.NewGateway(stack, jsii.String("Gateway"),
  &scud.GatewayProps{}
)

api.NewAuthorizerJwt("https://{tenant}.eu.auth0.com/", "https://example.com").
  AddResource("/example", handler, "my/scope")
```

## HowTo Contribute

The project is [MIT](https://github.com/fogfish/scud/blob/master/LICENSE) licensed and accepts contributions via GitHub pull requests:

1. Fork it and clone 
2. Create your feature branch (`git checkout -b my-new-feature`)
3. Commit your changes (`git commit -am 'Added some feature'`)
4. Push to the branch (`git push origin my-new-feature`)
5. Create new Pull Request

```bash
git clone https://github.com/fogfish/scud
cd scud

go build
go test
```

## License

[![See LICENSE](https://img.shields.io/github/license/fogfish/scud.svg?style=for-the-badge)](LICENSE)

## References

1. [Migrating AWS Lambda functions from the Go1.x runtime to the custom runtime on Amazon Linux 2](https://aws.amazon.com/blogs/compute/migrating-aws-lambda-functions-from-the-go1-x-runtime-to-the-custom-runtime-on-amazon-linux-2/)