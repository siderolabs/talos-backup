// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package main provides the command line interface for talos-backup.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	talosconfig "github.com/siderolabs/talos/pkg/machinery/client/config"

	"github.com/siderolabs/talos-backup/cmd/talos-backup/service"
	"github.com/siderolabs/talos-backup/pkg/config"
)

func run() error {
	ctx := context.Background()

	serviceConfig := config.GetServiceConfig()

	talosConfig, err := talosconfig.Open("")
	if err != nil {
		return fmt.Errorf("failed to get talosconfig: %w", err)
	}

	talosClient, err := talosclient.New(ctx, talosclient.WithConfig(talosConfig))
	if err != nil {
		return fmt.Errorf("failed to create talos client: %w", err)
	}

	return service.BackupEncryptedSnapshot(ctx, serviceConfig, talosConfig, talosClient)
}

func main() {
	if err := run(); err != nil {
		log.Println(err)

		os.Exit(-1)
	}
}
