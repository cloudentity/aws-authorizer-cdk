package authorizer

import (
	"strconv"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsefs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/jsii-runtime-go"
)

func createSyncLambda(stack awscdk.Stack, authorizer awslambda.Function, vpc awsec2.IVpc, efsAP awsefs.AccessPoint, props StackProps) awslambda.Function {
	var (
		code    awslambda.Code
		lambda  awslambda.Function
		memSize = 128
		maxHeap = int(float64(memSize) * 0.75)
	)
	if props.SyncZip != "" {
		code = getLocalCode(props.SyncZip)
	} else {
		code = getCodeFromS3(stack, props, props.S3SyncPrefix+props.Version+".zip")
	}
	syncLambdaEnvVars := map[string]*string{
		"ACP_CLIENT_ID":                    jsii.String(props.ClientID),
		"ACP_CLIENT_SECRET":                jsii.String(props.ClientSecret),
		"ACP_ISSUER_URL":                   jsii.String(props.IssuerURL),
		"LOGGING_LEVEL":                    jsii.String(props.LoggingLevel),
		"ANALYTICS_ENABLED":                jsii.String(strconv.FormatBool(props.AnalyticsEnabled)),
		"HTTP_CLIENT_ROOT_CA":              jsii.String(props.HTTPClientRootCA),
		"HTTP_CLIENT_INSECURE_SKIP_VERIFY": jsii.String(strconv.FormatBool(props.HTTPClientInsecureSkipVerify)),
		"AWS_LOCAL_CONFIGURATION":          jsii.String(EfsMountPath),
		"AWS_AUTHORIZER_ARN":               authorizer.FunctionArn(),
		"AWS_AUTOBIND_AUTHORIZER":          jsii.String(strconv.FormatBool(!props.ManuallyCreateAuthorizer)),
		"MAX_HEAP":                         jsii.String(strconv.Itoa(maxHeap)),
	}

	lambda = awslambda.NewFunction(stack, jsii.String("SyncLambda"), &awslambda.FunctionProps{
		Code:                         code,
		Handler:                      jsii.String("bootstrap"),
		Runtime:                      awslambda.Runtime_PROVIDED_AL2023(),
		MemorySize:                   jsii.Number(128),
		Timeout:                      awscdk.Duration_Seconds(jsii.Number(10)),
		Environment:                  &syncLambdaEnvVars,
		Vpc:                          vpc,
		Filesystem:                   awslambda.FileSystem_FromEfsAccessPoint(efsAP, jsii.String(EfsMountPath)),
		ReservedConcurrentExecutions: jsii.Number(1),
	})

	attachSyncLambdaPolicy(stack, lambda, props)

	return lambda
}

func attachSyncLambdaPolicy(stack awscdk.Stack, lambda awslambda.Function, props StackProps) {
	statements := []awsiam.PolicyStatement{
		awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
			Actions: &[]*string{
				jsii.String("apigateway:GET"),
			},
			Resources: &[]*string{
				jsii.String("arn:aws:apigateway:*::/restapis/*/deployments/*"),
				jsii.String("arn:aws:apigateway:*::/restapis/*/resources"),
				jsii.String("arn:aws:apigateway:*::/restapis/*/authorizers"),
				jsii.String("arn:aws:apigateway:*::/restapis/*/stages"),
				jsii.String("arn:aws:apigateway:*::/restapis"),
			},
		}),
	}

	// add auto-bind authorizer permissions
	if !props.ManuallyCreateAuthorizer {
		statements = append(statements,
			awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Actions: &[]*string{
					jsii.String("lambda:AddPermission"),
				},
				Resources: &[]*string{
					jsii.String("*"),
				},
			}),
			awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Actions: &[]*string{
					jsii.String("apigateway:PATCH"),
				},
				Resources: &[]*string{
					jsii.String("arn:aws:apigateway:*::/restapis/*/resources/*/methods/*"),
				},
			}),
			awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Actions: &[]*string{
					jsii.String("apigateway:POST"),
				},
				Resources: &[]*string{
					jsii.String("arn:aws:apigateway:*::/restapis/*/authorizers"),
				},
			}))
	}

	lambda.Role().AttachInlinePolicy(awsiam.NewPolicy(stack, jsii.String("SyncLambdaPolicy"), &awsiam.PolicyProps{
		Statements: &statements,
	}))
}
