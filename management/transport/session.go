package transport

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"sync"
)

type SessionManager struct {
	// FromID -> PublicKey
	idToKey sync.Map // map[uint64][32]byte
	// PublicKey -> FromID
	keyToId sync.Map // map[[32]byte]uint64
}

// 注册新会话（通常在 NATS 协商完成后调用）
func (m *SessionManager) Add(pubKey [32]byte, sid uint64) {
	m.idToKey.Store(sid, pubKey)
	m.keyToId.Store(pubKey, sid)
}

// 删除会话
func (m *SessionManager) Remove(pubKey [32]byte, sid uint64) {
	m.idToKey.Delete(sid)
	m.keyToId.Delete(pubKey)
}

func GenerateSessionID() (uint64, error) {
	var b [8]byte
	// Read 会从系统加密安全随机源填充 b
	_, err := rand.Read(b[:])
	if err != nil {
		return 0, fmt.Errorf("failed to generate random session id: %w", err)
	}
	// 将 8 字节转换为 uint64
	return binary.BigEndian.Uint64(b[:]), nil
}
