package server

import (
	"context"
	"linkany/management/client"
	"linkany/pkg/redis"
	"net"
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
