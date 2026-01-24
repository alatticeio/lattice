package main

import (
	"fmt"
	"os"
	"wireflow/internal/infra"
	nats2 "wireflow/management/nats"
	"wireflow/management/transport"
	"wireflow/pkg/utils"
	"wireflow/wrrper"

	"golang.zx2c4.com/wireguard/conn"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

func main() {
	args := os.Args
	localIdStr := args[1]
	remoteIdStr := args[2]

	key1, err := utils.ParseKey(localIdStr)
	if err != nil {
		panic(err)
	}
	key2, err := utils.ParseKey(remoteIdStr)
	if err != nil {
		panic(err)
	}

	localId := infra.FromKey(key1)
	remoteId := infra.FromKey(key2)

	//p := args[3]
	//port, err := strconv.Atoi(p)
	//if err != nil {
	//	panic(err)
	//}

	//localSessionId, err := transport.GenerateSessionID()
	//if err != nil {
	//	panic(err)
	//}
	wrrpClient, err := wrrper.NewWrrpClient(localId, "127.0.0.1:6266")
	if err := wrrpClient.Connect(); err != nil {
		panic(err)
	}

	go func() {
		for {
			bufs := make([][]byte, 1)
			bufs[0] = make([]byte, 1024)
			sizes := make([]int, 1)
			endpoints := make([]conn.Endpoint, 1)
			fn := wrrpClient.ReceiveFunc()
			_, err = fn(bufs, sizes, endpoints)
			if err != nil {
				panic(err)
			}
		}
	}()

	ctx := signals.SetupSignalHandler()
	nats, err := nats2.NewNatsService(ctx, "nats://81.68.109.143:4222")
	if err != nil {
		panic(err)
	}

	peerManager := infra.NewPeerManager()
	//conn, _, err := infra.ListenUDP("udp", uint16(port))
	//dialer := transport.NewIceDialer(&transport.ICEDialerConfig{
	//	Sender:                 nats.Send,
	//	LocalId:                localId,
	//	RemoteId:               remoteId,
	//	UniversalUdpMuxDefault: infra.NewUdpMux(conn, false),
	//	PeerManager:            peerManager,
	//})

	//peerManager.AddPeer(localId, &infra.Peer{
	//	PublicKey: localId,
	//})

	probeFactory := transport.NewProbeFactory(&transport.ProbeFactoryConfig{
		LocalId:     localId,
		Signal:      nats,
		Wrrp:        wrrpClient,
		PeerManager: peerManager,
	})

	wrrpClient.Configure(wrrper.WithOnMessage(probeFactory.Handle))

	if err = nats.Subscribe(fmt.Sprintf("%s.%s", "wireflow.signals.peers", localId), probeFactory.Handle); err != nil {
		panic(err)
	}

	probe, err := probeFactory.Get(remoteId)
	if err != nil {
		panic(err)
	}

	if err = probe.Start(ctx, remoteId); err != nil {
		panic(err)
	}

	<-ctx.Done()
}
