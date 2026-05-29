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

package serverutils

import (
	"net"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

func init() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		if err := v.RegisterValidation("safe_string", func(fl validator.FieldLevel) bool {
			return IsSafeString(fl.Field().String())
		}); err != nil {
			panic("failed to register safe_string validator: " + err.Error())
		}
		if err := v.RegisterValidation("cidr", func(fl validator.FieldLevel) bool {
			return IsValidCIDR(fl.Field().String())
		}); err != nil {
			panic("failed to register cidr validator: " + err.Error())
		}
	}
}

var safeStringRE = regexp.MustCompile(`^[a-zA-Z0-9_\-\. ]+$`)

// IsSafeString returns true if s contains no HTML/JS injection characters.
func IsSafeString(s string) bool {
	return safeStringRE.MatchString(s)
}

// IsValidCIDR returns true if s is a valid CIDR notation.
func IsValidCIDR(s string) bool {
	_, _, err := net.ParseCIDR(s)
	return err == nil
}

// SanitizeString trims and removes null bytes.
func SanitizeString(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\x00", "")
	return s
}
