package main

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	// 这里设置你想要的明文密码
	password := "admin123"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), 10)
	fmt.Println(string(hash))
}
