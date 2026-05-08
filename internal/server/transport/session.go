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

// Add registers a new session (typically called after NATS negotiation completes)
func (m *SessionManager) Add(pubKey [32]byte, sid uint64) {
	m.idToKey.Store(sid, pubKey)
	m.keyToId.Store(pubKey, sid)
}

// Remove deletes a session
func (m *SessionManager) Remove(pubKey [32]byte, sid uint64) {
	m.idToKey.Delete(sid)
	m.keyToId.Delete(pubKey)
}

func GenerateSessionID() (uint64, error) {
	var b [8]byte
	// Read fills b from the system's cryptographically secure random source
	_, err := rand.Read(b[:])
	if err != nil {
		return 0, fmt.Errorf("failed to generate random session id: %w", err)
	}
	// Convert 8 bytes to uint64
	return binary.BigEndian.Uint64(b[:]), nil
}
