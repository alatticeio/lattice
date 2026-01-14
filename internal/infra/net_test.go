package infra

import "testing"

func TestGetCidrFromIP(t *testing.T) {
	s := GetCidrFromIP("10.0.0.2")
	t.Logf("%s", s)
}
