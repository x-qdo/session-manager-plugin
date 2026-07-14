// Copyright 2026 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not use
// this file except in compliance with the License. A copy of the License is
// located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed on
// an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
// or implied. See the License for the specific language governing permissions
// and limitations under the License.

package uuid

import "testing"

func TestNewReturnsVersion4RFC4122UUID(t *testing.T) {
	value := New()
	if got := value[6] >> 4; got != 4 {
		t.Fatalf("version = %d, want 4", got)
	}
	if got := value[8] >> 6; got != 2 {
		t.Fatalf("variant bits = %d, want 2", got)
	}

	parsed, err := Parse(value.String())
	if err != nil {
		t.Fatalf("Parse(New().String()) returned error: %v", err)
	}
	if parsed != value {
		t.Fatalf("round trip = %v, want %v", parsed, value)
	}
}

func TestParseRejectsMalformedValues(t *testing.T) {
	for _, input := range []string{
		"",
		"dd01e56bff48483ea508b5f073f31b1",
		"dd01e56b_ff48-483e-a508-b5f073f31b16",
		"dd01e56b-ff48-483e-a508-b5f073f31b1z",
	} {
		if _, err := Parse(input); err == nil {
			t.Errorf("Parse(%q) succeeded, want error", input)
		}
	}
}

func TestFromBytesCopiesInput(t *testing.T) {
	input := make([]byte, 16)
	input[0] = 1
	value, err := FromBytes(input)
	if err != nil {
		t.Fatalf("FromBytes returned error: %v", err)
	}
	input[0] = 2
	if value[0] != 1 {
		t.Fatalf("UUID changed with input: got %d, want 1", value[0])
	}
	if _, err := FromBytes(input[:15]); err == nil {
		t.Fatal("FromBytes accepted a short input")
	}
}
