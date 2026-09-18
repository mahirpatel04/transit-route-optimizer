# Infrastructure (AWS CDK, Go)

Defines the whole stack in one CDK app (`infra.go`): the default VPC lookup, an S3 gateway endpoint and a Location Service Places interface endpoint (so the Lambda needs no internet access at all), Aurora Serverless v2, the Lambda (ingest + API, container image), an EventBridge weekly cron rule, and an AWS Location Service place index for geocoding. See [`../docs/ARCHITECTURE.md`](../docs/ARCHITECTURE.md) for how the pieces fit together and why.

## Deploying

```bash
export DB_MASTER_PASSWORD='...'                          # Aurora master password
export DEV_IP=$(curl -s https://checkip.amazonaws.com)   # optional: lets your laptop psql into Aurora directly
cdk deploy
```

## Useful commands

 * `cdk deploy`      deploy this stack to your default AWS account/region
 * `cdk diff`        compare deployed stack with current state
 * `cdk synth`       emits the synthesized CloudFormation template
 * `go test`         run unit tests
