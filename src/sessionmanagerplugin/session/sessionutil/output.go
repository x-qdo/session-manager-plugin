// Copyright 2026 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the License
// is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package sessionutil

import (
	"io"
	"os"
	"sync"
)

var (
	userOutMu sync.RWMutex
	userOut   io.Writer
)

// SetUserOut sets a custom output writer for shell output.
// Pass nil to restore the platform console output path.
func SetUserOut(w io.Writer) {
	userOutMu.Lock()
	defer userOutMu.Unlock()
	userOut = w
}

// configuredUserOut returns the configured output writer and whether the
// platform console output path has been overridden.
func configuredUserOut() (io.Writer, bool) {
	userOutMu.RLock()
	defer userOutMu.RUnlock()
	return userOut, userOut != nil
}

// getUserOut returns the configured output writer, defaulting to os.Stdout.
func getUserOut() io.Writer {
	if out, ok := configuredUserOut(); ok {
		return out
	}
	return os.Stdout
}
