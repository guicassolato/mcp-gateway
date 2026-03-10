package idmap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	redis "github.com/redis/go-redis/v9"
)

const (
	keyPrefix = "elicitation:"
	// safety-net TTL for entries that are not cleaned up via Flush (e.g. process crash)
	entryTTL = 1 * time.Hour
)

type redisMap struct {
	client *redis.Client
}

func newRedisMap(ctx context.Context, connectionString string) (*redisMap, error) {
	opt, err := redis.ParseURL(connectionString)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opt)
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &redisMap{client: client}, nil
}

func (m *redisMap) Store(ctx context.Context, backendID any, serverName string, sessionID string) (string, error) {
	id := uuid.NewString()

	entry := Entry{
		BackendID:  backendID,
		ServerName: serverName,
		SessionID:  sessionID,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return "", fmt.Errorf("marshal elicitation entry: %w", err)
	}

	if err := m.client.Set(ctx, keyPrefix+id, data, entryTTL).Err(); err != nil {
		return "", fmt.Errorf("store elicitation entry: %w", err)
	}

	return id, nil
}

func (m *redisMap) Lookup(ctx context.Context, gatewayID string) (Entry, bool, error) {
	data, err := m.client.GetDel(ctx, keyPrefix+gatewayID).Bytes()
	if errors.Is(err, redis.Nil) {
		return Entry{}, false, nil
	}
	if err != nil {
		return Entry{}, false, fmt.Errorf("lookup elicitation entry: %w", err)
	}

	var entry Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		return Entry{}, false, fmt.Errorf("unmarshal elicitation entry: %w", err)
	}

	return entry, true, nil
}

func (m *redisMap) Remove(ctx context.Context, gatewayID string) {
	// best-effort: errors are not propagated
	m.client.Del(ctx, keyPrefix+gatewayID)
}
