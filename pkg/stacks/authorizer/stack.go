package authorizer

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsefs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

const (
	EfsApPath    = "/ceauthconfig"
	EfsMountPath = "/mnt" + EfsApPath

	EventBridgeTriggerIntervalMinutes = 1
)

func Stack(scope constructs.Construct, id string, props StackProps) (awscdk.Stack, error) {
	var (
		err              error
		sprops           awscdk.StackProps
		stack            awscdk.Stack
		vpc              awsec2.IVpc
		efsAP            awsefs.AccessPoint
		authorizerLambda awslambda.Function
		syncLambda       awslambda.Function
	)
	setDefaultStackProps(&props)
	sprops = props.StackProps

	if err = validateStackProps(props); err != nil {
		return nil, fmt.Errorf("invalid stack props %w", err)
	}
	stack = awscdk.NewStack(scope, &id, &sprops)

	vpc = getVpc(stack, props.VpcID)
	efsAP = createEFSWithAccessPoint(stack, vpc)
	authorizerLambda = createAuthorizerLambda(stack, vpc, efsAP, props)
	syncLambda = createSyncLambda(stack, authorizerLambda, vpc, efsAP, props)
	triggerLambdaInIntervals(stack, syncLambda, props)

	return stack, nil
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
