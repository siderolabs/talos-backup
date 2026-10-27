// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"os"
	"strings"
	"unicode"
)

// ServiceConfig holds configuration values for the etcd snapshot service.
// The parameters CustomS3Endpoint, s3Prefix, clusterName are optional.
type ServiceConfig struct { //nolint:govet
	CustomS3Endpoint      string   `yaml:"customS3Endpoint"`
	Bucket                string   `yaml:"bucket"`
	Region                string   `yaml:"region"`
	S3Prefix              string   `yaml:"s3Prefix"`
	ClusterName           string   `yaml:"clusterName"`
	UsePathStyle          bool     `yaml:"usePathStyle"`
	AgeRecipientPublicKey []string `yaml:"ageRecipientPublicKey"`
	EnableCompression     bool     `yaml:"enableCompression"`
	DisableEncryption     bool     `yaml:"disableEncryption"`
}

const (
	customS3EndpointEnvVar      = "CUSTOM_S3_ENDPOINT"
	bucketEnvVar                = "BUCKET"
	regionEnvVar                = "AWS_REGION"
	s3PrefixEnvVar              = "S3_PREFIX"
	clusterNameEnvVar           = "CLUSTER_NAME"
	usePathStyleEnvVar          = "USE_PATH_STYLE"
	enableCompressionEnvVar     = "ENABLE_COMPRESSION"
	disableEncryptionEnvVar     = "DISABLE_ENCRYPTION"
	ageRecipientPublicKeyEnvVar = "AGE_RECIPIENT_PUBLIC_KEY"
	ageX25519PublicKeyEnvVar    = "AGE_X25519_PUBLIC_KEY"
)

// GetServiceConfig parses the backup service config at path.
func GetServiceConfig() *ServiceConfig {
	ageRecipientPublicKey := splitKeys(os.Getenv(ageRecipientPublicKeyEnvVar))
	ageRecipientPublicKey = append(ageRecipientPublicKey, splitKeys(os.Getenv(ageX25519PublicKeyEnvVar))...)

	return &ServiceConfig{
		CustomS3Endpoint:      os.Getenv(customS3EndpointEnvVar),
		Bucket:                os.Getenv(bucketEnvVar),
		Region:                os.Getenv(regionEnvVar),
		S3Prefix:              os.Getenv(s3PrefixEnvVar),
		ClusterName:           os.Getenv(clusterNameEnvVar),
		UsePathStyle:          os.Getenv(usePathStyleEnvVar) != "false",     //nolint:goconst
		EnableCompression:     os.Getenv(enableCompressionEnvVar) == "true", //nolint:goconst
		DisableEncryption:     os.Getenv(disableEncryptionEnvVar) == "true", //nolint:goconst
		AgeRecipientPublicKey: ageRecipientPublicKey,
	}
}

// splitKeys splits a comma separated list of keys, ignoring blanks.
func splitKeys(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == ',' || unicode.IsSpace(r) })
}
