// Copyright 2025 The Wireflow Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build windows
// +build windows

package client

import (
	"fmt"
	"net"
	"os"
	"wireflow/internal"
	"wireflow/management/vo"

	wg "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"golang.zx2c4.com/wireguard/wgctrl"
)

func Start(flags *Flags) error {

	var err error
	ctx := internal.SetupSignalHandler()

	logger := log.NewLogger(log.Loglevel, "linkany")

	// peers config to wireGuard
	engineCfg := &ClientConfig{
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
		engineCfg.SignalingUrl = fmt.Sprintf("nats://%s:%d", internal.SignalingDomain, internal.DefaultSignalingPort)
	}

	if flags.TurnServerUrl == "" {
		engineCfg.TurnServerUrl = fmt.Sprintf("%s:%d", internal.TurnServerDomain, internal.DefaultTurnServerPort)
	}

	engine, err := NewClient(engineCfg)
	if err != nil {
		return err
	}

	engine.GetNetworkMap = func() (*vo.NetworkMap, error) {
		// get network map from list
		conf, err := engine.ctrClient.GetNetMap()
		if err != nil {
			logger.Errorf("Get network map failed: %v", err)
			return nil, err
		}

		logger.Infof("Success get net map")

		return conf, err
	}

	//ticker := time.NewTicker(10 * time.Second) //30 seconds will sync config a time
	quit := make(chan struct{})
	defer close(quit)
	// start iface
	err = engine.Start()

	// open UAPI file (or use supplied fd)
	logger.Infof("got iface name: %s", engine.Name)

	uapi, err := ipc.UAPIListen(engine.Name)
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
	logger.Infof("UAPI listener started")

	<-ctx.Done()
	uapi.Close()

	logger.Infof("linkany shutting down")
	return err
}

// Stop stop linkany daemon
func Stop(flags *Flags) error {
	interfaceName := flags.InterfaceName
	if flags.InterfaceName == "" {
		ctr, err := wgctrl.New()
		if err != nil {
			return nil
		}

		devices, err := ctr.Devices()
		if err != nil {
			return err
		}

		if len(devices) == 0 {
			return fmt.Errorf("没有找到任何 Linkany 设备")
		}

		interfaceName = devices[0].Name
	}
	// 如果 UAPI 失败，尝试通过 PID 文件停止进程
	return stopViaPIDFile(interfaceName)

}

// stop linkany daemon via sock file
func stopViaPIDFile(interfaceName string) error {
	// get sock
	socketPath := fmt.Sprintf("/var/run/wireguard/%s.sock", interfaceName)
	// check sock
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		return fmt.Errorf("file %s not exists", socketPath)
	}

	// connect to the socket
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return fmt.Errorf("linkany sock connect failed: %v", err)
	}
	defer conn.Close()
	// 发送消息到服务器
	_, err = conn.Write([]byte("stop\n"))
	if err != nil {
		return fmt.Errorf("send stop failed: %v", err)
	}

	// receive
	buffer := make([]byte, 4096)
	_, err = conn.Read(buffer)
	if err != nil {
		return fmt.Errorf("receive error: %v", err)
	}

	fmt.Printf("linkany stopped: %s\n", interfaceName)
	return nil
}
