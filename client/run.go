//go:build !windows
// +build !windows

package client

import (
	"fmt"
	wg "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"k8s.io/klog/v2"
	"linkany/internal"
	"linkany/pkg/config"
	"os"
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
	logger := wg.NewLogger(
		wg.LogLevelVerbose,
		fmt.Sprintf("(%s) ", interfaceName),
	)

	conf, err := config.GetLocalConfig()
	if err != nil {
		return err
	}

	// peers config to wireguard
	engine, err := NewEngine(&EngineParams{
		Conf:           conf,
		Port:           51820,
		InterfaceName:  interfaceName,
		Logger:         logger,
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
			klog.Errorf("get networkmap failed: %v", err)
			return nil, err
		}

		klog.Infof("success get networkmap")

		return conf, err
	}

	err = engine.Start()

	// open UAPI file (or use supplied fd)
	klog.Infof("device name: %s", engine.Name)
	fileUAPI, err := func() (*os.File, error) {
		return ipc.UAPIOpen(engine.Name)
	}()

	uapi, err := ipc.UAPIListen(engine.Name, fileUAPI)
	if err != nil {
		klog.Errorf("Failed to listen on uapi socket: %v", err)
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
	klog.Infof("Linkany started")

	<-ctx.Done()
	uapi.Close()

	engine.close()
	klog.Infof("linkany shutting down")
	return err
}
