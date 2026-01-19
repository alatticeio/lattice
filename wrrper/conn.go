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

package wrrper

import (
	"bufio"
	"net"
)

// ReadWriterConn wrapper for missed data when hijack occur， for using Read/Write fn
type ReadWriterConn struct {
	net.Conn
	*bufio.ReadWriter
}

func (c *ReadWriterConn) Read(p []byte) (int, error) {
	return c.ReadWriter.Read(p)
}

func (c *ReadWriterConn) Write(p []byte) (int, error) {
	n, err := c.ReadWriter.Write(p)
	if err != nil {
		return n, err
	}
	// 确保数据立即发出，而不是留在 bufio 的写缓存里
	return n, c.ReadWriter.Flush()
}
