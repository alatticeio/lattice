package server

import (
	"context"
	"testing"
	"time"
)

func TestContext(t *testing.T) {
	t.Run("test", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		do()

		select {
		case <-ctx.Done():
			t.Logf("context done: %v", ctx.Err())
		default:
			t.Logf("context not done")
		}

	})
}

func do() {
	time.Sleep(15 * time.Second)
}
