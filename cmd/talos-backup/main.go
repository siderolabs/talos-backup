// Package main provides the command line interface for the talos-backup tool.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/adhocore/gronx"

	"github.com/siderolabs/talos-backup/pkg/config"
	"github.com/siderolabs/talos-backup/pkg/s3"
	"github.com/siderolabs/talos-backup/pkg/talos"
)

func main() {
	ctx := context.Background()

	snapList, err := config.ParseConfig(ctx, "./config.yaml")
	if err != nil {
		log.Fatal(err)
	}

	ticker := time.NewTicker(60 * time.Second)

	for range ticker.C {
		errs := make(chan error, len(snapList.Snapshots))

		for _, snapItem := range snapList.Snapshots {
			go processSnap(ctx, snapItem, errs)
		}

		for range snapList.Snapshots {
			err := <-errs
			if err != nil {
				log.Printf("%v", err)
			}
		}

		close(errs)
	}
}

func processSnap(ctx context.Context, snapItem config.Snapshot, errChan chan error) {
	gron := gronx.New()

	isDue, err := gron.IsDue(snapItem.Schedule)
	if err != nil {
		errChan <- fmt.Errorf("failed validating schedule for cluster %q: %w", snapItem.ClusterName, err)

		return
	}

	if isDue {
		log.Printf("cluster %q due for snapshotting", snapItem.ClusterName)

		err = snapIt(ctx, snapItem)
		if err != nil {
			errChan <- err

			return
		}
	}
	errChan <- nil
}

func snapIt(ctx context.Context, snapItem config.Snapshot) error {
	tc, err := talos.CreateClient(ctx, snapItem.TalosInfo.Secret)
	if err != nil {
		return fmt.Errorf("failed creating talos client for cluster %q: %w", snapItem.ClusterName, err)
	}

	log.Printf("taking etcd snapshot for cluster %q", snapItem.ClusterName)

	snapPath, err := talos.TakeEtcdSnapshot(ctx, tc, snapItem.ClusterName)
	if err != nil {
		return fmt.Errorf("failed taking snapshot for cluster %q: %w", snapItem.ClusterName, err)
	}

	s3c, err := s3.CreateClient(ctx, snapItem.S3Info)
	if err != nil {
		return fmt.Errorf("failed creating s3 client for cluster %q: %w", snapItem.ClusterName, err)
	}

	log.Printf("pushing snapshot %q for cluster %q to s3 bucket %q", snapPath, snapItem.ClusterName, snapItem.S3Info.Bucket)

	err = s3.PushSnapshot(ctx, snapItem.S3Info, s3c, snapItem.ClusterName, snapPath)
	if err != nil {
		return fmt.Errorf("failed pushing snapshot to s3 for cluster %q: %w", snapItem.ClusterName, err)
	}

	// Only after pushing will we delete the local copy. this might cause heartburn if we can never push for some reason :shrug:
	log.Printf("cleaning up pushed snapshot %q for cluster %q", snapPath, snapItem.ClusterName)

	err = os.Remove(snapPath)
	if err != nil {
		return fmt.Errorf("failed cleaning up pushed snapshot to s3 for cluster %q: %w", snapItem.ClusterName, err)
	}

	return nil
}
