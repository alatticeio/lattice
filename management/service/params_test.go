package service

import (
	"fmt"
	"linkany/management/dto"
	"testing"
)

func TestQueryParams_Generate(t *testing.T) {
	t.Run("test query params", func(t *testing.T) {
		var pubKey = "qwqasxzdfdsa"
		var userId = "123455"
		var status = 1

		params := &dto.QueryParams{
			PubKey: &pubKey,
			UserId: &userId,
			Status: &status,
		}

		sql, filters := Generate(params)
		fmt.Println(sql, filters)
	})
}
