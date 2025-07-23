//go:build !windows

package node

import (
	"fmt"
	wg "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"golang.zx2c4.com/wireguard/wgctrl"
	"linkany/internal"
	"linkany/management/vo"
	"linkany/pkg/config"
	"linkany/pkg/log"
	"net"
	"os"
	"path/filepath"
	"runtime"
)

// Start start linkany
func Start(flags *LinkFlags) error {
	var err error
	ctx := SetupSignalHandler()

	logger := log.NewLogger(log.Loglevel, "linkany")

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

	if flags.DaemonGround {
		fmt.Println("Run linkany in daemon mode")
		env := os.Environ()
		env = append(env, "LINKANY_DAEMON=true")
		if os.Getenv("LINKANY_DAEMON") == "" {
			// 确保日志目录存在
			var logDir string
			switch runtime.GOOS {
			case "darwin":
				host, _ := os.UserHomeDir()
				logDir = fmt.Sprintf("%s/%s", host, "Library/Logs/linkany")
			case "windows":
				logDir = "C:\\ProgramData\\linkany\\logs"
			default:
				logDir = "/var/log/linkany"
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
				filepath.Join(logDir, "linkany.log"),
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
	logger.Infof("Linkany started")

	<-ctx.Done()
	uapi.Close()

	engine.close()
	logger.Infof("Linkany shutting down")
	return err
}

// Stop stop linkany daemon
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
			return fmt.Errorf("%s", "Linkany daemon is not running, no devices found")
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
