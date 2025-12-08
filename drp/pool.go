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

package drp

import (
	"sync"
	"wireflow/internal/grpc"
)

// MessageManager 处理DrpMessage对象池的管理
type MessageManager struct {
	pool sync.Pool
}

// NewMessageManager 创建新的消息管理器实例
func NewMessageManager() *MessageManager {
	return &MessageManager{
		pool: sync.Pool{
			New: func() interface{} {
				return &grpc.DrpMessage{
					Body: make([]byte, 0, 32*1024),
				}
			},
		},
	}
}

// GetMessage 从对象池获取消息
func (m *MessageManager) GetMessage() *grpc.DrpMessage {
	return m.pool.Get().(*grpc.DrpMessage)
}

// ReleaseMessage 重置消息并返回到对象池
func (m *MessageManager) ReleaseMessage(msg *grpc.DrpMessage) {
	if msg == nil {
		return
	}
	m.resetMessage(msg)
	m.pool.Put(msg)
}

// resetMessage 重置消息的所有字段
func (m *MessageManager) resetMessage(msg *grpc.DrpMessage) {
	msg.Body = nil
	msg.From = ""
	msg.To = ""
	msg.Encrypt = 0
	msg.Version = 0
	msg.MsgType = grpc.MessageType_MessageDirectOfferType
}

// GetMessageFromPool 获取消息的新方法
func (p *Proxy) GetMessageFromPool() *grpc.DrpMessage {
	return p.msgManager.GetMessage()
}

// PutMessageToPool 释放消息的新方法
func (p *Proxy) PutMessageToPool(msg *grpc.DrpMessage) {
	p.msgManager.ReleaseMessage(msg)
}
