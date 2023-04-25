// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package service provides methods for the etcd snapshot service.
package service

import (
	"context"
	"fmt"

	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	talosconfig "github.com/siderolabs/talos/pkg/machinery/client/config"

	"github.com/siderolabs/talos-backup/pkg/config"
	"github.com/siderolabs/talos-backup/pkg/encryption"
	"github.com/siderolabs/talos-backup/pkg/s3"
	"github.com/siderolabs/talos-backup/pkg/talos"
	"github.com/siderolabs/talos-backup/pkg/util"
)

// BackupEncryptedSnapshot takes a snapshot of etcd, encrypts it and uploads it to S3.
func BackupEncryptedSnapshot(ctx context.Context, serviceConfig *config.ServiceConfig, talosConfig *talosconfig.Config, talosClient *talosclient.Client) error {
	clusterName := serviceConfig.ClusterName
	if clusterName == "" {
		clusterName = talosConfig.Context
	}

	snapshotPath, err := talos.TakeEtcdSnapshot(ctx, talosClient, clusterName)
	if err != nil {
		return fmt.Errorf("failed to take etcd snapshot: %w", err)
	}

	defer util.CleanupFile(snapshotPath)

	encryptedFileName, err := encryption.EncryptFile(snapshotPath, serviceConfig.AgeX25519PublicKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt etcd snapshot: %w", err)
	}

	defer util.CleanupFile(encryptedFileName)

	client, err := s3.CreateClientWithCustomEndpoint(ctx, serviceConfig.CustomS3Endpoint)
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %w", err)
	}

	s3Info := config.S3Info{
		Bucket: serviceConfig.Bucket,
	}

	s3Prefix := serviceConfig.S3Prefix
	if s3Prefix == "" {
		s3Prefix = clusterName
	}

	err = s3.PushSnapshot(ctx, s3Info, client, s3Prefix, encryptedFileName)
	if err != nil {
		return fmt.Errorf("failed to push encrypted snapshot: %w", err)
	}

	return nil
}
