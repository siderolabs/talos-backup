// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package config provides functions for parsing the
// backup configs
package config

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
