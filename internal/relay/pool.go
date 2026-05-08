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

package relay

import "sync"

var payloadPool = sync.Pool{
	New: func() interface{} {
		// allocate a large enough buffer (e.g. 1600 bytes to fit MTU)
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
		// allocate header pool buffer, used each time Marshal/Unmarshal is called
		b := make([]byte, HeaderSize)
		// return pointer to prevent memory escape to heap
		return &b
	},
}

func GetHeaderBuffer() *[]byte {
	return headerPool.Get().(*[]byte)
}

func PutHeaderBuffer(buf *[]byte) {
	headerPool.Put(buf)
}
