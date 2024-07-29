package authorizer

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsefs"
	"github.com/aws/jsii-runtime-go"
)

func createEFSWithAccessPoint(stack awscdk.Stack, vpc awsec2.IVpc) awsefs.AccessPoint {

	var (
		fs awsefs.FileSystem
		ap awsefs.AccessPoint
	)

	fs = awsefs.NewFileSystem(stack, jsii.String("AuthorizerConfigurationFileSystem"), &awsefs.FileSystemProps{
		Vpc:           vpc,
		RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
	})

	ap = fs.AddAccessPoint(jsii.String("EFSAccessPoint"), &awsefs.AccessPointOptions{
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
	ap.ApplyRemovalPolicy(awscdk.RemovalPolicy_DESTROY)

	return ap
}
