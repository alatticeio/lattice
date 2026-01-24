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

package transport

import (
	"net"
	"time"
	"wireflow/internal/infra"
)

type WrrpRawConn struct {
	wrrp       infra.Wrrp
	remoteAddr net.Addr
}

func (conn *WrrpRawConn) Write(b []byte) (n int, err error) {
	//TODO implement me
	panic("implement me")
}

func (conn *WrrpRawConn) Close() error {
	//TODO implement me
	panic("implement me")
}

func (conn *WrrpRawConn) LocalAddr() net.Addr {
	//TODO implement me
	panic("implement me")
}

func (conn *WrrpRawConn) RemoteAddr() net.Addr {
	return conn.wrrp.RemoteAddr()
}

func (conn *WrrpRawConn) SetDeadline(t time.Time) error {
	//TODO implement me
	panic("implement me")
}

func (conn *WrrpRawConn) SetReadDeadline(t time.Time) error {
	//TODO implement me
	panic("implement me")
}

func (conn *WrrpRawConn) SetWriteDeadline(t time.Time) error {
	//TODO implement me
	panic("implement me")
}

func (conn *WrrpRawConn) Read(buf []byte) (n int, err error) {
	return
}
