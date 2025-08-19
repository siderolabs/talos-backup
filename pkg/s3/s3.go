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
	"strings"
	"sync"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	buconfig "github.com/siderolabs/talos-backup/pkg/config"
)

// CreateClientWithCustomEndpoint returns an S3 minio client that loads the default AWS configuration.
// You may optionally specify `customS3Endpoint` for a custom S3 API endpoint.
func CreateClientWithCustomEndpoint(ctx context.Context, svcConf *buconfig.ServiceConfig) (*minio.Client, error) {
	endpoint := svcConf.CustomS3Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("s3.%s.amazonaws.com", svcConf.Region)
	}

	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")

	useSSL := !strings.HasPrefix(svcConf.CustomS3Endpoint, "http://")

	creds := credentials.NewChainCredentials(
		[]credentials.Provider{
			&credentials.EnvAWS{},
			&credentials.IAM{},
			&credentials.FileAWSCredentials{},
		},
	)

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  creds,
		Secure: useSSL,
		Region: svcConf.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load S3 configuration: %w", err)
	}

	log.Printf("S3 client created for endpoint: %s, region: %s, useSSL: %v",
		endpoint, svcConf.Region, useSSL)

	return client, nil
}

// PushSnapshot will push the given file into s3.
func PushSnapshot(ctx context.Context, conf buconfig.S3Info, s3c *minio.Client, s3Prefix, snapPath string) error {
	f, err := os.Open(snapPath)
	if err != nil {
		return err
	}

	closeOnce := sync.OnceValue(f.Close)
	defer closeOnce() //nolint:errcheck

	fileInfo, err := f.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	objectKey := fmt.Sprintf("%s/%s", s3Prefix, snapPath)

	log.Printf("Uploading %s (size: %d bytes) to bucket %s with key %s",
		snapPath, fileInfo.Size(), conf.Bucket, objectKey)

	_, err = s3c.PutObject(ctx, conf.Bucket, objectKey, f, fileInfo.Size(), minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	if err != nil {
		return fmt.Errorf("failed to upload %q snapshot to s3: %w", snapPath, err)
	}

	if err = closeOnce(); err != nil {
		return fmt.Errorf("failed to close snapshot file %q: %w", snapPath, err)
	}

	return nil
}
