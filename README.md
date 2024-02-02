# Cloudentity AWS Authorizer CDK

This project provides CDK deployment for Cloudentity AWS Authorizer.

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
