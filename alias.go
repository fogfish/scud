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

	"github.com/aws/aws-cdk-go/awscdk/v2"
	apigw2 "github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscertificatemanager"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53targets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsssm"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// AliasProps defines a stable custom domain mapped to a (versioned) HTTP API
// deployed by another stack.
type AliasProps struct {
	// Custom domain name of the alias (e.g. api.example.com)
	Host *string

	// Hosted zone (domain name) where the record is created.
	// Default is Host, the record is created at the zone apex.
	Zone *string

	// ARN of the certificate for the custom domain name.
	TlsArn *string

	// HTTP API the alias is mapped to. Use either Api, ApiId or ApiIdParameter.
	Api apigw2.IHttpApi

	// Id of HTTP API the alias is mapped to (e.g. stack output of the version).
	ApiId *string

	// Name of SSM parameter holding id of HTTP API the alias is mapped to
	// (see GatewayProps.ApiIdParameter).
	ApiIdParameter *string

	// Stage of the API, default is $default.
	Stage *string
}

// Alias is a stable custom domain name mapped to HTTP API.
//
// A version of API is an immutable deployment with its own host
// (e.g. v1.api.example.com). The alias (e.g. api.example.com) follows the
// live version. API Gateway custom domain answers only to its own name,
// so the alias is a custom domain of its own with an API mapping to the
// version's API. Deploy the alias by a stack that outlives versions,
// switching the version updates the API mapping in place.
type Alias struct {
	constructs.Construct
	Domain  apigw2.DomainName
	Mapping apigw2.ApiMapping
	Record  awsroute53.ARecord
}

func NewAlias(scope constructs.Construct, id *string, props *AliasProps) *Alias {
	if props.Host == nil || props.TlsArn == nil {
		panic(fmt.Errorf("alias requires Host and TlsArn"))
	}

	alias := &Alias{Construct: constructs.NewConstruct(scope, id)}

	api := props.Api
	if api == nil {
		apiId := props.ApiId
		if apiId == nil && props.ApiIdParameter != nil {
			apiId = awsssm.StringParameter_ValueForStringParameter(alias.Construct, props.ApiIdParameter, nil)
		}
		if apiId == nil {
			panic(fmt.Errorf("alias requires one of Api, ApiId or ApiIdParameter"))
		}

		api = apigw2.HttpApi_FromHttpApiAttributes(alias.Construct, jsii.String("Api"),
			&apigw2.HttpApiAttributes{HttpApiId: apiId},
		)
	}

	stageName := props.Stage
	if stageName == nil {
		stageName = jsii.String("$default")
	}

	stage := apigw2.HttpStage_FromHttpStageAttributes(alias.Construct, jsii.String("Stage"),
		&apigw2.HttpStageAttributes{
			Api:       api,
			StageName: stageName,
		},
	)

	zone := props.Zone
	if zone == nil {
		zone = props.Host
	}

	alias.Domain = newDomainName(alias.Construct, props.Host, props.TlsArn)

	alias.Mapping = apigw2.NewApiMapping(alias.Construct, jsii.String("Mapping"),
		&apigw2.ApiMappingProps{
			Api:        api,
			DomainName: alias.Domain,
			Stage:      stage,
		},
	)

	alias.Record = newDomainRecord(alias.Construct, props.Host, zone, alias.Domain)

	return alias
}

//------------------------------------------------------------------------------

func newDomainName(scope constructs.Construct, host, tlsArn *string) apigw2.DomainName {
	return apigw2.NewDomainName(scope, jsii.String("DomainName"),
		&apigw2.DomainNameProps{
			EndpointType: apigw2.EndpointType_REGIONAL,
			DomainName:   host,
			Certificate:  awscertificatemanager.Certificate_FromCertificateArn(scope, jsii.String("X509"), tlsArn),
		},
	)
}

// create alias record of the custom domain at the hosted zone, the zone is
// the host without first label unless defined explicitly.
func newDomainRecord(scope constructs.Construct, host, zone *string, domain apigw2.DomainName) awsroute53.ARecord {
	zoneName := ""
	if zone != nil {
		zoneName = *zone
	} else {
		zoneName = strings.Join(strings.Split(*host, ".")[1:], ".")
	}

	hz := awsroute53.HostedZone_FromLookup(scope, jsii.String("HZone"),
		&awsroute53.HostedZoneProviderProps{
			DomainName: jsii.String(zoneName),
		},
	)

	return awsroute53.NewARecord(scope, jsii.String("ARecord"),
		&awsroute53.ARecordProps{
			RecordName: host,
			Target: awsroute53.RecordTarget_FromAlias(
				awsroute53targets.NewApiGatewayv2DomainProperties(domain.RegionalDomainName(), domain.RegionalHostedZoneId()),
			),
			Ttl:  awscdk.Duration_Seconds(jsii.Number(60)),
			Zone: hz,
		},
	)
}
