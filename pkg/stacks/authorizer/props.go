package authorizer

import (
	"time"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/go-playground/validator/v10"
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
	// S3AuthorizerPrefix is the file name prefix for authorizer lambda
	S3AuthorizerPrefix string
	// S3SyncPrefix is the file name prefix for sync lambda
	S3SyncPrefix string
}

var DefaultStackProps = StackProps{
	LoggingLevel:       "info",
	ReloadInterval:     time.Second * 10,
	S3BucketName:       "cloudentity-aws-api-gateway-authorizer",
	S3AuthorizerPrefix: "cloudentity-aws-authorizer-v2-",
	S3SyncPrefix:       "cloudentity-aws-authorizer-v2-sync-",
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

func validateStackProps(props StackProps) error {
	validate := validator.New()
	return validate.Struct(props)
}
