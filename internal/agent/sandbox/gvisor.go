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

package sandbox

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/go-logr/logr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	gvisorVersion = "latest"
	gvisorBaseURL = "https://storage.googleapis.com/gvisor/releases"
	defaultBinDir = ".lattice/bin"
	runscBinary   = "runsc"
)

// GVisorInstaller 负责安装和管理 gVisor
type GVisorInstaller struct {
	binDir string
	log    logr.Logger
}

// NewGVisorInstaller 创建一个新的 gVisor 安装器
func NewGVisorInstaller() *GVisorInstaller {
	home, _ := os.UserHomeDir()
	return &GVisorInstaller{
		binDir: filepath.Join(home, defaultBinDir),
		log:    logf.Log.WithName("gvisor-installer"),
	}
}

// NewGVisorInstallerWithDir 使用指定目录创建安装器
func NewGVisorInstallerWithDir(binDir string) *GVisorInstaller {
	return &GVisorInstaller{
		binDir: binDir,
		log:    logf.Log.WithName("gvisor-installer"),
	}
}

// IsInstalled 检查 gVisor 是否已安装
func (gi *GVisorInstaller) IsInstalled() bool {
	runscPath := gi.GetRunscPath()
	_, err := os.Stat(runscPath)
	return err == nil
}

// GetRunscPath 返回 runsc 二进制路径
func (gi *GVisorInstaller) GetRunscPath() string {
	return filepath.Join(gi.binDir, runscBinary)
}

// EnsureInstalled 确保 gVisor 已安装，如果未安装则自动下载
func (gi *GVisorInstaller) EnsureInstalled() (string, error) {
	if gi.IsInstalled() {
		gi.log.V(1).Info("gVisor already installed", "path", gi.GetRunscPath())
		return gi.GetRunscPath(), nil
	}

	gi.log.Info("Installing gVisor...")
	if err := gi.Install(); err != nil {
		return "", fmt.Errorf("install gVisor: %w", err)
	}

	return gi.GetRunscPath(), nil
}

// Install 下载并安装 gVisor
func (gi *GVisorInstaller) Install() error {
	// 创建 bin 目录
	if err := os.MkdirAll(gi.binDir, 0755); err != nil {
		return fmt.Errorf("create bin directory: %w", err)
	}

	// 下载 runsc
	url := gi.getDownloadURL()
	gi.log.Info("Downloading gVisor", "url", url)

	if err := gi.download(url, gi.GetRunscPath()); err != nil {
		return fmt.Errorf("download runsc: %w", err)
	}

	// 设置可执行权限
	if err := os.Chmod(gi.GetRunscPath(), 0755); err != nil {
		return fmt.Errorf("chmod runsc: %w", err)
	}

	gi.log.Info("gVisor installed successfully", "path", gi.GetRunscPath())
	return nil
}

// Uninstall 卸载 gVisor
func (gi *GVisorInstaller) Uninstall() error {
	if !gi.IsInstalled() {
		return nil
	}

	gi.log.Info("Uninstalling gVisor", "path", gi.GetRunscPath())
	return os.Remove(gi.GetRunscPath())
}

// GetVersion 获取已安装的 gVisor 版本
func (gi *GVisorInstaller) GetVersion() (string, error) {
	if !gi.IsInstalled() {
		return "", fmt.Errorf("gVisor not installed")
	}

	cmd := exec.Command(gi.GetRunscPath(), "version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("get gVisor version: %w", err)
	}

	return string(output), nil
}

// getDownloadURL 返回 gVisor 下载 URL
func (gi *GVisorInstaller) getDownloadURL() string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// gVisor 下载 URL 格式
	// https://storage.googleapis.com/gvisor/releases/release/latest/{os}/{arch}/runsc
	return fmt.Sprintf("%s/release/latest/%s/%s/runsc", gvisorBaseURL, goos, goarch)
}

// download 下载文件到指定路径
func (gi *GVisorInstaller) download(url, dest string) error {
	// 创建临时文件
	tmpFile, err := os.CreateTemp(filepath.Dir(dest), ".runsc-download-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	defer func() { _ = tmpFile.Close() }()

	// 下载文件
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: status %d", resp.StatusCode)
	}

	// 写入临时文件
	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return fmt.Errorf("write to temp file: %w", err)
	}

	// 关闭临时文件
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	// 重命名为目标文件
	if err := os.Rename(tmpFile.Name(), dest); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}
