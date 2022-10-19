// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package util provides utility methods.
package util

import (
	"log"
	"os"
)

// CleanupFile removes the file at filePath and logs an error if there is one.
func CleanupFile(filePath string) {
	if err := os.Remove(filePath); err != nil {
		log.Printf("error cleaning up file %q: %q", filePath, err)
	}
}
