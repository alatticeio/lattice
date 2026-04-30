//go:build windows
// +build windows

// // Copyright 2026 The Lattice Authors, Inc.
// //
// // Licensed under the Apache License, Version 2.0 (the "License");
// // you may not use this file except in compliance with the License.
// // You may obtain a copy of the License at
// //
// //     http://www.apache.org/licenses/LICENSE-2.0
// //
// // Unless required by applicable law or agreed to in writing, software
// // distributed under the License is distributed on an "AS IS" BASIS,
// // WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// // See the License for the specific language governing permissions and
// // limitations under the License.
package agent

import (
	"context"
	"fmt"
	"github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/wireguard"
	"github.com/alatticeio/lattice/internal/dns"
	"golang.zx2c4.com/wireguard/ipc"
	"golang.zx2c4.com/wireguard/wgctrl"
	"net"
	"os"
	"path/filepath"
	"runtime"
)

func Start(ctx context.Context, flags *config.Config) error {
	var (
		logFile *os.File
		path    string
		err     error
	)

	log.SetLevel(flags.Level)

	logger := log.GetLogger("lattice")

	// peers config to wireGuard
	agentCfg := &NodeConfig{
		Logger:        logger,
		Port:          flags.WgPort,
		InterfaceName: flags.InterfaceName,
		Token:         flags.Token,
		ShowLog:       flags.EnableSysLog,
		Flags:         flags,
	}

	if flags.EnableDaemon {
		fmt.Println("Run lattice in daemon mode")
		env := os.Environ()
		env = append(env, "LATTICE_DAEMON=true")
		if os.Getenv("LATTICE_DAEMON") == "" {
			// 确保日志目录存在
			var logDir string
			switch runtime.GOOS {
			case "darwin":
				host, _ := os.UserHomeDir()
				logDir = fmt.Sprintf("%s/%s", host, "Library/Logs/lattice")
			case "windows":
				logDir = "C:\\ProgramData\\lattice\\logs"
			default:
				logDir = "/var/log/lattice"
			}

			if _, err = os.Stat(logDir); err != nil {
				// 如果目录不存在或不是目录，则创建目录
				if err = os.MkdirAll(logDir, 0755); err != nil {
					fmt.Printf("Failed to create log directory: %v\n", err)
					os.Exit(1)
				}
			} else {
				fmt.Printf("Log directory already exists: %s\n", logDir)
			}

			// 打开日志文件
			logFile, err = os.OpenFile(
				filepath.Join(logDir, "lattice.log"),
				os.O_CREATE|os.O_WRONLY|os.O_APPEND,
				0644,
			)
			if err != nil {
				fmt.Printf("Failed to open log file: %v\n", err)
				os.Exit(1)
			}

			files := [3]*os.File{}
			if flags.Level != "" && flags.Level != "slient" {
				files[0], _ = os.Open(os.DevNull)
				files[1] = logFile
				files[2] = logFile
			} else {
				files[0], _ = os.Open(os.DevNull)
				files[1], _ = os.Open(os.DevNull)
				files[2], _ = os.Open(os.DevNull)
			}
			attr := &os.ProcAttr{
				Files: []*os.File{
					files[0], // stdin
					files[1], // stdout
					files[2], // stderr
					//tdev.File(),
					//fileUAPI,
				},
				Dir: ".",
				Env: env,
			}

			path, err = os.Executable()
			if err != nil {
				logger.Error("Failed to determine executable", err)
				os.Exit(1)
			}

			filteredArgs := make([]string, 0)
			for _, arg := range os.Args {
				if arg != "--daemon" && arg != "-d" && arg != "--foreground" && arg != "-f" {
					filteredArgs = append(filteredArgs, arg)
				}
			}

			process, err := os.StartProcess(
				path,
				filteredArgs,
				attr,
			)
			if err != nil {
				logger.Error("Failed to daemonize", err)
				os.Exit(1)
			}
			process.Release()
			os.Exit(0) // exit parent
		}
	}

	// enable DNS
	if flags.EnableDNS {
		go func() {
			nativeDNS := dns.NewNativeDNS(&dns.DNSConfig{})
			nativeDNS.Start()
			fmt.Println("Dns started")
		}()
	}

	c, err := NewNode(ctx, agentCfg)
	if err != nil {
		return err
	}

	c.GetNetworkMap = func() (*infra.Message, error) {
		// get network map from list
		msg, err := c.ctrClient.GetNetMap(flags.Token)
		if err != nil {
			logger.Error("Get network map failed", err)
			return nil, err
		}

		logger.Debug("network map fetched")

		return msg, err
	}

	err = c.Start(ctx)

	// Start heartbeat so the management server can track online status.
	go c.StartHeartbeat(ctx)

	// open UAPI file
	logger.Debug("Interface name", "name", c.Name)

	uapi, err := ipc.UAPIListen(c.Name)
	if err != nil {
		logger.Error("failed to listen on UAPI socket", err)
		os.Exit(-1)
	}

	go func() {
		for {
			conn, err := uapi.Accept()
			if err != nil {
				return
			}
			go c.DeviceManager.IpcHandle(conn)
		}
	}()
	logger.Info("lattice started")

	<-ctx.Done()
	uapi.Close()

	c.close()
	logger.Info("lattice shutting down")
	return err
}

// Stop stop lattice daemon
func Stop(flags *config.Config) error {
	interfaceName := flags.InterfaceName
	if flags.InterfaceName == "" {
		ctr, err := wgctrl.New()
		if err != nil {
			return nil
		}

		ifaces, err := ctr.Devices()
		if err != nil {
			return err
		}

		if len(ifaces) == 0 {
			return fmt.Errorf("%s", "Lattice daemon is not running, no ifaces found")
		}

		interfaceName = ifaces[0].Name
	}
	// 如果 UAPI 失败，尝试通过 PID 文件停止进程
	return stopViaPIDFile(interfaceName)

}

func Status(flags *config.Config) error {
	return wireguard.PrintStatus(flags.InterfaceName)
}

// stop lattice daemon via sock file
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
		return fmt.Errorf("lattice sock connect failed: %v", err)
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

	fmt.Printf("lattice stopped: %s\n", interfaceName)
	return nil
}
