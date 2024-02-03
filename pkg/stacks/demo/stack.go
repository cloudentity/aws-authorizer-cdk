package demo

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigateway"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

func NewStack(scope constructs.Construct, id string, authorizerLambda awslambda.Function, props awscdk.StackProps) (awscdk.Stack, error) {
	stack := awscdk.NewStack(scope, &id, &props)

	createAPI(stack, *authorizerLambda.FunctionArn())
	return stack, nil
}

func createAPI(stack awscdk.Stack, authorizerLambdaArn string) {
	api := awsapigateway.NewRestApi(stack, jsii.String("SampleAPI"), &awsapigateway.RestApiProps{
		RestApiName: jsii.String("SampleAPI"),
		Description: jsii.String("Sample API"),
	})

	api.Root().AddMethod(jsii.String("GET"), awsapigateway.NewMockIntegration(&awsapigateway.IntegrationOptions{}), &awsapigateway.MethodOptions{
		AuthorizationType: awsapigateway.AuthorizationType_CUSTOM,
		Authorizer: awsapigateway.NewRequestAuthorizer(stack, jsii.String("SampleAuthorizer"), &awsapigateway.RequestAuthorizerProps{
			Handler:        awslambda.Function_FromFunctionArn(stack, jsii.String("SampleAuthorizerHandler"), jsii.String(authorizerLambdaArn)),
			AuthorizerName: jsii.String("CloudentityAWSAuthorizer"),
			IdentitySources: &[]*string{
				jsii.String("method.request.header.Authorization"),
			},
		}),
	})
}
