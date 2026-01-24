package utils

import "golang.zx2c4.com/wireguard/wgctrl/wgtypes"

func GetPublicKey() (string, error) {
	privateKey, err := wgtypes.GenerateKey()
	if err != nil {
		return "", err
	}

	return privateKey.PublicKey().String(), nil
}

func KeyPair() (wgtypes.Key, wgtypes.Key, error) {
	privateKey, err := wgtypes.GenerateKey()
	if err != nil {
		return wgtypes.Key{}, wgtypes.Key{}, err
	}
	publicKey := privateKey.PublicKey()

	return privateKey, publicKey, nil
}

func ParseKey(str string) (wgtypes.Key, error) {
	return wgtypes.ParseKey(str)
}
