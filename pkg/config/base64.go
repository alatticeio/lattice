package config

import "encoding/base64"

func StringToBase64(str string) string {
	return base64.StdEncoding.EncodeToString([]byte(str))
}

func Base64Decode(str string) (string, error) {
	buff, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return "", err
	}

	return string(buff), nil
}
