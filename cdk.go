package main

import (
	"fmt"
	"os"
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
}

var DefaultStackProps = StackProps{
	LoggingLevel:   "info",
	ReloadInterval: time.Second * 10,
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
}

func createAuthorizerLambda(stack awscdk.Stack, vpc awsec2.IVpc, efsAP awsefs.AccessPoint, props StackProps) awslambda.Function {
	var (
		code     awslambda.Code
		env      map[string]*string
		localZip = props.AuthorizerZip
		lambda   awslambda.Function
		memSize  = 128
		maxHeap  = int(float64(memSize) * 0.75)
	)

	code = awslambda.Code_FromAsset(
		jsii.String(localZip),
		&awss3assets.AssetOptions{},
	)
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
		code     awslambda.Code
		lambda   awslambda.Function
		localZip = props.SyncZip
		memSize  = 128
		maxHeap  = int(float64(memSize) * 0.75)
	)
	if localZip != "" {
		code = awslambda.Code_FromAsset(
			jsii.String(localZip),
			&awss3assets.AssetOptions{},
		)
	} else {
		code = awslambda.Code_FromBucket(
			awss3.Bucket_FromBucketName(
				stack,
				jsii.String("S3Bucket"),
				// TODO proper s3 bucket name
				// maybe from context
				jsii.String("cloudentity-aws-api-gateway-authorizer-sync"),
			),
			jsii.String("aws-authorizer-sync.zip"),
			jsii.String(props.Version),
		)
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

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	props := StackProps{
		StackProps: awscdk.StackProps{
			Env: env(),
		},
	}

	if err := readStackProps(app, &props); err != nil {
		fmt.Printf("could not read context values %s", err)
		return
	}

	if _, err := Stack(app, "CloudentityAWSAuthorizer", props); err != nil {
		fmt.Printf("could not create stack %s", err)
		return
	}

	app.Synth(nil)
}

func readStackProps(app awscdk.App, props *StackProps) error {
	var err error
	props.SyncZip = readCtxParam[string](app, "syncZip")
	props.AuthorizerZip = readCtxParam[string](app, "authorizerZip")
	props.ManuallyCreateAuthorizer = readCtxParam[bool](app, "manuallyCreateAuthorizer")
	props.ClientID = readCtxParam[string](app, "clientID")
	// read secret from env var
	props.ClientSecret = getEnvFromVars("ACP_CLIENT_SECRET")
	props.IssuerURL = readCtxParam[string](app, "issuerURL")
	props.VpcID = readCtxParam[string](app, "vpcID")
	props.Version = readCtxParam[string](app, "version")
	props.LoggingLevel = readCtxParam[string](app, "loggingLevel")

	reloadInterval := readCtxParam[string](app, "reloadInterval")
	if reloadInterval != "" {
		if props.ReloadInterval, err = time.ParseDuration(reloadInterval); err != nil {
			return fmt.Errorf("invalid reloadInterval duration %w", err)
		}
	}
	props.AnalyticsEnabled = readCtxParam[bool](app, "analyticsEnabled")
	props.InjectContext = readCtxParam[bool](app, "injectContext")
	props.EnforcementAllowUnknown = readCtxParam[bool](app, "enforcementAllowUnknown")
	props.HTTPClientRootCA = readCtxParam[string](app, "httpClientRootCA")
	props.HTTPClientInsecureSkipVerify = readCtxParam[bool](app, "httpClientInsecureSkipVerify")
	props.StackName = jsii.String(readCtxParam[string](app, "stackName"))
	return nil
}

func readCtxParam[T any](app awscdk.App, key string) T {
	var t T
	val, ok := app.Node().TryGetContext(jsii.String(key)).(T)
	if !ok {
		return t
	}
	return val
}

func env() *awscdk.Environment {
	return &awscdk.Environment{
		Account: jsii.String(getEnvFromVars("CDK_DEPLOY_ACCOUNT", "CDK_DEFAULT_ACCOUNT")),
		Region:  jsii.String(getEnvFromVars("CDK_DEPLOY_REGION", "CDK_DEFAULT_REGION")),
	}
}

func getEnvFromVars(vars ...string) string {
	for _, v := range vars {
		if val, ok := os.LookupEnv(v); ok {
			return val
		}
	}
	return ""
}
