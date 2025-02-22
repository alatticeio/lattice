//go:build windows
// +build windows

package client

import (
	"fmt"
	wg "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"linkany/management/client"
	"linkany/pkg/config"
	"linkany/pkg/log"
	"os"
	"time"
)

func Start(interfaceName string, isRelay bool) error {

	var err error
	ctx := SetupSignalHandler()

	// new device
	//logLevel := func() int {
	//	switch os.Getenv("LOG_LEVEL") {
	//	case "verbose", "debug":
	//		return wg.LogLevelVerbose
	//	case "error":
	//		return wg.LogLevelError
	//	case "silent":
	//		return wg.LogLevelSilent
	//	}
	//	return wg.LogLevelError
	//}()
	logger := log.NewLogger(log.LogLevelVerbose, "linkany")

	conf, err := config.GetLocalConfig()
	if err != nil {
		return err
	}

	// peers config to wireguard
	engine, err := NewEngine(&EngineParams{
		Conf:          conf,
		Port:          51820,
		InterfaceName: interfaceName,
		Logger: wg.NewLogger(
			wg.LogLevelVerbose,
			fmt.Sprintf("(%s) ", interfaceName),
		),
		ForceRelay: isRelay,
	})
	if err != nil {
		return err
	}

	engine.GetNetworkMap = func(c client.ClientInterface) (*config.DeviceConf, error) {
		// control plane fetch config from origin server
		// update config
		conf, err := c.List()
		if err != nil {
			logger.Errorf("sync peers failed: %v", err)
		}
		logger.Infof("success synced!!!")

		return conf, err
	}

	ticker := time.NewTicker(10 * time.Second) //30 seconds will sync config a time
	quit := make(chan struct{})
	defer close(quit)
	// start device
	err = engine.Start(ticker, quit)

	// open UAPI file (or use supplied fd)
	logger.Infof("got device name: %s", engine.Name)

	uapi, err := ipc.UAPIListen(engine.Name)
	if err != nil {
		wgLogger.Errorf("Failed to listen on uapi socket: %v", err)
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
	logger.Infof("UAPI listener started")

	<-ctx.Done()
	uapi.Close()

	logger.Infof("linkany shutting down")
	return err
}
