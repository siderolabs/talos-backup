// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package talos provides functions for connecting to and
// taking snapshots from a given talos cluster
package talos

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	talosconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
)

// CreateClient returns a talos API client given a talosconfig string.
func CreateClient(ctx context.Context, talosSecret string) (*talosclient.Client, error) {
	decodedSecret, err := base64.StdEncoding.DecodeString(talosSecret)
	if err != nil {
		return nil, fmt.Errorf("failed decoding talosSecret: %w", err)
	}

	t, err := talosconfig.FromBytes(decodedSecret)
	if err != nil {
		return nil, fmt.Errorf("failed creating config from bytes: %w", err)
	}

	return talosclient.New(ctx, talosclient.WithConfig(t))
}

// TakeEtcdSnapshot will take an etcd snapshot given a talos client
// and save/validate it locally.
func TakeEtcdSnapshot(ctx context.Context, tc *talosclient.Client, clusterName string) (string, error) {
	timeStamp := time.Now()

	dbPath := fmt.Sprintf("%s-%s.snap", clusterName, timeStamp.Format(time.RFC3339))
	partPath := dbPath + ".part"

	defer os.RemoveAll(partPath) //nolint:errcheck

	dest, err := os.OpenFile(partPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("error creating temp file: %w", err)
	}

	defer dest.Close() //nolint:errcheck

	r, err := tc.EtcdSnapshot(ctx, &machine.EtcdSnapshotRequest{})
	if err != nil {
		return "", fmt.Errorf("error taking snapshot: %w", err)
	}

	defer r.Close() //nolint:errcheck

	size, err := io.Copy(dest, r)
	if err != nil {
		return "", fmt.Errorf("error reading: %w", err)
	}

	if err = dest.Sync(); err != nil {
		return "", fmt.Errorf("error fsyncing: %w", err)
	}

	// TODO: probably need to clean up the snap if there's an issue w/ it
	// this check is from https://github.com/etcd-io/etcd/blob/client/v3.5.0-alpha.0/client/v3/snapshot/v3_snapshot.go#L46
	if (size % 512) != sha256.Size {
		return "", fmt.Errorf("sha256 checksum not found (size %d)", size)
	}

	if err = os.Rename(partPath, dbPath); err != nil {
		return "", fmt.Errorf("sha256 checksum not found (size %d)", size)
	}

	log.Printf("etcd snapshot for cluster %q saved to %q (%d bytes)\n", clusterName, dbPath, size)

	return dbPath, nil
}
