// Package idmap maps gateway-assigned request IDs to backend server request IDs.
package idmap

import (
	"sync"

	"github.com/google/uuid"
)

// Map stores and retrieves request ID mappings.
type Map interface {
	// Store a new id mapping, returning the downstream gateway id
	Store(backendID any, serverName string, sessionID string) string
	// Lookup gets an entry for a gateway id, deleting the entry in the map
	// This is done as once there has been an elicitation response, we don't want the request entry anymore
	Lookup(gatewayID string) (Entry, bool)
	// Explicit removal for a gateway id
	Remove(gatewayID string)
}

// Entry holds a backend request ID and its associated server/session info.
type Entry struct {
	BackendID  any // per mcp spec, the ID can be string, int64, or float64
	ServerName string
	SessionID  string
}

type idmap struct {
	mu      sync.Mutex
	entries map[string]Entry
}

// New returns an initialized Map.
func New() Map {
	return &idmap{entries: make(map[string]Entry)}
}

func (m *idmap) Store(backendID any, serverName string, sessionID string) string {
	id := uuid.NewString()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.entries[id] = Entry{
		BackendID:  backendID,
		ServerName: serverName,
		SessionID:  sessionID,
	}

	return id
}

func (m *idmap) Lookup(gatewayID string) (Entry, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.entries[gatewayID]
	if !ok {
		return Entry{}, false
	}

	delete(m.entries, gatewayID)

	return entry, ok
}

func (m *idmap) Remove(gatewayID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.entries, gatewayID)
}
