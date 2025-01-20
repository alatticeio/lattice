package utils

import "testing"

func TestTokener_Generate(t *testing.T) {
	username := "linkany"
	password := "linkany.io"
	tokener := NewTokener()
	token, err := tokener.Generate(username, password)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(token)
}

func TestTokener_Verify(t *testing.T) {
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYmYiOjE0NDQ0Nzg0MDAsInBhc3N3b3JkIjoibGlua2FueS5pbyIsInVzZXJuYW1lIjoibGlua2FueSJ9.Jy5OtOZmytoAcwP8oa2uJO1ibE_9bjV0aRfo1tqwEhw"
	username := "linkany"
	password := "linkany.io"
	tokener := NewTokener()
	if b, err := tokener.Verify(username, password, token); err != nil {
		t.Fatal(err)
	} else {
		t.Log(b)
	}
}
