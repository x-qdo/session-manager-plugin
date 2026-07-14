// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

//go:build darwin || freebsd || linux || netbsd || openbsd
// +build darwin freebsd linux netbsd openbsd

// Package shellsession starts shell session.
package shellsession

import (
	"bytes"
	"os"
	"os/exec"

	"github.com/aws/session-manager-plugin/src/log"
)

// disableEchoAndInputBuffering disables echo to avoid double echo and disable input buffering
func (s *ShellSession) disableEchoAndInputBuffering() {
	if getState(&s.originalSttyState) != nil {
		return
	}
	s.terminalConfigured = true
	if setState(bytes.NewBufferString("cbreak")) != nil {
		return
	}
	if setState(bytes.NewBufferString("-echo")) != nil {
		return
	}
}

// getState gets current state of terminal
func getState(state *bytes.Buffer) error {
	cmd := exec.Command("stty", "-g")
	cmd.Stdin = os.Stdin
	cmd.Stdout = state
	return cmd.Run()
}

// setState sets the new settings to terminal
func setState(state *bytes.Buffer) error {
	cmd := exec.Command("stty", state.String())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

// stop restores the terminal settings and exits
func (s *ShellSession) Stop() {
	if s.terminalConfigured {
		setState(&s.originalSttyState)
		setState(bytes.NewBufferString("echo")) // for linux and ubuntu
	}
	if !s.EmbeddedMode {
		os.Exit(0)
	}
}

func (s *ShellSession) handleKeyboardInput(log log.T) (err error) {
	input := GetInput()
	if input == nil {
		s.disableEchoAndInputBuffering()
		input = os.Stdin
	}
	return s.handleInput(log, input)
}
