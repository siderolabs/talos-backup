// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

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

// CreateClientWithCustomEndpoint returns an S3 client that loads the default AWS configuration.
// You may optionally specify `customS3Endpoint` for a custom S3 API endpoint.
func CreateClientWithCustomEndpoint(ctx context.Context, svcConf *buconfig.ServiceConfig) (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(svcConf.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if svcConf.CustomS3Endpoint != "" {
			// Ref: https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/endpoints/
			o.BaseEndpoint = aws.String(svcConf.CustomS3Endpoint)
		}
	})

	return client, nil
}

// PushSnapshot will push the given file into s3.
func PushSnapshot(ctx context.Context, conf buconfig.S3Info, s3c *s3.Client, s3Prefix, snapPath string) error {
	f, err := os.Open(snapPath)
	if err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck

	s3In := &s3.PutObjectInput{
		Bucket: aws.String(conf.Bucket),
		Key:    aws.String(fmt.Sprintf("%s/%s", s3Prefix, snapPath)),
		Body:   f,
	}

	_, err = s3c.PutObject(ctx, s3In)
	if err != nil {
		return err
	}

	return nil
}
