// Package s3 provides functions for pushing a file to s3
package s3

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	buconfig "github.com/siderolabs/talos-backup/pkg/config"
)

// CreateClient will return an s3 client for use.
func CreateClient(ctx context.Context, conf buconfig.S3Info) (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	cfg.Region = conf.Region

	s3c := s3.NewFromConfig(cfg) //nolint:contextcheck

	return s3c, nil
}

// PushSnapshot will push the given file into s3.
func PushSnapshot(ctx context.Context, conf buconfig.S3Info, s3c *s3.Client, clusterName, snapPath string) error {
	f, err := os.Open(snapPath)
	if err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck

	s3In := &s3.PutObjectInput{
		Bucket: aws.String(conf.Bucket),
		Key:    aws.String(fmt.Sprintf("%s/%s", clusterName, snapPath)),
		Body:   f,
	}

	_, err = s3c.PutObject(ctx, s3In)
	if err != nil {
		return err
	}

	return nil
}
