package client

import (
	"golang.zx2c4.com/wireguard/conn"
	wg "golang.zx2c4.com/wireguard/device"
	"linkany/pkg/iface"
	"linkany/pkg/probe"
)

type Backend struct {
	device       *wg.Device
	bind         conn.Bind
	relayChecker probe.RelayChecker
	wgConfiger   iface.WGConfigure
}
