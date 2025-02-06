package mapper

import (
	"fmt"
	"testing"
)

func TestQueryParams_Generate(t *testing.T) {
	t.Run("test query params", func(t *testing.T) {
		var pubKey = "qwqasxzdfdsa"
		var userId = "123455"
		var online = 1

		params := &QueryParams{
			PubKey: &pubKey,
			UserId: &userId,
			Online: &online,
		}

		sql, filters := Generate(params)
		fmt.Println(sql, filters)
	})
}
