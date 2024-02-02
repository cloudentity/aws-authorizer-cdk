#
# Cloudentity AWS Authorizer CDK
#
include .env

BRANCH = $(shell git rev-parse --abbrev-ref HEAD)
HASH   = $(shell git rev-parse HEAD)

STACK_NAME = CloudentityAwsAuthorizer-$(BRANCH)

.EXPORT_ALL_VARIABLES:

CONTEXT_PARAMS = \
	-c clientID=$(ACP_CLIENT_ID) \
	-c issuerURL=$(ACP_ISSUER_URL) \
	-c stackName=$(STACK_NAME)

LOCAL_CONTEXT_PARAMS =\
	-c syncZip=$(LOCAL_LAMBDAS_DIR)aws-authorizer-sync.zip  \
	-c authorizerZip=$(LOCAL_LAMBDAS_DIR)aws-authorizer.zip  $(CONTEXT_PARAMS)

.PHONY: deploy
deploy:
	@echo "Deploying to AWS..."
	cdk bootstrap $(CONTEXT_PARAMS) && cdk deploy $(CONTEXT_PARAMS)

.PHONY: deploy-local-files
deploy-local-files:
	@echo "Deploying to AWS..."
	cdk bootstrap $(LOCAL_CONTEXT_PARAMS) && cdk deploy $(LOCAL_CONTEXT_PARAMS)

.PHONY: destroy
destroy:
	@echo "Destroying AWS resources..."
	cdk destroy $(CONTEXT_PARAMS)
