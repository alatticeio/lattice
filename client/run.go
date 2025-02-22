//go:build !windows
// +build !windows

package client

import (
	"fmt"
	wg "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"linkany/internal"
	"linkany/pkg/config"
	"linkany/pkg/log"
	"os"
)

func Start(interfaceName string, isRelay bool) error {

	var err error
	ctx := SetupSignalHandler()

	logger := log.NewLogger(log.Loglevel, fmt.Sprintf("[%s] ", "linkany"))

	conf, err := config.GetLocalConfig()
	if err != nil {
		return err
	}

	// peers config to wireguard
	engine, err := NewEngine(&EngineParams{
		Logger:        logger,
		Conf:          conf,
		Port:          51820,
		InterfaceName: interfaceName,
		WgLogger: wg.NewLogger(
			wg.LogLevelError,
			fmt.Sprintf("(%s) ", interfaceName),
		),
		ForceRelay:     isRelay,
		ManagementAddr: fmt.Sprintf("%s:%d", internal.ManagementDomain, internal.DefaultManagementPort),
		SignalingAddr:  fmt.Sprintf("%s:%d", internal.SignalingDomain, internal.DefaultSignalingPort),
	})
	if err != nil {
		return err
	}

	engine.GetNetworkMap = func() (*config.DeviceConf, error) {
		// get network map from list
		conf, err := engine.client.List()
		if err != nil {
			logger.Errorf("get networkmap failed: %v", err)
			return nil, err
		}

		logger.Infof("success get networkmap")

		return conf, err
	}

	err = engine.Start()

	// open UAPI file (or use supplied fd)
	logger.Infof("device name: %s", engine.Name)
	fileUAPI, err := func() (*os.File, error) {
		return ipc.UAPIOpen(engine.Name)
	}()

	uapi, err := ipc.UAPIListen(engine.Name, fileUAPI)
	if err != nil {
		logger.Errorf("Failed to listen on uapi socket: %v", err)
		os.Exit(-1)
	}

	go func() {
		for {
			conn, err := uapi.Accept()
			if err != nil {
				return
			}
			go engine.IpcHandle(conn)
		}
	}()
	logger.Infof("Linkany started")

	<-ctx.Done()
	uapi.Close()

	engine.close()
	logger.Infof("linkany shutting down")
	return err
}
