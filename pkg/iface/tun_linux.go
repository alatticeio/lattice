package iface

import (
	"golang.zx2c4.com/wireguard/tun"
	"linkany/pkg/log"
	"math/rand"
	"time"
)

func CreateTUN(mtu int, logger *log.Logger) (string, tun.Device, error) {
	name := getInterfaceName()
	device, err := tun.CreateTUN(name, mtu)
	return name, device, err
}

func getInterfaceName() string {
	rand.Seed(time.Now().UnixNano())
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	bytes := make([]byte, 3)
	for i := 0; i < 3; i++ {
		bytes[i] = letters[rand.Intn(len(letters))]
	}

	return "linkany-" + string(bytes)
}
