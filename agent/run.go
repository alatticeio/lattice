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

//go:build !windows

package agent

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"wireflow/dns"
	"wireflow/internal/config"
	"wireflow/internal/infra"
	"wireflow/internal/log"
	"wireflow/monitor"

	"golang.org/x/sync/errgroup"
	wg "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"golang.zx2c4.com/wireguard/wgctrl"
)

// Start start wireflow
// nolint:all
func Start(ctx context.Context, flags *config.Flags) error {
	var (
		logFile *os.File
		path    string
		err     error
		process *os.Process
	)

	log.SetLevel(flags.Level)

	logger := log.GetLogger("wireflow")

	agentCfg := &AgentConfig{
		Logger:        logger,
		Port:          51820,
		InterfaceName: flags.InterfaceName,
		Token:         flags.Token,
		ShowLog:       flags.EnableSysLog,
		WgLogger: wg.NewLogger(
			wg.LogLevelError,
			fmt.Sprintf("(%s) ", flags.InterfaceName),
		),
		Flags: flags,
	}

	// 创建一个随主程序生命周期管理的 Context
	g, newCtx := errgroup.WithContext(ctx)

	if flags.EnableDaemon {
		fmt.Println("Run wireflow in daemon mode")
		env := os.Environ()
		env = append(env, "WIREFLOW_DAEMON=true")
		if os.Getenv("WIREFLOW_DAEMON") == "" {
			// 确保日志目录存在
			var logDir string
			switch runtime.GOOS {
			case "darwin":
				host, _ := os.UserHomeDir()
				logDir = fmt.Sprintf("%s/%s", host, "Library/Logs/wireflow")
			case "windows":
				logDir = "C:\\ProgramData\\wireflow\\logs"
			default:
				logDir = "/var/log/wireflow"
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
				filepath.Join(logDir, "wireflow.log"),
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

			process, err = os.StartProcess(
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

	// enable metrics
	if flags.EnableMetric {
		g.Go(func() error {
			select {
			case <-newCtx.Done():
				return newCtx.Err()
			default:
				runner := monitor.NewMonitorRunner(infra.NewPeerManager())
				err := runner.Run(ctx)
				return err
			}

		})
	}

	// enable DNS
	if flags.EnableDNS {
		go func() {
			nativeDNS := dns.NewNativeDNS(&dns.DNSConfig{})
			err := nativeDNS.Start()
			if err != nil {
				logger.Error("Failed to start", err)
			}
			fmt.Println("Dns started")
		}()
	}

	var c *Agent

	c, err = NewAgent(ctx, agentCfg)
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

		logger.Debug("Success get net map")

		return msg, err
	}

	err = c.Start(ctx)

	// open UAPI file
	logger.Debug("Interface name", "name", c.Name)
	fileUAPI, err := func() (*os.File, error) {
		return ipc.UAPIOpen(c.Name)
	}()

	uapi, err := ipc.UAPIListen(c.Name, fileUAPI)
	if err != nil {
		logger.Error("Failed to listen on uapi socket", err)
		os.Exit(-1)
	}

	g.Go(func() error {
		// 1. 监听退出信号，手动关闭 listener 以解开 Accept 的阻塞
		go func() {
			<-newCtx.Done()
			uapi.Close()
		}()

		for {
			// Accept 会阻塞在这里，直到有新连接或 listener 被关闭
			conn, err := uapi.Accept()
			if err != nil {
				// 如果是因为 context 取消导致的关闭，返回错误
				select {
				case <-newCtx.Done():
					return newCtx.Err()
				default:
					return fmt.Errorf("ipc accept error: %w", err)
				}
			}

			// 2. 异步处理连接，不阻塞 Accept 接收下一个请求
			go func(nc net.Conn) {
				defer nc.Close() // 3. 确保连接关闭
				c.DeviceManager.IpcHandle(nc)
			}(conn)
		}
	})

	logger.Info("wireflow started")

	// 等待所有任务完成（或者任意一个任务出错退出）
	if err = g.Wait(); err != nil {
		uapi.Close()
		c.close()
		logger.Warn("wireflow shutting down")
	}

	return err
}

// Stop stop wireflow daemon
func Stop(flags *config.Flags) error {
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
			return fmt.Errorf("%s", "Wireflow daemon is not running, no ifaces found")
		}

		interfaceName = ifaces[0].Name
	}
	// 如果 UAPI 失败，尝试通过 PID 文件停止进程
	return stopViaPIDFile(interfaceName)
}

func Status(flags *config.Flags) error {
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
			return fmt.Errorf("Could not found WireFlow Devices")
		}

		interfaceName = devices[0].Name
	}

	fmt.Printf("Wierflow interface: %s\n", interfaceName)
	return nil
}

// stop wireflow daemon via sock file
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
		return fmt.Errorf("wireflow sock connect failed: %v", err)
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

	fmt.Printf("wireflow stopped: %s\n", interfaceName)
	return nil
}
