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
	"bufio"
	"bytes"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/aws/session-manager-plugin/src/log"
	"github.com/aws/session-manager-plugin/src/message"
)

// user-configurable IO for shell session. Defaults to stdio for backward compatibility.
var userIn io.Reader = os.Stdin
var userOut io.Writer = os.Stdout

// ConfigureIO allows callers to override the input and output streams used by the
// session-manager shell plugin. Pass nil to use OS stdio defaults.
func ConfigureIO(r io.Reader, w io.Writer) {
	if r != nil {
		userIn = r
	} else {
		userIn = os.Stdin
	}
	if w != nil {
		userOut = w
	} else {
		userOut = os.Stdout
	}
}

// UserIn returns the current configured input stream for the shell session.
// When not overridden, this is os.Stdin.
func UserIn() io.Reader {
	return userIn
}

// UserOut returns the current configured output stream for the shell session.
// When not overridden, this is os.Stdout.
func UserOut() io.Writer {
	return userOut
}

// disableEchoAndInputBuffering disables echo to avoid double echo and disable input buffering
func (s *ShellSession) disableEchoAndInputBuffering() {
	if userIn == os.Stdin && userOut == os.Stdout {
		getState(&s.originalSttyState)
		setState(bytes.NewBufferString("cbreak"))
		setState(bytes.NewBufferString("-echo"))
	}
}

// getState gets current state of terminal
func getState(state *bytes.Buffer) error {
	// Only manipulate terminal state when using OS stdio
	if userIn == os.Stdin {
		cmd := exec.Command("stty", "-g")
		cmd.Stdin = os.Stdin
		cmd.Stdout = state
		return cmd.Run()
	}
	return nil
}

// setState sets the new settings to terminal
func setState(state *bytes.Buffer) error {
	// Only manipulate terminal state when using OS stdio
	if userIn == os.Stdin && userOut == os.Stdout {
		cmd := exec.Command("stty", state.String())
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		return cmd.Run()
	}
	return nil
}

// stop restores the terminal settings and exits
func (s *ShellSession) Stop() {
	setState(&s.originalSttyState)
	setState(bytes.NewBufferString("echo")) // for linux and ubuntu
	os.Exit(0)
}

// handleKeyboardInput handles input entered by customer on terminal
func (s *ShellSession) handleKeyboardInput(log log.T) (err error) {
	var (
		stdinBytesLen int
	)

	//handle double echo and disable input buffering
	s.disableEchoAndInputBuffering()

	stdinBytes := make([]byte, StdinBufferLimit)
	reader := bufio.NewReader(userIn)
	for {
		if stdinBytesLen, err = reader.Read(stdinBytes); err != nil {
			log.Errorf("Unable read from Stdin: %v", err)
			break
		}

		if err = s.Session.DataChannel.SendInputDataMessage(log, message.Output, stdinBytes[:stdinBytesLen]); err != nil {
			log.Errorf("Failed to send UTF8 char: %v", err)
			break
		}
		// sleep to limit the rate of data transfer
		time.Sleep(time.Millisecond)
	}
	return
}
