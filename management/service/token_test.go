package service

import (
	"testing"
)

func TestTokener_Generate(t *testing.T) {
	username := "linkany"
	password := "linkany.io"
	tokener := NewTokenService(nil)
	token, err := tokener.Generate(username, password)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(token)
}

func TestTokener_Verify(t *testing.T) {

}
