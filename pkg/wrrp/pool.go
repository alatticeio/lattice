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

package wrrp

import "sync"

var payloadPool = sync.Pool{
	New: func() interface{} {
		// 申请一个足够大的缓冲区（比如符合 MTU 的 1600 字节）
		b := make([]byte, 2048)
		return &b
	},
}

func GetPayloadBuffer() *[]byte {
	return payloadPool.Get().(*[]byte)
}

func PutPayloadBuffer(buf *[]byte) {
	payloadPool.Put(buf)
}

var headerPool = sync.Pool{
	New: func() interface{} {
		//申请header pool size, 每次Marshal / UnMarshal时使用
		b := make([]byte, HeaderSize)
		//返回指针，防止发生内存逃逸
		return &b
	},
}

func GetHeaderBuffer() *[]byte {
	return headerPool.Get().(*[]byte)
}

func PutHeaderBuffer(buf *[]byte) {
	headerPool.Put(buf)
}
