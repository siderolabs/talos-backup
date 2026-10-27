// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package s3_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	buconfig "github.com/siderolabs/talos-backup/pkg/config"
	"github.com/siderolabs/talos-backup/pkg/s3"
)

func TestPushSnapshotCustomEndpoint(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Chdir(t.TempDir())

	require.NoError(t, os.WriteFile("snap.db", []byte("snapshot"), 0o600))

	var req *http.Request

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		req = r
	}))
	t.Cleanup(srv.Close)

	client, err := s3.CreateClientWithCustomEndpoint(t.Context(), &buconfig.ServiceConfig{
		CustomS3Endpoint: srv.URL,
		Region:           "us-east-1",
		UsePathStyle:     true,
	})
	require.NoError(t, err)

	require.NoError(t, s3.PushSnapshot(t.Context(), buconfig.S3Info{Bucket: "bucket"}, client, "prefix", "snap.db"))

	require.Equal(t, "/bucket/prefix/snap.db", req.URL.Path)

	// S3-compatible providers often reject the SDK default flexible checksums
	for name := range req.Header {
		require.False(t, strings.HasPrefix(strings.ToLower(name), "x-amz-checksum-"), "unexpected checksum header %s", name)
	}

	require.NotContains(t, req.Header.Get("X-Amz-Content-Sha256"), "TRAILER")
}
