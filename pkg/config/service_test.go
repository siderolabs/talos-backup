// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos-backup/pkg/config"
)

func TestGetServiceConfig(t *testing.T) {
	for _, tc := range []struct {
		name     string
		env      map[string]string
		expected config.ServiceConfig
	}{
		{
			name: "defaults",
			expected: config.ServiceConfig{
				UsePathStyle: true,
			},
		},
		{
			name: "all set",
			env: map[string]string{
				"CUSTOM_S3_ENDPOINT":       "https://s3.example.com",
				"BUCKET":                   "bucket",
				"AWS_REGION":               "eu-west-1",
				"S3_PREFIX":                "prefix",
				"CLUSTER_NAME":             "cluster",
				"USE_PATH_STYLE":           "false",
				"ENABLE_COMPRESSION":       "true",
				"DISABLE_ENCRYPTION":       "true",
				"AGE_RECIPIENT_PUBLIC_KEY": "age1a,age1b",
				"AGE_X25519_PUBLIC_KEY":    "age1c",
			},
			expected: config.ServiceConfig{
				CustomS3Endpoint:      "https://s3.example.com",
				Bucket:                "bucket",
				Region:                "eu-west-1",
				S3Prefix:              "prefix",
				ClusterName:           "cluster",
				UsePathStyle:          false,
				EnableCompression:     true,
				DisableEncryption:     true,
				AgeRecipientPublicKey: []string{"age1a", "age1b", "age1c"},
			},
		},
		{
			name: "only legacy x25519 key",
			env: map[string]string{
				"AGE_X25519_PUBLIC_KEY": "age1c",
			},
			expected: config.ServiceConfig{
				UsePathStyle:          true,
				AgeRecipientPublicKey: []string{"age1c"},
			},
		},
		{
			name: "blank and spaced keys are ignored",
			env: map[string]string{
				"AGE_RECIPIENT_PUBLIC_KEY": "age1a, age1b,,",
				"AGE_X25519_PUBLIC_KEY":    " ",
			},
			expected: config.ServiceConfig{
				UsePathStyle:          true,
				AgeRecipientPublicKey: []string{"age1a", "age1b"},
			},
		},
		{
			name: "booleans only match exact values",
			env: map[string]string{
				"USE_PATH_STYLE":     "False",
				"ENABLE_COMPRESSION": "1",
				"DISABLE_ENCRYPTION": "yes",
			},
			expected: config.ServiceConfig{
				UsePathStyle: true,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, name := range []string{
				"CUSTOM_S3_ENDPOINT", "BUCKET", "AWS_REGION", "S3_PREFIX", "CLUSTER_NAME", "USE_PATH_STYLE",
				"ENABLE_COMPRESSION", "DISABLE_ENCRYPTION", "AGE_RECIPIENT_PUBLIC_KEY", "AGE_X25519_PUBLIC_KEY",
			} {
				t.Setenv(name, tc.env[name])
			}

			actual := config.GetServiceConfig()

			// nil and empty recipient lists are equivalent
			if len(actual.AgeRecipientPublicKey) == 0 {
				actual.AgeRecipientPublicKey = nil
			}

			require.Equal(t, &tc.expected, actual)
		})
	}
}
