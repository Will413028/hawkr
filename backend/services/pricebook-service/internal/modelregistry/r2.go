package modelregistry

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// R2Downloader fetches objects from a Cloudflare R2 bucket via R2's
// S3-compatible API. Implements Downloader (fetch.go).
type R2Downloader struct {
	client   *s3.Client
	bucket   string
	endpoint string
}

// NewR2Downloader builds a client scoped to one R2 account + bucket. Region
// is fixed to "auto" — R2 doesn't have real AWS regions, this is the value
// R2's own docs and this repo's existing Java R2StorageService both use.
func NewR2Downloader(ctx context.Context, accountID, accessKeyID, secretAccessKey, bucket string) (*R2Downloader, error) {
	if accountID == "" {
		return nil, fmt.Errorf("modelregistry: R2 accountID must not be empty")
	}
	if accessKeyID == "" {
		return nil, fmt.Errorf("modelregistry: R2 accessKeyID must not be empty")
	}
	if secretAccessKey == "" {
		return nil, fmt.Errorf("modelregistry: R2 secretAccessKey must not be empty")
	}
	if bucket == "" {
		return nil, fmt.Errorf("modelregistry: R2 bucket must not be empty")
	}

	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)

	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("auto"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("modelregistry: loading AWS config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})

	return &R2Downloader{client: client, bucket: bucket, endpoint: endpoint}, nil
}

// Download implements Downloader.
func (d *R2Downloader) Download(ctx context.Context, objectKey string) ([]byte, error) {
	out, err := d.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(d.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return nil, fmt.Errorf("modelregistry: R2 GetObject(%s/%s): %w", d.bucket, objectKey, err)
	}
	defer func() { _ = out.Body.Close() }()

	body, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("modelregistry: reading R2 object body %s/%s: %w", d.bucket, objectKey, err)
	}
	return body, nil
}
