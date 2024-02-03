package authorizer

import (
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsefs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsevents"
	"github.com/aws/aws-cdk-go/awscdk/v2/awseventstargets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambdaeventsources"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3assets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssqs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctionstasks"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
	"github.com/go-playground/validator/v10"
)

const (
	EfsApPath    = "/ceauthconfig"
	EfsMountPath = "/mnt" + EfsApPath

	EventBridgeTriggerIntervalMinutes = 1
)

type StackProps struct {
	awscdk.StackProps
	// SyncZip is a path to zip file with sync lambda function
	SyncZip string
	// AuthorizerZip is a path to zip file with authorizer lambda function
	AuthorizerZip string
	// When ManuallyCreateAuthorizer is set to true, the stack will configure sync lambda to skip auto-binding authorizer
	ManuallyCreateAuthorizer bool
	// ClientID is a client id of the client that will be used to authenticate with ACP
	ClientID string `validate:"required"`
	// ClientSecret is a client secret of the client that will be used to authenticate with ACP
	ClientSecret string `validate:"required"`
	// IssuerURL is an issuer url of ACP
	IssuerURL string `validate:"required"`
	// VpcID is an id of VPC that will be used to create lambda function
	VpcID string
	// Version is a version of lambda function
	Version string `validate:"omitempty,semver"`
	// LoggingLevel is a logging level of lambda function
	LoggingLevel string `validate:"omitempty,oneof=debug info warn error"`
	// ReloadInterval is a reload interval of lambda function
	ReloadInterval time.Duration `validate:"omitempty,max=1m,min=1s"`
	// AnalyticsEnabled is a flag that enables analytics
	AnalyticsEnabled bool
	// InjectContext is a flag that enables injecting context to the request
	InjectContext bool
	// EnforcementAllowUnknown is a flag that enables allowing unknown enforcement
	EnforcementAllowUnknown bool
	// HTTPClientRootCA is a root CA of HTTP client
	HTTPClientRootCA string
	// HTTPClientInsecureSkipVerify is a flag that enables skipping HTTP client verification
	HTTPClientInsecureSkipVerify bool
	// S3BucketName is a name of S3 bucket
	S3BucketName string
}

var DefaultStackProps = StackProps{
	LoggingLevel:   "info",
	ReloadInterval: time.Second * 10,
	S3BucketName:   "cloudentity-aws-api-gateway-authorizer-us-east-1",
}

func Stack(scope constructs.Construct, id string, props StackProps) (awscdk.Stack, error) {
	setDefaultStackProps(&props)
	sprops := props.StackProps

	if err := validateStackProps(props); err != nil {
		return nil, fmt.Errorf("invalid stack props %w", err)
	}

	stack := awscdk.NewStack(scope, &id, &sprops)

	vpc := getVpc(stack, props.VpcID)

	fs := awsefs.NewFileSystem(stack, jsii.String("AuthorizerConfigurationFileSystem"), &awsefs.FileSystemProps{
		Vpc: vpc,
	})

	authorizerEfsAP := createEFSAccessPoint(stack, "AuthorizerEFSAccessPoint", fs)
	authorizerLambda := createAuthorizerLambda(stack, vpc, authorizerEfsAP, props)

	syncEfsAP := createEFSAccessPoint(stack, "SyncEFSAccessPoint", fs)

	sqsQueue := createSQSQueue(stack)
	createSyncLambda(stack, authorizerLambda, vpc, syncEfsAP, sqsQueue, props)

	stateMachine := createStateMachine(stack, sqsQueue, props)
	createEventBridgeRule(stack, stateMachine)

	return stack, nil
}

func setDefaultStackProps(props *StackProps) {
	if props.LoggingLevel == "" {
		props.LoggingLevel = DefaultStackProps.LoggingLevel
	}
	if props.ReloadInterval == 0 {
		props.ReloadInterval = DefaultStackProps.ReloadInterval
	}
	if props.S3BucketName == "" {
		props.S3BucketName = DefaultStackProps.S3BucketName
	}
}

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
		code = getCodeFromS3(stack, props, "cloudentity-aws-authorizer-v2-"+props.Version+".zip")
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

func createSyncLambda(stack awscdk.Stack, authorizer awslambda.Function, vpc awsec2.IVpc, efsAP awsefs.AccessPoint, sqsQueue awssqs.Queue, props StackProps) awslambda.Function {
	var (
		code    awslambda.Code
		lambda  awslambda.Function
		memSize = 128
		maxHeap = int(float64(memSize) * 0.75)
	)
	if props.SyncZip != "" {
		code = getLocalCode(props.SyncZip)
	} else {
		code = getCodeFromS3(stack, props, "cloudentity-aws-authorizer-v2-sync-"+props.Version+".zip")
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

	lambda.AddEventSource(awslambdaeventsources.NewSqsEventSource(sqsQueue, &awslambdaeventsources.SqsEventSourceProps{
		BatchSize: jsii.Number(1),
	}))

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

	return lambda
}

func getLocalCode(localPath string) awslambda.Code {
	return awslambda.Code_FromAsset(
		jsii.String(localPath),
		&awss3assets.AssetOptions{},
	)
}

func getCodeFromS3(stack awscdk.Stack, props StackProps, s3FileName string) awslambda.Code {
	return awslambda.Code_FromBucket(
		awss3.Bucket_FromBucketName(
			stack,
			jsii.String("S3Bucket"+s3FileName),
			jsii.String(props.S3BucketName),
		),
		jsii.String(s3FileName),
		nil,
	)
}

func validateStackProps(props StackProps) error {
	validate := validator.New()
	return validate.Struct(props)
}

func getVpc(stack awscdk.Stack, vpcID string) awsec2.IVpc {
	if vpcID != "" {
		return awsec2.Vpc_FromLookup(stack, jsii.String("VPC"), &awsec2.VpcLookupOptions{
			VpcId: jsii.String(vpcID),
		})
	} else {
		return awsec2.NewVpc(stack, jsii.String("VPC"), &awsec2.VpcProps{})
	}
}

func createEFSAccessPoint(stack awscdk.Stack, id string, fs awsefs.FileSystem) awsefs.AccessPoint {
	accessPoint := fs.AddAccessPoint(jsii.String(id), &awsefs.AccessPointOptions{
		Path: jsii.String(EfsApPath),
		CreateAcl: &awsefs.Acl{
			OwnerGid:    jsii.String("1001"), // Using POSIX user and group
			OwnerUid:    jsii.String("1001"), // it can be adjusted accordingly
			Permissions: jsii.String("750"),
		},
		PosixUser: &awsefs.PosixUser{
			Uid: jsii.String("1001"),
			Gid: jsii.String("1001"),
		},
	})
	return accessPoint
}

func createSQSQueue(stack awscdk.Stack) awssqs.Queue {
	deadLetterQueue := awssqs.NewQueue(stack, jsii.String("DeadLetterQueue"), &awssqs.QueueProps{
		RetentionPeriod: awscdk.Duration_Minutes(jsii.Number(1)),
	})

	return awssqs.NewQueue(stack, jsii.String("SQSQueue"), &awssqs.QueueProps{
		VisibilityTimeout: awscdk.Duration_Seconds(jsii.Number(10)),
		DeadLetterQueue: &awssqs.DeadLetterQueue{
			Queue:           deadLetterQueue,
			MaxReceiveCount: jsii.Number(1),
		},
	})
}

func createStateMachine(stack awscdk.Stack, queue awssqs.Queue, props StackProps) awsstepfunctions.StateMachine {
	var (
		seconds  = int(props.ReloadInterval.Seconds())
		count    = EventBridgeTriggerIntervalMinutes * 60 / seconds
		sqsTasks = make([]awsstepfunctionstasks.SqsSendMessage, count)
	)
	for i := 0; i < count; i++ {
		sqsTasks[i] = awsstepfunctionstasks.NewSqsSendMessage(stack, jsii.String(fmt.Sprintf("Send Delayed SQS Trigger Message - %d seconds", i*seconds)), &awsstepfunctionstasks.SqsSendMessageProps{
			MessageBody: awsstepfunctions.TaskInput_FromText(jsii.String("Sync")),
			Queue:       queue,
			Delay:       awscdk.Duration_Seconds(jsii.Number(i * seconds)),
		})
	}

	definition := awsstepfunctions.Chain_Start(sqsTasks[0])
	for _, sqsTask := range sqsTasks[1:] {
		definition = definition.Next(sqsTask)
	}

	return awsstepfunctions.NewStateMachine(stack, jsii.String("Sync Looper"), &awsstepfunctions.StateMachineProps{
		DefinitionBody: awsstepfunctions.ChainDefinitionBody_FromChainable(definition),
	})
}

func createEventBridgeRule(stack awscdk.Stack, syncLooper awsstepfunctions.StateMachine) {
	rule := awsevents.NewRule(stack, jsii.String("Run Step Function"), &awsevents.RuleProps{
		Schedule: awsevents.Schedule_Rate(awscdk.Duration_Minutes(jsii.Number(EventBridgeTriggerIntervalMinutes))),
	})
	rule.AddTarget(awseventstargets.NewSfnStateMachine(syncLooper, &awseventstargets.SfnStateMachineProps{}))
}
