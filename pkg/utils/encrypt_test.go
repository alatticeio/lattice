// Copyright 2026 The Lattice Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"errors"
	"testing"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name  string
		pw    string
		errIs error
	}{
		{"too short", "Ab1", ErrPasswordTooShort},
		{"no upper", "abcdefgh1", ErrPasswordNoUpper},
		{"no lower", "ABCDEFGH1", ErrPasswordNoLower},
		{"no digit", "Abcdefgh", ErrPasswordNoDigit},
		{"valid", "Abcdefg1", nil},
		{"valid long", "MySecureP@ssw0rd!", nil},
		{"valid exactly 8", "Abcdefg1", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.pw)
			if tt.errIs != nil {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.errIs)
				} else if !errors.Is(err, tt.errIs) {
					t.Fatalf("expected error %q, got %v", tt.errIs, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestEncryptPassword(t *testing.T) {
	password := "123456"
	t.Run("EncryptPassword", func(t *testing.T) {
		hashedPassword, err := EncryptPassword(password)
		if err != nil {
			t.Fatal(err)
		}

		t.Log(hashedPassword)
	})

	t.Run("ComparePassword", func(t *testing.T) {
		hashedPassword := "$2a$10$PLHhDRCM1u5b10kCXCTu9O6nWk/dSLo5RWlwbKoyMITOwfBFVuzn2"
		err := ComparePassword(hashedPassword, password)
		if err != nil {
			t.Fatal(err)
		}
		t.Log(true)
	})
}
