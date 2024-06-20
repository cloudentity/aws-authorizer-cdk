package authorizer

import (
	"strconv"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsefs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/jsii-runtime-go"
)

func createAuthorizerLambda(stack awscdk.Stack, vpc awsec2.IVpc, efsAP awsefs.AccessPoint, props StackProps) awslambda.Function {
	var (
		code    awslambda.Code
		env     map[string]*string
		lambda  awslambda.Function
		memSize = 128
		maxHeap = int(float64(memSize) * 0.75)
	)

	if props.AuthorizerZip != "" {
		code = getLocalCode(props.AuthorizerZip)
	} else {
		code = getCodeFromS3(stack, props, props.S3AuthorizerPrefix+props.Version+".zip")
	}

	env = map[string]*string{
		"ACP_CLIENT_ID":                    jsii.String(props.ClientID),
		"ACP_CLIENT_SECRET":                jsii.String(props.ClientSecret),
		"ACP_ISSUER_URL":                   jsii.String(props.IssuerURL),
		"LOGGING_LEVEL":                    jsii.String(props.LoggingLevel),
		"ANALYTICS_ENABLED":                jsii.String(strconv.FormatBool(props.AnalyticsEnabled)),
		"HTTP_CLIENT_ROOT_CA":              jsii.String(props.HTTPClientRootCA),
		"HTTP_CLIENT_INSECURE_SKIP_VERIFY": jsii.String(strconv.FormatBool(props.HTTPClientInsecureSkipVerify)),
		"AWS_LOCAL_CONFIGURATION":          jsii.String(EfsMountPath),
		"MAX_HEAP":                         jsii.String(strconv.Itoa(maxHeap)),
	}

	lambda = awslambda.NewFunction(stack, jsii.String("AuthorizerLambda"), &awslambda.FunctionProps{
		Code:        code,
		Handler:     jsii.String("bootstrap"),
		Runtime:     awslambda.Runtime_PROVIDED_AL2023(),
		MemorySize:  jsii.Number(128),
		Timeout:     awscdk.Duration_Seconds(jsii.Number(10)),
		Environment: &env,
		Vpc:         vpc,
		Filesystem:  awslambda.FileSystem_FromEfsAccessPoint(efsAP, jsii.String(EfsMountPath)),
	})

	return lambda
}
