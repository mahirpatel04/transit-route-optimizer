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
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslocation"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsrds"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

const (
	dbMasterUsername = "transit"
	dbName           = "transit_optimizer"
	placeIndexName   = "TransitRouteOptimizerPlaceIndex"
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

	// Default VPC, same region as the GTFS bucket (us-east-1) so the S3
	// gateway endpoint below applies.
	vpc := awsec2.Vpc_FromLookup(stack, jsii.String("DefaultVpc"), &awsec2.VpcLookupOptions{
		IsDefault: jsii.Bool(true),
	})

	// Restrict to the AZs Aurora already uses — no need for all 6 default-VPC
	// AZs, and fewer AZs means fewer interface endpoint ENIs to pay for.
	activeAZs := []*string{jsii.String("us-east-1a"), jsii.String("us-east-1c"), jsii.String("us-east-1d")}

	// Free, private route to S3 — no NAT Gateway/Instance needed.
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

	// Optional: DEV_IP lets a developer's laptop psql in directly.
	if devIP := os.Getenv("DEV_IP"); devIP != "" {
		auroraSecurityGroup.AddIngressRule(
			awsec2.Peer_Ipv4(jsii.String(devIP+"/32")),
			awsec2.Port_Tcp(jsii.Number(5432)),
			jsii.String("Allow a developer laptop to reach Aurora"),
			jsii.Bool(false),
		)
	}

	auroraSecurityGroup.AddIngressRule(
		awsec2.Peer_SecurityGroupId(lambdaSecurityGroup.SecurityGroupId(), nil),
		awsec2.Port_Tcp(jsii.Number(5432)),
		jsii.String("Allow the ingest Lambda to reach Aurora"),
		jsii.Bool(false),
	)

	// Publicly accessible; the security group above is what actually
	// restricts who can connect, not subnet placement.
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

	// Esri is the cheaper of the two available data providers and has solid
	// NYC coverage — no need for HERE here.
	placeIndex := awslocation.NewCfnPlaceIndex(stack, jsii.String("PlaceIndex"), &awslocation.CfnPlaceIndexProps{
		IndexName:  jsii.String(placeIndexName),
		DataSource: jsii.String("Esri"),
	})

	// Interface VPC endpoint for Location Service Places — reachable entirely
	// inside the VPC, no NAT/internet path needed.
	locationEndpointSecurityGroup := awsec2.NewSecurityGroup(stack, jsii.String("LocationEndpointSecurityGroup"), &awsec2.SecurityGroupProps{
		Vpc:              vpc,
		AllowAllOutbound: jsii.Bool(true),
		Description:      jsii.String("Security group for the Location Service Places VPC endpoint"),
	})
	locationEndpointSecurityGroup.AddIngressRule(
		awsec2.Peer_SecurityGroupId(lambdaSecurityGroup.SecurityGroupId(), nil),
		awsec2.Port_Tcp(jsii.Number(443)),
		jsii.String("Allow the ingest Lambda to reach the Location Service Places endpoint"),
		jsii.Bool(false),
	)
	vpc.AddInterfaceEndpoint(jsii.String("LocationPlacesEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service:        awsec2.InterfaceVpcEndpointAwsService_LOCATION_SERVICE_PLACES(),
		Subnets:        &awsec2.SubnetSelection{SubnetType: awsec2.SubnetType_PUBLIC, AvailabilityZones: &activeAZs},
		SecurityGroups: &[]awsec2.ISecurityGroup{locationEndpointSecurityGroup},
	})

	// ClusterEndpoint().Hostname() is a CDK token; CDK resolves the embedded
	// token into a CloudFormation Fn::Join automatically.
	databaseURL := fmt.Sprintf(
		"postgres://%s:%s@%s:5432/%s?sslmode=require",
		url.QueryEscape(dbMasterUsername), url.QueryEscape(dbMasterPassword), *cluster.ClusterEndpoint().Hostname(), dbName,
	)

	ingestFunction := awslambda.NewDockerImageFunction(stack, jsii.String("IngestFunction"), &awslambda.DockerImageFunctionProps{
		Code: awslambda.DockerImageCode_FromImageAsset(jsii.String(".."), &awslambda.AssetImageCodeProps{
			File: jsii.String("dockerfile.lambda"),
		}),
		Vpc:            vpc,
		VpcSubnets:     &awsec2.SubnetSelection{SubnetType: awsec2.SubnetType_PUBLIC, AvailabilityZones: &activeAZs},
		SecurityGroups: &[]awsec2.ISecurityGroup{lambdaSecurityGroup},
		// No internet needed (S3 gateway + Location Service interface endpoint);
		// CDK's default safety check doesn't know that, so this is explicit.
		AllowPublicSubnet: jsii.Bool(true),
		Environment: &map[string]*string{
			"DATABASE_URL":     jsii.String(databaseURL),
			"PLACE_INDEX_NAME": jsii.String(placeIndexName),
		},
		Timeout:    awscdk.Duration_Seconds(jsii.Number(60)),
		MemorySize: jsii.Number(512),
		// No ReservedConcurrentExecutions: account concurrency limit is 10,
		// and AWS requires 10 stay unreserved — revisit if that quota rises.
	})

	ingestFunction.Role().AddToPrincipalPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions:   &[]*string{jsii.String("geo:SearchPlaceIndexForText")},
		Resources: &[]*string{placeIndex.AttrArn()},
	}))

	fnUrl := ingestFunction.AddFunctionUrl(&awslambda.FunctionUrlOptions{
		AuthType: awslambda.FunctionUrlAuthType_NONE,
		Cors: &awslambda.FunctionUrlCorsOptions{
			AllowedOrigins: &[]*string{jsii.String("https://mahirpatel04.github.io")},
			AllowedMethods: &[]awslambda.HttpMethod{awslambda.HttpMethod_GET},
		},
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

	awscdk.NewCfnOutput(stack, jsii.String("ApiUrl"), &awscdk.CfnOutputProps{
		Value: fnUrl.Url(),
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
