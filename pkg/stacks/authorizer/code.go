package authorizer

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3assets"
	"github.com/aws/jsii-runtime-go"
)

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
			jsii.String(props.S3BucketName+"-"+*stack.Region()),
		),
		jsii.String(s3FileName),
		nil,
	)
}
