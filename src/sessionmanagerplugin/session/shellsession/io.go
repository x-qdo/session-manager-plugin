// Copyright 2024 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package shellsession

import (
	"io"
	"sync"
)

var (
	ioMu      sync.RWMutex
	customIn  io.Reader
	customOut io.Writer
)

// ConfigureIO sets custom input/output streams for shell sessions.
// Pass nil to reset to default (os.Stdin/os.Stdout).
func ConfigureIO(in io.Reader, out io.Writer) {
	ioMu.Lock()
	defer ioMu.Unlock()
	customIn = in
	customOut = out
}

// GetInput returns the configured input reader, or nil if using default.
func GetInput() io.Reader {
	ioMu.RLock()
	defer ioMu.RUnlock()
	return customIn
}

// GetOutput returns the configured output writer, or nil if using default.
func GetOutput() io.Writer {
	ioMu.RLock()
	defer ioMu.RUnlock()
	return customOut
}
