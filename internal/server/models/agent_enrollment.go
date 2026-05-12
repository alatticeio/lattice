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

import "time"

// AgentEnrollmentToken is a one-time-use bootstrap token that allows an
// AI Agent to register with LatticeD and receive an AgentIdentity + JWT.
// Tokens are short-lived (default 1 hour) and consumed on first use.
type AgentEnrollmentToken struct {
	Model
	// Token is the random hex string presented during registration.
	Token string `gorm:"uniqueIndex;size:64"`
	// AllowedNamespace restricts which namespace the registering agent may join.
	AllowedNamespace string `gorm:"size:253"`
	// AllowedTools is a JSON-encoded list of tool names the registered agent will receive.
	AllowedTools string `gorm:"type:text"`
	// UsedAt is non-nil once the token has been consumed.
	UsedAt *time.Time
	// ExpiresAt is the time after which the token is no longer valid.
	ExpiresAt time.Time
	// CreatedBy is the user ID of the admin who created this token.
	CreatedBy string `gorm:"size:64"`
}

func (AgentEnrollmentToken) TableName() string { return "la_agent_enrollment_tokens" }
