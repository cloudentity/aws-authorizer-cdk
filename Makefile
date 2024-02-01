#
# Cloudentity AWS Authorizer CDK
#
include .env

CONTEXT_PARAMS = -c clientID=$(ACP_CLIENT_ID) -c issuerURL=$(ACP_ISSUER_URL)
LOCAL_CONTEXT_PARAMS = -c syncZip=./build/aws-authorizer-sync.zip  -c authorizerZip=./build/aws-authorizer.zip  $(CONTEXT_PARAMS)

.PHONY: deploy-local-files
local-deploy:
	@echo "Deploying to AWS..."
	cd cdk && cdk bootstrap $(LOCAL_CONTEXT_PARAMS) && cdk deploy $(LOCAL_CONTEXT_PARAMS)
