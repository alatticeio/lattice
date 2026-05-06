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

package server

import (
	"fmt"

	"github.com/alatticeio/lattice/pkg/utils/resp"
	"github.com/gin-gonic/gin"
)

// requireFeature returns a Gin middleware that checks whether the active license
// grants the specified feature. If not, it returns a 402 Payment Required response.
// In community builds, the license verifier always returns false for all features,
// so this effectively gates Pro features behind a valid license.
//
// Usage:
//
//	router.GET("/api/v1/foo", s.requireFeature("foo"), s.handleFoo)
func (s *Server) requireFeature(feature string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.licenseVerifier.HasFeature(feature) {
			resp.PaymentRequired(c,
				fmt.Sprintf("%s requires Lattice Pro — upgrade at https://alattice.io/pro", feature))
			c.Abort()
			return
		}
		c.Next()
	}
}
