package main

import (
	"k8s.io/klog/v2"
	"net"
	"time"
)

type Message struct {
	Addr net.UDPAddr
	Buff []byte
}

func main() {

	client, _ := probe.NewClient(&probe.ClientConfig{
		ServerUrl: "stun.linkany.io:3478",
	})

	info, err := client.GetRelayInfo(true)
	if err != nil {
		panic(err)
	}

	klog.Infof("MappedAddr: %v, RelayConn: %v", info.MappedAddr, info.RelayConn.LocalAddr())
	udpAddr, _ := net.ResolveUDPAddr("udp", "81.68.109.143:0")
	if err := client.CreatePermission(udpAddr); err != nil {
		panic(err)
	}

	go func() {
		for {
			b := make([]byte, 1024)
			n, addr, err := info.RelayConn.ReadFrom(b)
			if err != nil {
				panic(err)
			}

			klog.Infof("Received from %v: %v", addr, string(b[:n]))

			info.RelayConn.WriteTo(b, addr)
			klog.Infof("Sent to %v: %v", addr, string(b[:n]))
		}
	}()

	for {
		time.Sleep(1000000)
	}
}
