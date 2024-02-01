#
# Cloudentity AWS Authorizer CDK
#
include .env

CONTEXT_PARAMS = \
	-c clientID=$(ACP_CLIENT_ID) \
	-c issuerURL=$(ACP_ISSUER_URL)

LOCAL_CONTEXT_PARAMS =\
	-c syncZip=$(LOCAL_LAMBDAS_DIR)aws-authorizer-sync.zip  \
	-c authorizerZip=$(LOCAL_LAMBDAS_DIR)aws-authorizer.zip  $(CONTEXT_PARAMS)

.PHONY: deploy-local-files
deploy-local-files:
	@echo "Deploying to AWS..."
	cdk bootstrap $(LOCAL_CONTEXT_PARAMS) && cdk deploy $(LOCAL_CONTEXT_PARAMS)
