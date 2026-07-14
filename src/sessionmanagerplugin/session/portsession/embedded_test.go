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

package portsession

import (
	"os"
	"testing"

	"github.com/aws/session-manager-plugin/src/sessionmanagerplugin/session"
)

func TestPortForwardingStopsReturnInEmbeddedMode(t *testing.T) {
	originalExit := exitFunc
	exitCalled := false
	exitFunc = func(int) { exitCalled = true }
	defer func() { exitFunc = originalExit }()

	embeddedSession := session.Session{EmbeddedMode: true}
	(&BasicPortForwarding{session: embeddedSession}).Stop()
	(&MuxPortForwarding{session: embeddedSession}).Stop()

	if exitCalled {
		t.Fatal("embedded port forwarding stop called the process exit hook")
	}
}

func TestPortForwardingDoesNotRegisterSignalsInEmbeddedMode(t *testing.T) {
	originalNotify := notifySignals
	notifyCalled := false
	notifySignals = func(chan<- os.Signal, ...os.Signal) { notifyCalled = true }
	defer func() { notifySignals = originalNotify }()

	embeddedSession := session.Session{EmbeddedMode: true}
	(&BasicPortForwarding{session: embeddedSession}).handleControlSignals(mockLog)
	(&MuxPortForwarding{session: embeddedSession}).handleControlSignals(mockLog)

	if notifyCalled {
		t.Fatal("embedded port forwarding registered a process signal handler")
	}
}

func TestStandardStreamStopLeavesCallerStreamsOpenInEmbeddedMode(t *testing.T) {
	input, inputWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	defer inputWriter.Close()

	outputReader, output, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer outputReader.Close()
	defer output.Close()

	forwarding := &StandardStreamForwarding{
		inputStream:  input,
		outputStream: output,
		session:      session.Session{EmbeddedMode: true},
	}
	forwarding.Stop()

	if _, err := inputWriter.Write([]byte("still open")); err != nil {
		t.Fatalf("embedded stop closed caller-owned input: %v", err)
	}
	if _, err := output.Write([]byte("still open")); err != nil {
		t.Fatalf("embedded stop closed caller-owned output: %v", err)
	}
}
