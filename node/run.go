//go:build !windows

package node

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"
	"wireflow/dns"
	"wireflow/internal"
	"wireflow/management/vo"
	"wireflow/monitor"
	"wireflow/monitor/collector"
	"wireflow/pkg/config"
	"wireflow/pkg/log"

	wg "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"golang.zx2c4.com/wireguard/wgctrl"
)

// Start start wireflow
func Start(flags *LinkFlags) error {
	var err error
	ctx := SetupSignalHandler()

	logger := log.NewLogger(log.Loglevel, "wireflow")

	conf, err := config.GetLocalConfig()
	if err != nil {
		return err
	}

	engineCfg := &EngineConfig{
		Logger:        logger,
		Conf:          conf,
		Port:          51820,
		InterfaceName: flags.InterfaceName,
		ManagementUrl: flags.ManagementUrl,
		SignalingUrl:  flags.SignalingUrl,
		TurnServerUrl: flags.TurnServerUrl,
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

	if flags.DaemonGround {
		fmt.Println("Run wireflow in daemon mode")
		env := os.Environ()
		env = append(env, "LINKANY_DAEMON=true")
		if os.Getenv("LINKANY_DAEMON") == "" {
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

			if _, err := os.Stat(logDir); err != nil {
				// 如果目录不存在或不是目录，则创建目录
				if err := os.MkdirAll(logDir, 0755); err != nil {
					fmt.Printf("Failed to create log directory: %v\n", err)
					os.Exit(1)
				}
			} else {
				fmt.Printf("Log directory already exists: %s\n", logDir)
			}

			// 打开日志文件
			logFile, err := os.OpenFile(
				filepath.Join(logDir, "wireflow.log"),
				os.O_CREATE|os.O_WRONLY|os.O_APPEND,
				0644,
			)
			if err != nil {
				fmt.Printf("Failed to open log file: %v\n", err)
				os.Exit(1)
			}

			files := [3]*os.File{}
			if flags.LogLevel != "" && flags.LogLevel != "slient" {
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

			path, err := os.Executable()
			if err != nil {
				logger.Errorf("Failed to determine executable: %v", err)
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
				logger.Errorf("Failed to daemonize: %v", err)
				os.Exit(1)
			}
			process.Release()
			os.Exit(0) // exit parent
		}

	}

	// enable metrics
	if flags.MetricsEnable {
		go func() {
			metric := monitor.NewNodeMonitor(10*time.Second, collector.NewPrometheusStorage(""), nil)
			metric.AddCollector(&collector.CPUCollector{})
			metric.AddCollector(&collector.MemoryCollector{})
			metric.AddCollector(&collector.DiskCollector{})
			metric.AddCollector(&collector.TrafficCollector{})
			metric.Start()
			fmt.Println("metrics started")
		}()
	}

	// enable linkDNS
	if flags.DnsEnable {
		go func() {
			linkDns := dns.NewLinkDNS(&dns.DNSConfig{})
			linkDns.Start()
			fmt.Println("linkDNS started")
		}()
	}

	engine, err := NewEngine(engineCfg)
	if err != nil {
		return err
	}

	engine.GetNetworkMap = func() (*vo.NetworkMap, error) {
		// get network map from list
		conf, err := engine.mgtClient.GetNetMap()
		if err != nil {
			logger.Errorf("Get network map failed: %v", err)
			return nil, err
		}

		logger.Infof("Success get net map")

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
	logger.Infof("wireflow started")

	<-ctx.Done()
	uapi.Close()

	engine.close()
	logger.Infof("wireflow shutting down")
	return err
}

// Stop stop wireflow daemon
func Stop(flags *LinkFlags) error {
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
			return fmt.Errorf("%s", "Wireflow daemon is not running, no devices found")
		}

		interfaceName = devices[0].Name
	}
	// 如果 UAPI 失败，尝试通过 PID 文件停止进程
	return stopViaPIDFile(interfaceName)

}

func Status(flags *LinkFlags) error {
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

	fmt.Printf("Linkany interface: %s\n", interfaceName)
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
