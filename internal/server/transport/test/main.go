package main

import (
	"fmt"
	"os"

	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/relay"
	nats2 "github.com/alatticeio/lattice/internal/server/nats"
	"github.com/alatticeio/lattice/internal/server/transport"
	"github.com/alatticeio/lattice/pkg/utils"

	"golang.zx2c4.com/wireguard/conn"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

// nolint:all
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

	localId := infra.NewPeerIdentity(key1.String(), key1)
	remoteId := infra.NewPeerIdentity(key2.String(), key2)

	ctx := signals.SetupSignalHandler()
	nats, err := nats2.NewNatsService(ctx, "test", "client", "nats://81.68.109.143:4222")
	if err != nil {
		panic(err)
	}

	peerManager := infra.NewPeerManager()

	// probeFactory is declared first so its Handle method can be passed directly
	// to NewWrrpClient; wrrpClient is captured by the GetWrrp closure so
	// probeFactory sees it once assigned — no Configure() on either side.
	var wrrpClient *relay.TCPClient
	probeFactory := transport.NewProbeFactory(&transport.ProbeFactoryConfig{
		LocalId:     localId,
		Signal:      nats,
		PeerManager: peerManager,
		GetWrrp:     func() infra.Wrrp { return wrrpClient },
	})

	wrrpClient, err = relay.NewTCPClient(ctx, localId.ID(), "127.0.0.1:6266", probeFactory.Handle)
	if err != nil {
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

	if err = nats.Subscribe(fmt.Sprintf("%s.%s", "lattice.signals.peers", localId), probeFactory.Handle); err != nil {
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
