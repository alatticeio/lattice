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

package models

import "github.com/golang-jwt/jwt/v5"

// AgentClaims are the JWT claims issued to an AI Agent after successful registration.
// The "sub" field (from RegisteredClaims) holds the AgentIdentity name.
// Agents present this JWT in Authorization: Bearer <token> for tool API calls.
type AgentClaims struct {
	jwt.RegisteredClaims
	// AgentID is the AgentIdentity resource name (same as sub, kept explicit for readability).
	AgentID string `json:"agent_id"`
	// Namespace is the K8s namespace this agent belongs to.
	Namespace string `json:"namespace"`
	// AllowedTools is a snapshot of the tool whitelist at issuance time.
	// The live check always re-reads the AgentIdentity CRD; this is for audit purposes.
	AllowedTools []string `json:"allowed_tools"`
}

// IsAgentToken returns true if this is an agent JWT (as opposed to a user JWT).
// Used by middleware to route to the correct claim type.
func IsAgentToken(claims jwt.Claims) bool {
	_, ok := claims.(*AgentClaims)
	return ok
}
