// Package config provides functions for parsing the
// backer-upper configs
package config

import (
	"context"
	"os"

	"gopkg.in/yaml.v2"
)

// SnapshotList is the struct for highlevel snapshots key.
type SnapshotList struct {
	Snapshots []Snapshot `yaml:"snapshots"`
}

// Snapshot is the struct for a single snapshot item.
type Snapshot struct {
	ClusterName string    `yaml:"clusterName"`
	Schedule    string    `yaml:"schedule"`
	TalosInfo   TalosInfo `yaml:"talosInfo"`
	S3Info      S3Info    `yaml:"s3Info"`
}

// TalosInfo is the struct to hold info on talking to the Talos CP node.
type TalosInfo struct {
	Endpoint string `yaml:"endpoint"`
	Secret   string `yaml:"secret"`
}

// S3Info is the struct to hold info on where to push in s3.
type S3Info struct {
	Bucket string `yaml:"bucket"`
	Region string `yaml:"region"`
}

// ParseConfig will ingest a backer-upper config at the given path.
func ParseConfig(ctx context.Context, path string) (*SnapshotList, error) {
	sl := &SnapshotList{}

	confBytes, err := os.ReadFile(path)
	if err != nil {
		return sl, err
	}

	err = yaml.Unmarshal(confBytes, sl)
	if err != nil {
		return sl, err
	}

	return sl, nil
}
