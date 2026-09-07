package main

import (
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsevents"
	"github.com/aws/aws-cdk-go/awscdk/v2/awseventstargets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsrds"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

const (
	dbMasterUsername = "transit"
	dbName           = "transit_optimizer"
)

type InfraStackProps struct {
	awscdk.StackProps
}

func NewInfraStack(scope constructs.Construct, id string, props *InfraStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	dbMasterPassword := os.Getenv("DB_MASTER_PASSWORD")
	if dbMasterPassword == "" {
		log.Fatal("DB_MASTER_PASSWORD env variable not set — export it before running cdk commands")
	}

	// The account's default VPC, in whatever region this stack deploys to
	// (see env() below — everything now lives in us-east-1, same region as
	// the public GTFS bucket, so the S3 gateway endpoint actually applies).
	vpc := awsec2.Vpc_FromLookup(stack, jsii.String("DefaultVpc"), &awsec2.VpcLookupOptions{
		IsDefault: jsii.Bool(true),
	})

	// S3 gateway endpoint: free, private route to S3 so the VPC-attached Lambda
	// can fetch the GTFS zip without a NAT Gateway/Instance. Only works because
	// the bucket and this VPC are now in the same region (us-east-1).
	vpc.AddGatewayEndpoint(jsii.String("S3Endpoint"), &awsec2.GatewayVpcEndpointOptions{
		Service: awsec2.GatewayVpcEndpointAwsService_S3(),
	})

	auroraSecurityGroup := awsec2.NewSecurityGroup(stack, jsii.String("AuroraSecurityGroup"), &awsec2.SecurityGroupProps{
		Vpc:              vpc,
		AllowAllOutbound: jsii.Bool(true),
		Description:      jsii.String("Security group for the Aurora cluster"),
	})

	lambdaSecurityGroup := awsec2.NewSecurityGroup(stack, jsii.String("IngestLambdaSecurityGroup"), &awsec2.SecurityGroupProps{
		Vpc:              vpc,
		AllowAllOutbound: jsii.Bool(true),
		Description:      jsii.String("Security group for the GTFS ingest Lambda"),
	})

	// Optional: allow a developer's laptop to psql in directly. Set to your
	// current public IP (e.g. `curl -s https://checkip.amazonaws.com`) before
	// deploying — it changes across networks, so this isn't hardcoded. Without
	// it, only the ingest Lambda can reach Aurora.
	if devIP := os.Getenv("DEV_IP"); devIP != "" {
		auroraSecurityGroup.AddIngressRule(
			awsec2.Peer_Ipv4(jsii.String(devIP+"/32")),
			awsec2.Port_Tcp(jsii.Number(5432)),
			jsii.String("Allow a developer's laptop to reach Aurora"),
			jsii.Bool(false),
		)
	}

	auroraSecurityGroup.AddIngressRule(
		awsec2.Peer_SecurityGroupId(lambdaSecurityGroup.SecurityGroupId(), nil),
		awsec2.Port_Tcp(jsii.Number(5432)),
		jsii.String("Allow the ingest Lambda to reach Aurora"),
		jsii.Bool(false),
	)

	// Publicly accessible so a human can still psql in from their laptop, same
	// as the hand-created cluster from Phase 2 — the security group above is
	// what actually restricts who can connect, not subnet placement.
	cluster := awsrds.NewDatabaseCluster(stack, jsii.String("AuroraCluster"), &awsrds.DatabaseClusterProps{
		Engine: awsrds.DatabaseClusterEngine_AuroraPostgres(&awsrds.AuroraPostgresClusterEngineProps{
			Version: awsrds.AuroraPostgresEngineVersion_VER_17_7(),
		}),
		Credentials:             awsrds.Credentials_FromPassword(jsii.String(dbMasterUsername), awscdk.SecretValue_UnsafePlainText(jsii.String(dbMasterPassword))),
		DefaultDatabaseName:     jsii.String(dbName),
		Vpc:                     vpc,
		VpcSubnets:              &awsec2.SubnetSelection{SubnetType: awsec2.SubnetType_PUBLIC},
		SecurityGroups:          &[]awsec2.ISecurityGroup{auroraSecurityGroup},
		ServerlessV2MinCapacity: jsii.Number(0),
		ServerlessV2MaxCapacity: jsii.Number(1),
		Writer: awsrds.ClusterInstance_ServerlessV2(jsii.String("Writer"), &awsrds.ServerlessV2ClusterInstanceProps{
			PubliclyAccessible: jsii.Bool(true),
		}),
	})

	// Built via string concatenation with a CDK token (ClusterEndpoint().Hostname()
	// isn't known until deploy time) — CDK detects the embedded token and resolves
	// it into a CloudFormation Fn::Join automatically when this is used as a prop.
	databaseURL := fmt.Sprintf(
		"postgres://%s:%s@%s:5432/%s?sslmode=require",
		url.QueryEscape(dbMasterUsername), url.QueryEscape(dbMasterPassword), *cluster.ClusterEndpoint().Hostname(), dbName,
	)

	ingestFunction := awslambda.NewDockerImageFunction(stack, jsii.String("IngestFunction"), &awslambda.DockerImageFunctionProps{
		Code: awslambda.DockerImageCode_FromImageAsset(jsii.String(".."), &awslambda.AssetImageCodeProps{
			File: jsii.String("dockerfile.lambda"),
		}),
		Vpc:            vpc,
		VpcSubnets:     &awsec2.SubnetSelection{SubnetType: awsec2.SubnetType_PUBLIC},
		SecurityGroups: &[]awsec2.ISecurityGroup{lambdaSecurityGroup},
		// The only "internet" resource this Lambda needs is the GTFS zip on S3,
		// reachable via the S3 gateway endpoint above without a NAT Gateway/Instance.
		// CDK's default safety check doesn't know that, so this is explicit.
		AllowPublicSubnet: jsii.Bool(true),
		Environment: &map[string]*string{
			"DATABASE_URL": jsii.String(databaseURL),
		},
		Timeout:    awscdk.Duration_Seconds(jsii.Number(60)),
		MemorySize: jsii.Number(512),
	})

	// GTFS static feeds republish on the agency's own cadence (days to weeks),
	// not continuously — weekly keeps this cheap without meaningful staleness.
	schedule := awsevents.NewRule(stack, jsii.String("WeeklyIngestSchedule"), &awsevents.RuleProps{
		Schedule: awsevents.Schedule_Cron(&awsevents.CronOptions{
			WeekDay: jsii.String("MON"),
			Hour:    jsii.String("6"),
			Minute:  jsii.String("0"),
		}),
	})
	schedule.AddTarget(awseventstargets.NewLambdaFunction(ingestFunction, nil))

	awscdk.NewCfnOutput(stack, jsii.String("AuroraEndpoint"), &awscdk.CfnOutputProps{
		Value: cluster.ClusterEndpoint().Hostname(),
	})

	return stack
}

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)

	NewInfraStack(app, "InfraStack", &InfraStackProps{
		awscdk.StackProps{
			Env: env(),
		},
	})

	app.Synth(nil)
}

// env pins the stack to us-east-1 — everything (VPC, Aurora, Lambda) now lives
// there so the free S3 gateway endpoint actually applies (the GTFS bucket is
// in us-east-1; the gateway endpoint trick only works same-region).
func env() *awscdk.Environment {
	return &awscdk.Environment{
		Account: jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT")),
		Region:  jsii.String("us-east-1"),
	}
}
