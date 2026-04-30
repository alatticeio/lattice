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

//go:build !pro

package dex

import (
	"errors"
	"github.com/alatticeio/lattice/internal/server/service"

	"github.com/gin-gonic/gin"
)

var errProRequired = errors.New("Dex OIDC/SSO is a Lattice Pro feature — upgrade at https://alattice.io/pro")

// Dex stub: satisfies call sites in management/server/api.go.
type Dex struct{}

func NewDex(_ service.UserService) (*Dex, error) {
	return nil, errProRequired
}

func (d *Dex) Login(c *gin.Context) {
	c.JSON(503, gin.H{"error": "OIDC authentication requires Lattice Pro"})
}
