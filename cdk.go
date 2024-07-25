package main

import (
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/jsii-runtime-go"
	"github.com/cloudentity/awsauthorizercdk/pkg/stacks/authorizer"
	"github.com/cloudentity/awsauthorizercdk/pkg/stacks/demo"
)

func main() {
	var (
		err             error
		app             awscdk.App
		awsStackProps   awscdk.StackProps
		props           authorizer.StackProps
		authorizerStack authorizer.Stack
	)
	defer jsii.Close()
	app = awscdk.NewApp(nil)

	awsStackProps = awscdk.StackProps{
		Env: env(),
	}
	props = authorizer.StackProps{
		StackProps: awsStackProps,
	}

	if err = readStackProps(app, &props); err != nil {
		fmt.Printf("could not read context values %s", err)
		return
	}

	if authorizerStack, err = authorizer.NewStack(app, "CloudentityAWSAuthorizer", props); err != nil {
		fmt.Printf("could not create stack %s", err)
		return
	}

	if readBoolCtxParam(app, "deployDemo") {
		fmt.Println("Deploying demo stack")
		if _, err = demo.NewStack(app, "DemoAPIStack", authorizerStack.AuthorizerLambda, awsStackProps); err != nil {
			fmt.Printf("could not create demo stack %s", err)
			return
		}
	}

	app.Synth(nil)
}

func readStackProps(app awscdk.App, props *authorizer.StackProps) error {
	var err error
	props.SyncZip = readCtxParam(app, "syncZip")
	props.AuthorizerZip = readCtxParam(app, "authorizerZip")
	props.ManuallyCreateAuthorizer = readBoolCtxParam(app, "manuallyCreateAuthorizer")
	props.ClientID = readCtxParam(app, "clientID")
	// read secret from env var
	props.ClientSecret = getEnvFromVars("ACP_CLIENT_SECRET")
	props.IssuerURL = readCtxParam(app, "issuerURL")
	props.VpcID = readCtxParam(app, "vpcID")
	props.Version = readCtxParam(app, "version")
	props.LoggingLevel = readCtxParam(app, "loggingLevel")

	reloadInterval := readCtxParam(app, "reloadInterval")
	if reloadInterval != "" {
		if props.ReloadInterval, err = time.ParseDuration(reloadInterval); err != nil {
			return fmt.Errorf("invalid reloadInterval duration %w", err)
		}
	}
	props.AnalyticsDisabled = readBoolCtxParam(app, "analyticsDisabled")
	props.InjectContext = readBoolCtxParam(app, "injectContext")
	props.EnforcementAllowUnknown = readBoolCtxParam(app, "enforcementAllowUnknown")
	props.HTTPClientRootCA = readCtxParam(app, "httpClientRootCA")
	props.HTTPClientInsecureSkipVerify = readBoolCtxParam(app, "httpClientInsecureSkipVerify")
	props.S3BucketName = readCtxParam(app, "s3BucketName")

	props.StackName = jsii.String(readCtxParam(app, "stackName"))
	return nil
}

func readCtxParam(app awscdk.App, key string) string {
	val, ok := app.Node().TryGetContext(jsii.String(key)).(string)
	if !ok {
		return ""
	}
	return val
}

func readBoolCtxParam(app awscdk.App, key string) bool {
	return readCtxParam(app, key) == "true"
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
