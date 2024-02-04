# Cloudentity AWS Authorizer CDK

This project provides CDK deployment for Cloudentity AWS Authorizer.

## Disclaimer

This repository is currently in alpha stage and is subject to change.

## Prerequisites

- CDK
- Go

## Deploy

Run `make deploy` to deploy the stack using authorizer from S3.
Run `make deploy-local` to deploy the stack using authorizer from local `.zip` files.

## Set env variables

Make sure you have required env variables set in the `.env` file

```
ACP_CLIENT_ID=xxxxx
ACP_CLIENT_SECRET=xxxx
ACP_ISSUER_URL=xxxx
```

If you're using SSO set

```
AWS_PROFILE=xxx
```

You can also set

```
CDK_DEPLOY_ACCOUNT=XXX
CDK_DEPLOY_REGION=XXX
```

if you don't set those variables, `CDK_DEFAULT_ACCOUNT` and `CDK_DEPLOY_REGION` are going to be used.

### Local lambda packages (without S3)

If you have lambda `.zip` packages locally, set

```
LOCAL_LAMBDAS_DIR=xxxxx
```

to the directory where your lambdas `.zip` files are.

## IaC

By default, authorizer will get deployed with automatic authorizer creation.
It will scan all APIs and attach an authorizer to all methods in all APIs.

This is not recommended for production environments, or any environments managed using IaC tools.
To disable this behaviour, set the context parameter `-c manuallyCreateAuthorizer=true`.

This will disable automated authorizer creation.

## Demo API

If you want to deply a demo API connected to the authorizer, pass `-c deployDemo=true` context param to cdk.

You can also use a helper make target `make deploy-with-demo-api`
