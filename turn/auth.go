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
