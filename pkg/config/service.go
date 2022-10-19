// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"os"
)

// ServiceConfig holds configuration values for the etcd snapshot service.
// The parameters CustomS3Endpoint, s3Prefix and clusterName are optional.
type ServiceConfig struct {
	CustomS3Endpoint   string `yaml:"customS3Endpoint"`
	Bucket             string `yaml:"bucket"`
	S3Prefix           string `yaml:"s3Prefix"`
	ClusterName        string `yaml:"clusterName"`
	AgeX25519PublicKey string `yaml:"ageX25519PublicKey"`
}

const (
	customS3EndpointEnvVar   = "CUSTOM_S3_ENDPOINT"
	bucketEnvVar             = "BUCKET"
	s3PrefixEnvVar           = "S3_PREFIX"
	clusterNameEnvVar        = "CLUSTER_NAME"
	ageX25519PublicKeyEnvVar = "AGE_X25519_PUBLIC_KEY"
)

// GetServiceConfig parses the backup service config at path.
func GetServiceConfig() *ServiceConfig {
	return &ServiceConfig{
		CustomS3Endpoint:   os.Getenv(customS3EndpointEnvVar),
		Bucket:             os.Getenv(bucketEnvVar),
		S3Prefix:           os.Getenv(s3PrefixEnvVar),
		ClusterName:        os.Getenv(clusterNameEnvVar),
		AgeX25519PublicKey: os.Getenv(ageX25519PublicKeyEnvVar),
	}
}
