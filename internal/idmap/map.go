// Package idmap maps gateway-assigned request IDs to backend server request IDs.
package idmap

import (
	"context"
	"time"
)

// Map stores and retrieves request ID mappings.
type Map interface {
	// Store a new id mapping, returning the downstream gateway id
	Store(ctx context.Context, backendID any, serverName string, sessionID string) (string, error)
	// Lookup gets an entry for a gateway id without removing it.
	// Callers must call Remove explicitly after successful processing.
	Lookup(ctx context.Context, gatewayID string) (Entry, bool, error)
	// Remove is explicit best-effort removal for a gateway id
	Remove(ctx context.Context, gatewayID string)
}

// Entry holds a backend request ID and its associated server/session info.
type Entry struct {
	BackendID  any    `json:"backendID"` // per mcp spec, the ID can be string, int64, or float64
	ServerName string `json:"serverName"`
	SessionID  string `json:"sessionID"`
}

type mapConfig struct {
	connectionString string
	entryTTL         time.Duration
}

// New returns an initialized Map. When WithConnectionString is provided, the
// returned Map is backed by Redis; otherwise it uses an in-memory store.
func New(ctx context.Context, opts ...func(*mapConfig)) (Map, error) {
	cfg := &mapConfig{}
	for _, o := range opts {
		o(cfg)
	}
	if cfg.connectionString != "" {
		return newRedisMap(ctx, cfg.connectionString, cfg.entryTTL)
	}
	return newInMemoryMap(), nil
}

// WithConnectionString configures the Map to use a Redis backend.
// Format: redis://<user>:<pass>@localhost:6379/<db>
func WithConnectionString(url string) func(*mapConfig) {
	return func(c *mapConfig) {
		c.connectionString = url
	}
}

// WithEntryTTL sets the safety-net TTL for Redis-backed entries.
// Only applies when a Redis connection string is configured.
func WithEntryTTL(ttl time.Duration) func(*mapConfig) {
	return func(c *mapConfig) {
		c.entryTTL = ttl
	}
}
