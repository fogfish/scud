//
// Copyright (C) 2020 - 2025 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/scud
//

package scud

import (
	"cmp"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// Defaults of functions (and log groups) created by the library within
// a scope. Properties given explicitly to the function take precedence.
//
// Functions behind API Gateway HTTP API must have the timeout shorter than
// the integration timeout (30 seconds by default, maximum).
type Defaults struct {
	// Function timeout, the library's default is 1 minute.
	Timeout awscdk.Duration

	// Function memory size (MB), the AWS Lambda's default is 128 MB.
	MemorySize *float64

	// Log group shared by all functions in the scope.
	// When defined, per-function log groups are not created.
	LogGroup awslogs.ILogGroup

	// Prefix of per-function log group names, e.g. "/myapp/mystack".
	// The log group is named <prefix>/<function name>.
	// The library's default is the function name.
	LogGroupPrefix string

	// Retention of per-function log groups, the library's default is 5 days.
	LogRetention awslogs.RetentionDays

	// Removal policy of per-function log groups, the library's default is DESTROY.
	LogRemovalPolicy awscdk.RemovalPolicy

	// Format of function logs, the library's default is JSON.
	// Use TEXT to opt out, log levels do not apply to it.
	LoggingFormat awslambda.LoggingFormat

	// Level of logs written by AWS Lambda itself, the library's default is WARN:
	// START, END and REPORT lines are not logged. Use INFO to log them.
	SystemLogLevel awslambda.SystemLogLevel

	// Level of logs written by the function, the library's default is INFO.
	ApplicationLogLevel awslambda.ApplicationLogLevel
}

const defaultsConstructId = "ScudDefaults"
const defaultsContextKey = "@fogfish/scud:defaults"

var (
	defaultsLock     sync.Mutex
	defaultsRegistry = map[string]*Defaults{}
	defaultsSeq      = 0
)

// SetDefaults defines defaults of functions created within the scope
// (stack or any construct) and its descendants. The defaults set on
// the nearest ancestor apply. The function can be called once per scope.
func SetDefaults(scope constructs.Construct, defaults *Defaults) {
	defaultsLock.Lock()
	defer defaultsLock.Unlock()

	defaultsSeq++
	key := fmt.Sprintf("%d", defaultsSeq)
	defaultsRegistry[key] = defaults

	// the marker construct has no resources, it binds defaults to the scope
	marker := constructs.NewConstruct(scope, jsii.String(defaultsConstructId))
	marker.Node().SetContext(jsii.String(defaultsContextKey), jsii.String(key))
}

// lookup defaults of the nearest ancestor
func defaultsOf(scope constructs.Construct) *Defaults {
	for node := constructs.IConstruct(scope); node != nil; node = node.Node().Scope() {
		marker := node.Node().TryFindChild(jsii.String(defaultsConstructId))
		if marker == nil {
			continue
		}

		val, ok := marker.Node().TryGetContext(jsii.String(defaultsContextKey)).(string)
		if !ok {
			continue
		}

		defaultsLock.Lock()
		defaults := defaultsRegistry[val]
		defaultsLock.Unlock()

		if defaults != nil {
			return defaults
		}
	}

	return &Defaults{}
}

// function timeout
func (d *Defaults) timeout() awscdk.Duration {
	if d.Timeout != nil {
		return d.Timeout
	}
	return awscdk.Duration_Minutes(jsii.Number(1))
}

// create log group for the function, unless shared one is defined
func (d *Defaults) logGroup(scope constructs.Construct, id string, functionName *string) awslogs.ILogGroup {
	if d.LogGroup != nil {
		return d.LogGroup
	}

	name := functionName
	if d.LogGroupPrefix != "" {
		name = jsii.String(strings.TrimSuffix(d.LogGroupPrefix, "/") + "/" + *functionName)
	}

	retention := d.LogRetention
	if retention == "" {
		retention = awslogs.RetentionDays_FIVE_DAYS
	}

	removal := d.LogRemovalPolicy
	if removal == "" {
		removal = awscdk.RemovalPolicy_DESTROY
	}

	// See: https://docs.aws.amazon.com/lambda/latest/dg/monitoring-cloudwatchlogs-loggroups.html
	return awslogs.NewLogGroup(scope, jsii.String(id),
		&awslogs.LogGroupProps{
			LogGroupName:  name,
			Retention:     retention,
			RemovalPolicy: removal,
		},
	)
}

// logging properties of the function, including deprecated ones.
// AWS CDK does not accept a property together with its deprecated variant.
type logging struct {
	format         *awslambda.LoggingFormat
	system         *awslambda.SystemLogLevel
	application    *awslambda.ApplicationLogLevel
	oldFormat      *string
	oldSystem      *string
	oldApplication *string
}

// is JSON format used, only this format supports log levels
func (l logging) isJSON() bool {
	if l.oldFormat != nil {
		return *l.oldFormat == string(awslambda.LoggingFormat_JSON)
	}
	return *l.format == awslambda.LoggingFormat_JSON
}

// define logging properties that are not given explicitly to the function
func (d *Defaults) logging(l logging) {
	if *l.format == "" && l.oldFormat == nil {
		*l.format = cmp.Or(d.LoggingFormat, awslambda.LoggingFormat_JSON)
	}

	if !l.isJSON() {
		return
	}

	if *l.system == "" && l.oldSystem == nil {
		*l.system = cmp.Or(d.SystemLogLevel, awslambda.SystemLogLevel_WARN)
	}

	if *l.application == "" && l.oldApplication == nil {
		*l.application = cmp.Or(d.ApplicationLogLevel, awslambda.ApplicationLogLevel_INFO)
	}
}

// AWS Lambda limits function name to 64 characters
const maxFunctionNameLength = 64

// default function name: <stack name>-<shortened path>
func defaultFunctionName(scope constructs.Construct, uniqueID string) *string {
	name := jsii.Sprintf("%s-%s", *awscdk.Aws_STACK_NAME(), uniqueID)

	// the name is a token, validate the length using actual stack name
	stack := awscdk.Stack_Of(scope).StackName()
	if !*awscdk.Token_IsUnresolved(stack) {
		validateFunctionName(fmt.Sprintf("%s-%s", *stack, uniqueID))
	}

	return name
}

func validateFunctionName(name string) {
	if len(name) > maxFunctionNameLength {
		panic(fmt.Errorf("function name %s is %d characters long, AWS Lambda allows at most %d; set FunctionName explicitly or shorten the stack name",
			name, len(name), maxFunctionNameLength))
	}
}

func checkFunctionName(name *string) {
	if name != nil && !*awscdk.Token_IsUnresolved(name) {
		validateFunctionName(*name)
	}
}
