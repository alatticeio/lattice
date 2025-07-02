package client

import "sync"

type TurnManager struct {
	mu        sync.Mutex
	RelayInfo *RelayInfo
}

func (m *TurnManager) GetInfo() *RelayInfo {
	return m.RelayInfo
}

func (m *TurnManager) SetInfo(info *RelayInfo) {
	m.mu.Lock()
	m.RelayInfo = info
	m.mu.Unlock()
}
