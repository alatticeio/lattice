package utils

import "testing"

func TestGenerateUUID(t *testing.T) {
	uuid := GenerateUUID()
	if len(uuid) != 32 {
		t.Errorf("GenerateUUID() = %v; want length 32", uuid)
	}

	t.Log(uuid)
}
