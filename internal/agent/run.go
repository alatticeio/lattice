// Copyright 2026 The Lattice Authors, Inc.
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
	"errors"
	"fmt"
	"github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/wireguard"
	"github.com/alatticeio/lattice/internal/dns"
	"github.com/alatticeio/lattice/internal/telemetry"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.zx2c4.com/wireguard/ipc"
	"golang.zx2c4.com/wireguard/wgctrl"
)

// Start start lattice
// nolint:all
func Start(ctx context.Context, flags *config.Config) error {
	log.SetLevel(flags.Level)
	logger := log.GetLogger("lattice")

	if flags.EnableDaemon && os.Getenv("LATTICE_DAEMON") == "" {
		return startDaemon(flags, logger)
	}

	agentCfg := &NodeConfig{
		Logger:        logger,
		Port:          flags.WgPort,
		InterfaceName: flags.InterfaceName,
		Token:         flags.Token,
		ShowLog:       flags.EnableSysLog,
		Flags:         flags,
	}

	// 写 PID 文件，让 lattice stop 能发 SIGTERM
	pidPath := pidFilePath(flags.InterfaceName)
	if err := writePIDFile(pidPath); err != nil {
		logger.Warn("failed to write PID file", "err", err)
	} else {
		defer os.Remove(pidPath)
	}

	g, gCtx := errgroup.WithContext(ctx)

	if flags.EnableDNS {
		go func() {
			nativeDNS := dns.NewNativeDNS(&dns.DNSConfig{})
			if err := nativeDNS.Start(); err != nil {
				logger.Error("DNS start failed", err)
			}
		}()
	}

	c, err := NewNode(gCtx, agentCfg)
	if err != nil {
		return err
	}

	c.GetNetworkMap = func() (*infra.Message, error) {
		msg, err := c.ctrClient.GetNetMap(flags.Token)
		if err != nil {
			logger.Error("get network map failed", err)
			return nil, err
		}
		return msg, nil
	}

	if err = c.Start(gCtx); err != nil {
		return err
	}

	// Start heartbeat so the management server can track online status.
	go c.StartHeartbeat(gCtx)

	logger.Debug("Interface name", "name", c.Name)

	if flags.EnableMetric && flags.Telemetry.VMEndpoint != "" {
		tc := telemetry.Config{
			VMEndpoint: flags.Telemetry.VMEndpoint,
			Interval:   time.Duration(flags.Telemetry.IntervalSeconds) * time.Second,
		}
		collector, err := telemetry.New(tc, c.GetPeerManager(), logger)
		if err != nil {
			logger.Warn("telemetry init failed, skipping", "err", err)
		} else {
			// Resolve NetworkID after workspace config is applied; fall back to AppId namespace.
			networkID := ""
			if c.current != nil {
				networkID = c.current.NetworkId
			}
			collector.SetIdentity(telemetry.Identity{
				PeerID:    flags.AppId,
				NetworkID: networkID,
				Interface: c.GetDeviceName(),
			})
			g.Go(func() error { return collector.Run(gCtx) })
		}
	}

	fileUAPI, err := ipc.UAPIOpen(c.Name)
	if err != nil {
		return fmt.Errorf("failed to open UAPI socket: %w", err)
	}

	uapi, err := ipc.UAPIListen(c.Name, fileUAPI)
	if err != nil {
		return fmt.Errorf("failed to listen on UAPI socket: %w", err)
	}

	g.Go(func() error {
		go func() {
			<-gCtx.Done()
			uapi.Close()
		}()

		for {
			conn, err := uapi.Accept()
			if err != nil {
				select {
				case <-gCtx.Done():
					return gCtx.Err()
				default:
					return fmt.Errorf("ipc accept error: %w", err)
				}
			}
			go func(nc net.Conn) {
				defer nc.Close()
				c.DeviceManager.IpcHandle(nc)
			}(conn)
		}
	})

	logger.Info("lattice started")

	if err = g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("lattice exited with error", err)
	}

	if stopErr := c.Stop(); stopErr != nil {
		logger.Warn("lattice stop error", "err", stopErr)
	}
	logger.Info("lattice shutting down")

	return nil
}

// startDaemon forks the current process as a background daemon and exits the parent.
func startDaemon(flags *config.Config, logger *log.Logger) error {
	fmt.Println("Run lattice in daemon mode")

	var logDir string
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		logDir = filepath.Join(home, "Library/Logs/lattice")
	case "windows":
		logDir = `C:\ProgramData\lattice\logs`
	default:
		logDir = "/var/log/lattice"
	}

	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	var stdout, stderr *os.File
	if flags.Level != "" && flags.Level != "silent" {
		f, err := os.OpenFile(filepath.Join(logDir, "lattice.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		stdout, stderr = f, f
	} else {
		devNull, _ := os.Open(os.DevNull)
		stdout, stderr = devNull, devNull
	}

	devNull, _ := os.Open(os.DevNull)
	attr := &os.ProcAttr{
		Files: []*os.File{devNull, stdout, stderr},
		Dir:   ".",
		Env:   append(os.Environ(), "LATTICE_DAEMON=true"),
	}

	path, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to determine executable: %w", err)
	}

	var filteredArgs []string
	for _, arg := range os.Args {
		if arg != "--daemon" && arg != "-d" {
			filteredArgs = append(filteredArgs, arg)
		}
	}

	process, err := os.StartProcess(path, filteredArgs, attr)
	if err != nil {
		return fmt.Errorf("failed to daemonize: %w", err)
	}
	if err = process.Release(); err != nil {
		return fmt.Errorf("failed to release process: %w", err)
	}

	logger.Info("daemon started", "pid", process.Pid)
	os.Exit(0)
	return nil // unreachable
}

// Stop sends SIGTERM to the running lattice daemon via its PID file.
func Stop(flags *config.Config) error {
	interfaceName := flags.InterfaceName
	if interfaceName == "" {
		ctr, err := wgctrl.New()
		if err != nil {
			return err
		}
		ifaces, err := ctr.Devices()
		if err != nil {
			return err
		}
		if len(ifaces) == 0 {
			return fmt.Errorf("lattice daemon is not running, no interfaces found")
		}
		interfaceName = ifaces[0].Name
	}

	pidPath := pidFilePath(interfaceName)
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return fmt.Errorf("failed to read PID file %s: %w (is lattice running?)", pidPath, err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return fmt.Errorf("invalid PID in %s: %w", pidPath, err)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("process %d not found: %w", pid, err)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM to PID %d: %w", pid, err)
	}

	fmt.Printf("sent SIGTERM to lattice daemon (interface: %s, PID: %d)\n", interfaceName, pid)
	return nil
}

func Status(flags *config.Config) error {
	return wireguard.PrintStatus(flags.InterfaceName)
}

func pidFilePath(iface string) string {
	return fmt.Sprintf("/var/run/wireguard/%s.pid", iface)
}

func writePIDFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0644)
}
