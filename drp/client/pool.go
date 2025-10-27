package client

import (
	"sync"
	"wireflow/drp/grpc"
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
