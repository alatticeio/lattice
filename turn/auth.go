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

package turn

import (
	"context"
	"net"
	"wireflow/management/client"
	"wireflow/pkg/redis"
)

type Handler struct {
	client *client.Client
	rdb    *redis.Client
}

func (h *Handler) AuthHandler(username string, realm string, srcAddr net.Addr) ([]byte, bool) { // nolint: revive

	ctx := context.Background()

	// Get the user from redis
	user, err := h.rdb.Get(ctx, username)
	if err != nil {
		return nil, false
	}

	if user == "" {
		return nil, false
	}
	key := []byte(user)

	return key, true

}
