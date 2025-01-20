package utils

import "testing"

func TestEncryptPassword(t *testing.T) {
	password := "123456"
	t.Run("EncryptPassword", func(t *testing.T) {
		hashedPassword, err := EncryptPassword(password)
		if err != nil {
			t.Fatal(err)
		}

		t.Log(hashedPassword)
	})

	t.Run("ComparePassword", func(t *testing.T) {
		hashedPassword := "$2a$10$PLHhDRCM1u5b10kCXCTu9O6nWk/dSLo5RWlwbKoyMITOwfBFVuzn2"
		err := ComparePassword(hashedPassword, password)
		if err != nil {
			t.Fatal(err)
		}
		t.Log(true)
	})
}
