package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
}

type dbSecret struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
}

func Load(ctx context.Context) (*Config, error) {
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		return loadFromSecretsManager(ctx)
	}

	return loadFromEnv()
}

func loadFromEnv() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		fmt.Printf("godotenv warning: %v\n", err)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL env variable not set")
	}

	return &Config{
		DatabaseURL: databaseURL,
	}, nil
}

func loadFromSecretsManager(ctx context.Context) (*Config, error) {
	secretArn := os.Getenv("DB_SECRET_ARN")
	if secretArn == "" {
		return nil, fmt.Errorf("DB_SECRET_ARN env variable not set")
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := secretsmanager.NewFromConfig(awsCfg)

	out, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: &secretArn,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch secret: %w", err)
	}

	var secret dbSecret
	if err := json.Unmarshal([]byte(*out.SecretString), &secret); err != nil {
		return nil, fmt.Errorf("failed to parse secret: %w", err)
	}

	databaseURL := (&url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(secret.User, secret.Password),
		Host:     fmt.Sprintf("%s:%s", secret.Host, secret.Port),
		Path:     "/" + secret.DBName,
		RawQuery: "sslmode=require",
	}).String()

	return &Config{
		DatabaseURL: databaseURL,
	}, nil
}
