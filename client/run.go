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

// Start starts the linkany
func Start(flags *ClientFlags) error {

	var err error
	ctx := SetupSignalHandler()

	logger := log.NewLogger(log.Loglevel, fmt.Sprintf("[%s] ", "linkany "))

	conf, err := config.GetLocalConfig()
	if err != nil {
		return err
	}

	engineCfg := &EngineConfig{
		Logger:        logger,
		Conf:          conf,
		Port:          51820,
		InterfaceName: flags.InterfaceName,
		WgLogger: wg.NewLogger(
			wg.LogLevelError,
			fmt.Sprintf("(%s) ", flags.InterfaceName),
		),
		ForceRelay: flags.ForceRelay,
	}

	if flags.ManagementUrl == "" {
		engineCfg.ManagementUrl = fmt.Sprintf("%s:%d", internal.ManagementDomain, internal.DefaultManagementPort)
	}

	if flags.SignalingUrl == "" {
		engineCfg.SignalingUrl = fmt.Sprintf("%s:%d", internal.SignalingDomain, internal.DefaultSignalingPort)
	}

	if flags.TurnServerUrl == "" {
		engineCfg.TurnServerUrl = fmt.Sprintf("%s:%d", internal.TurnServerDomain, internal.DefaultTurnServerPort)
	}

	engine, err := NewEngine(engineCfg)
	if err != nil {
		return err
	}

	engine.GetNetworkMap = func() (*config.DeviceConf, error) {
		// get network map from list
		conf, err := engine.client.List()
		if err != nil {
			logger.Errorf("Get network map failed: %v", err)
			return nil, err
		}

		logger.Infof("Success get network map")

		return conf, err
	}

	err = engine.Start()

	// open UAPI file
	logger.Infof("Interface name is: [%s]", engine.Name)
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
	logger.Infof("Linkany shutting down")
	return err
}
