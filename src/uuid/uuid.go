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

// Package uuid provides the small RFC 4122 UUID surface used by the plugin.
package uuid

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
)

// UUID is a 128-bit RFC 4122 universally unique identifier.
type UUID [16]byte

// New returns a random version 4 UUID and panics if the system random source
// cannot be read.
func New() UUID {
	var value UUID
	if _, err := rand.Read(value[:]); err != nil {
		panic(fmt.Sprintf("uuid: random source failed: %v", err))
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return value
}

// FromBytes returns the UUID represented by input.
func FromBytes(input []byte) (UUID, error) {
	var value UUID
	if len(input) != len(value) {
		return value, fmt.Errorf("uuid: invalid byte length %d", len(input))
	}
	copy(value[:], input)
	return value, nil
}

// Parse accepts the canonical hyphenated or 32-character hexadecimal form.
func Parse(input string) (UUID, error) {
	var value UUID
	var encoded string
	switch len(input) {
	case 36:
		if input[8] != '-' || input[13] != '-' || input[18] != '-' || input[23] != '-' {
			return value, errors.New("uuid: invalid format")
		}
		encoded = input[:8] + input[9:13] + input[14:18] + input[19:23] + input[24:]
	case 32:
		encoded = input
	default:
		return value, fmt.Errorf("uuid: invalid string length %d", len(input))
	}

	if _, err := hex.Decode(value[:], []byte(encoded)); err != nil {
		return UUID{}, fmt.Errorf("uuid: invalid format: %w", err)
	}
	return value, nil
}

// String returns the canonical lowercase hyphenated representation.
func (value UUID) String() string {
	var output [36]byte
	hex.Encode(output[0:8], value[0:4])
	output[8] = '-'
	hex.Encode(output[9:13], value[4:6])
	output[13] = '-'
	hex.Encode(output[14:18], value[6:8])
	output[18] = '-'
	hex.Encode(output[19:23], value[8:10])
	output[23] = '-'
	hex.Encode(output[24:36], value[10:16])
	return string(output[:])
}

// MarshalText implements encoding.TextMarshaler.
func (value UUID) MarshalText() ([]byte, error) {
	return []byte(value.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (value *UUID) UnmarshalText(input []byte) error {
	parsed, err := Parse(string(input))
	if err != nil {
		return err
	}
	*value = parsed
	return nil
}
