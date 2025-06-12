// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package compression provides methods for compression of etcd snapshot.
package compression

import (
	"fmt"
	"io"
	"os"

	"github.com/klauspost/compress/zstd"

	"github.com/siderolabs/talos-backup/pkg/util"
)

// CompressFile compresses the file at fileToCompressPath and returns the name of the compressed file.
func CompressFile(fileToCompressPath string) (string, error) {
	compressedFileName, err := compressFile(fileToCompressPath)

	if err != nil && compressedFileName != "" {
		util.CleanupFile(compressedFileName)
	}

	return compressedFileName, err
}

// Compress input to output.
func compressFile(fileToCompressPath string) (string, error) {
	fileToCompress, err := os.Open(fileToCompressPath)
	if err != nil {
		return "", fmt.Errorf("failed to open file for Compression %q: %w", fileToCompressPath, err)
	}

	defer fileToCompress.Close() //nolint:errcheck

	compressedFileName := fileToCompressPath + ".zst"

	compressedFile, err := os.OpenFile(compressedFileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("failed to allocate compressed file %q: %w", compressedFileName, err)
	}

	defer compressedFile.Close() //nolint:errcheck

	encoder, err := zstd.NewWriter(compressedFile)
	if err != nil {
		return "", err
	}

	defer encoder.Close() //nolint:errcheck

	if _, err := io.Copy(encoder, fileToCompress); err != nil {
		return "", fmt.Errorf("failed to write compressed file %q: %w", compressedFileName, err)
	}

	if err := encoder.Close(); err != nil {
		return "", fmt.Errorf("failed to close writer: %w", err)
	}

	if err := compressedFile.Sync(); err != nil {
		return "", fmt.Errorf("failed to sync compressed file to disk: %w", err)
	}

	return compressedFileName, nil
}
