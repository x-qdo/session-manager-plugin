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
	"bytes"
	"testing"

	"github.com/aws/session-manager-plugin/src/log"
	"github.com/aws/session-manager-plugin/src/message"
)

func TestDisplayModeUsesConfiguredOutput(t *testing.T) {
	var output bytes.Buffer
	SetUserOut(&output)
	defer SetUserOut(nil)

	displayMode := NewDisplayMode(log.NewMockLog())
	displayMode.DisplayMessage(log.NewMockLog(), message.ClientMessage{Payload: []byte("hello")})

	if got := output.String(); got != "hello" {
		t.Fatalf("configured output = %q, want %q", got, "hello")
	}
}
