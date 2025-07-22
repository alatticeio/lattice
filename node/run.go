package node

import (
	"fmt"
	wg "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"linkany/internal"
	"linkany/management/vo"
	"linkany/pkg/config"
	"linkany/pkg/log"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Start start linkany daemon
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
		env := os.Environ()
		files := [3]*os.File{}
		//if os.Getenv("LOG_LEVEL") != "" && logLevel != device.LogLevelSilent {
		//	files[0], _ = os.Open(os.DevNull)
		//	files[1] = os.Stdout
		//	files[2] = os.Stderr
		//} else {
		files[0], _ = os.Open(os.DevNull)
		files[1], _ = os.Open(os.DevNull)
		files[2], _ = os.Open(os.DevNull)
		//}
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

		process, err := os.StartProcess(
			path,
			os.Args,
			attr,
		)
		if err != nil {
			logger.Errorf("Failed to daemonize: %v", err)
			os.Exit(1)
		}
		process.Release()
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

	}

	// 尝试通过 UAPI 发送停止命令
	err := stopViaUAPI(interfaceName)
	if err == nil {

		return nil
	}

	// 如果 UAPI 失败，尝试通过 PID 文件停止进程
	return stopViaPIDFile(interfaceName)

}

// 通过 UAPI 停止服务
func stopViaUAPI(interfaceName string) error {
	uapiConn, err := ipc.UAPIOpen(interfaceName)
	if err != nil {
		return fmt.Errorf("无法连接到守护进程的 UAPI 套接字: %v", err)
	}
	defer uapiConn.Close()

	// 发送停止命令，可以是特定的命令字符串
	_, err = uapiConn.Write([]byte("stop\n\n"))
	if err != nil {
		return fmt.Errorf("发送停止命令失败: %v", err)
	}

	// 读取响应
	buf := make([]byte, 4096)
	n, err := uapiConn.Read(buf)
	if err != nil {
		return fmt.Errorf("读取响应失败: %v", err)
	}

	if string(buf[:n]) != "OK\n\n" {
		return fmt.Errorf("未收到成功响应")
	}

	return nil
}

// 通过 PID 文件停止进程
func stopViaPIDFile(interfaceName string) error {
	// 获取 PID 文件路径
	pidFilePath := fmt.Sprintf("/var/run/linkany-%s.pid", interfaceName)

	// 读取 PID 文件
	pidBytes, err := os.ReadFile(pidFilePath)
	if err != nil {
		return fmt.Errorf("无法读取 PID 文件: %v", err)
	}

	// 解析 PID
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		return fmt.Errorf("无效的 PID: %v", err)
	}

	// 查找进程
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("找不到进程 (PID %d): %v", pid, err)
	}

	// 发送 SIGTERM 信号
	err = process.Signal(syscall.SIGTERM)
	if err != nil {
		return fmt.Errorf("无法发送终止信号: %v", err)
	}

	// 等待进程退出
	done := make(chan error, 1)
	go func() {
		_, err := process.Wait()
		done <- err
	}()

	// 设置超时
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("等待进程退出时出错: %v", err)
		}
	case <-time.After(5 * time.Second):
		// 超时后尝试发送 SIGKILL
		process.Kill()
		return fmt.Errorf("进程未在预期时间内退出，已强制终止")
	}

	// 删除 PID 文件
	os.Remove(pidFilePath)

	return nil
}
