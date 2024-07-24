#
# Cloudentity AWS Authorizer CDK
#
include .env

BRANCH = $(shell git rev-parse --abbrev-ref HEAD)
HASH   = $(shell git rev-parse HEAD)

ifeq ($(STACK_NAME),)
STACK_NAME = CloudentityAwsAuthorizer-$(BRANCH)
endif

VERSION = 2.22.0-1

.EXPORT_ALL_VARIABLES:

CONTEXT_PARAMS = \
	-c clientID=$(ACP_CLIENT_ID) \
	-c issuerURL=$(ACP_ISSUER_URL) \
	-c stackName=$(STACK_NAME) \
	-c version=$(VERSION)

ifneq ($(LOGGING_LEVEL),)
	CONTEXT_PARAMS := $(CONTEXT_PARAMS) -c loggingLevel=$(LOGGING_LEVEL)
endif


LOCAL_CONTEXT_PARAMS =\
	-c syncZip=$(realpath $(LOCAL_LAMBDAS_DIR))/aws-authorizer-sync.zip  \
	-c authorizerZip=$(realpath $(LOCAL_LAMBDAS_DIR))/aws-authorizer.zip  $(CONTEXT_PARAMS)

DEMO_CONTEXT_PARAMS =\
	-c manuallyCreateAuthorizer=true \
	-c deployDemo=true $(CONTEXT_PARAMS)

.PHONY: bootstrap
bootstrap:
	@echo "Bootstrapping AWS environment..."
	cdk bootstrap $(CONTEXT_PARAMS)

.PHONY: deploy
deploy:
	@echo "Deploying to AWS..."
	cdk deploy $(CONTEXT_PARAMS)

.PHONY: deploy-local-files
deploy-local-files:
	@echo "Deploying to AWS..."
	cdk deploy $(LOCAL_CONTEXT_PARAMS)

.PHONY: destroy
destroy:
	@echo "Destroying AWS resources..."
	cdk destroy $(CONTEXT_PARAMS)

.PHONY: deploy-with-demo-api
deploy-with-demo-api:
	@echo "Deploying to AWS with Demo API..."
	cdk deploy $(DEMO_CONTEXT_PARAMS) --all
