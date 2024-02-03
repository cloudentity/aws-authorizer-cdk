package main

import (
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/jsii-runtime-go"
	"github.com/cloudentity/awsauthorizercdk/pkg/stacks/authorizer"
)

func main() {
	var (
		err   error
		app   awscdk.App
		props authorizer.StackProps
	)
	defer jsii.Close()

	app = awscdk.NewApp(nil)
	props = authorizer.StackProps{
		StackProps: awscdk.StackProps{
			Env: env(),
		},
	}

	if err = readStackProps(app, &props); err != nil {
		fmt.Printf("could not read context values %s", err)
		return
	}

	if _, err = authorizer.Stack(app, "CloudentityAWSAuthorizer", props); err != nil {
		fmt.Printf("could not create stack %s", err)
		return
	}

	app.Synth(nil)
}

func readStackProps(app awscdk.App, props *authorizer.StackProps) error {
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
	props.S3BucketName = readCtxParam[string](app, "s3BucketName")

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
