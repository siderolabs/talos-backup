// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package s3 provides functions for pushing a file to s3
package s3

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	buconfig "github.com/siderolabs/talos-backup/pkg/config"
)

// CreateClientWithCustomEndpoint returns an S3 client that loads the default AWS configuration.
// You may optionally specify `customS3Endpoint` for a custom S3 API endpoint.
func CreateClientWithCustomEndpoint(ctx context.Context, svcConf *buconfig.ServiceConfig) (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(svcConf.Region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if svcConf.CustomS3Endpoint == "" {
			return
		}

		o.BaseEndpoint = aws.String(svcConf.CustomS3Endpoint)
		o.UsePathStyle = svcConf.UsePathStyle
		// Many S3-compatible providers reject the flexible checksums the SDK sends by default,
		// only send them when the operation requires it.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})

	log.Printf("S3 client created for endpoint: %q, region: %s, usePathStyle: %v",
		svcConf.CustomS3Endpoint, cfg.Region, svcConf.UsePathStyle)

	return client, nil
}

// PushSnapshot will push the given file into s3.
func PushSnapshot(ctx context.Context, conf buconfig.S3Info, s3c *s3.Client, s3Prefix, snapPath string) error {
	f, err := os.Open(snapPath)
	if err != nil {
		return err
	}

	closeOnce := sync.OnceValue(f.Close)
	defer closeOnce() //nolint:errcheck

	objectKey := fmt.Sprintf("%s/%s", s3Prefix, snapPath)

	log.Printf("Uploading %s to bucket %s with key %s", snapPath, conf.Bucket, objectKey)

	// transfermanager does not inherit the client checksum setting, it has to be passed along
	uploader := transfermanager.New(s3c, func(o *transfermanager.Options) {
		o.RequestChecksumCalculation = s3c.Options().RequestChecksumCalculation
	})

	_, err = uploader.UploadObject(ctx, &transfermanager.UploadObjectInput{
		Bucket:      aws.String(conf.Bucket),
		Key:         aws.String(objectKey),
		Body:        f,
		ContentType: aws.String("application/octet-stream"),
	})
	if err != nil {
		return fmt.Errorf("failed to upload %q snapshot to s3: %w", snapPath, err)
	}

	if err = closeOnce(); err != nil {
		return fmt.Errorf("failed to close snapshot file %q: %w", snapPath, err)
	}

	return nil
}
