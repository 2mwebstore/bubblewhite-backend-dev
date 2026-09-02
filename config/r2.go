package config

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// R2Client is the shared S3 client pointed at the Cloudflare R2 endpoint.
var R2Client *s3.Client

// ConnectR2 builds an S3 client against R2's S3-compatible API. R2 has no
// concept of AWS "regions", so we pass a static placeholder and rely on the
// custom endpoint + path-style addressing.
func ConnectR2(cfg *Config) *s3.Client {
	if cfg.R2AccessKeyID == "" || cfg.R2SecretAccessKey == "" || cfg.R2Endpoint == "" {
		log.Println("config: R2 credentials not set — image uploads will fail until configured")
	}
	if cfg.R2PublicURL == "" {
		log.Println("config: R2_PUBLIC_URL not set — uploaded image URLs will be relative (missing scheme+host), which breaks anything that needs to parse them as absolute URLs (e.g. deriving the object key to delete an image later). Set R2_PUBLIC_URL in .env.")
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion("auto"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.R2AccessKeyID, cfg.R2SecretAccessKey, "",
		)),
	)
	if err != nil {
		log.Fatalf("config: failed to build R2 client config: %v", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.R2Endpoint)
		o.UsePathStyle = true
	})

	R2Client = client
	log.Println("config: R2 client ready, bucket:", cfg.R2Bucket)
	return client
}
