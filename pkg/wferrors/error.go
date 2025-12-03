// Copyright 2025 wireflowio.com, Inc.
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

package wferrors

import "errors"

var (
	ErrAppKeyRequired      = errors.New("app key is required")
	ErrInvalidToken        = errors.New("invalid token")
	ErrorServerInterval    = errors.New("interval server error")
	ErrInvalidOffer        = errors.New("invalid offer")
	ErrChannelNotExists    = errors.New("channel not exists")
	ErrClientCanceled      = errors.New("client canceled")
	ErrClientClosed        = errors.New("client closed")
	ErrProberNotFound      = errors.New("prober not found")
	ErrPasswordRequired    = errors.New("password required")
	ErrUserNotFound        = errors.New("user not found")
	ErrInvalidPassword     = errors.New("invalid password")
	ErrAgentNotFound       = errors.New("agent not found")
	ErrProbeFailed         = errors.New("probe connect failed, need check the network you are in")
	ErrorNotSameGroup      = errors.New("not in the same group")
	ErrInvitationExists    = errors.New("invitation already exists")
	ErrNoAccessPermissions = errors.New("no permissions to access this resource,please contact to resource owner")

	ErrDeleteSharedGroup = errors.New("cannot delete shared group, please contact the owner")
	ErrDeleteSharedNode  = errors.New("cannot delete shared node, please contact the owner")
)
