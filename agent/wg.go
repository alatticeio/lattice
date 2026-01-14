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

package agent

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"syscall"
	"wireflow/internal/log"
	"wireflow/internal/wferrors"

	wg "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
)

type DeviceManager struct {
	logger *log.Logger
	device *wg.Device
}

func NewDeviceManager(logger *log.Logger, device *wg.Device) *DeviceManager {
	return &DeviceManager{logger: logger, device: device}
}

func (c *DeviceManager) IpcHandle(socket net.Conn) {
	defer socket.Close()

	buffered := func(s io.ReadWriter) *bufio.ReadWriter {
		reader := bufio.NewReader(s)
		writer := bufio.NewWriter(s)
		return bufio.NewReadWriter(reader, writer)
	}(socket)
	for {
		op, err := buffered.ReadString('\n')
		if err != nil {
			return
		}

		// handle operation
		switch op {
		case "stop\n":
			buffered.Write([]byte("OK\n\n"))
			// send kill signal
			syscall.Kill(os.Getpid(), syscall.SIGTERM)
		case "set=1\n":
			err = c.device.IpcSetOperation(buffered.Reader)
		case "get=1\n":
			var nextByte byte
			nextByte, err = buffered.ReadByte()
			if err != nil {
				return
			}
			if nextByte != '\n' {
				err = wferrors.IpcErrorf(ipc.IpcErrorInvalid, "trailing character in UAPI get: %q", nextByte)
				break
			}
			err = c.device.IpcGetOperation(buffered.Writer)
		default:
			c.logger.Error("invalid UAPI operation", errors.New("set error"), "op", op)
			return
		}

		// write status
		var status *wferrors.IPCError
		if err != nil && !errors.As(err, &status) {
			// shouldn't happen
			status = wferrors.IpcErrorf(ipc.IpcErrorUnknown, "other UAPI error: %w", err)
		}
		if status != nil {
			c.logger.Error("status", status)
			fmt.Fprintf(buffered, "errno=%d\n\n", status.ErrorCode())
		} else {
			fmt.Fprintf(buffered, "errno=0\n\n")
		}
		buffered.Flush()
	}

}
