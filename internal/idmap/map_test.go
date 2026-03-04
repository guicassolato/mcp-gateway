package idmap

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStoreLookup(t *testing.T) {
	tests := []struct {
		name    string
		entries []struct {
			backendID  any
			serverName string
			sessionID  string
		}
		expectFound   bool
		expectDeleted bool
	}{
		{
			name: "string backend ID",
			entries: []struct {
				backendID  any
				serverName string
				sessionID  string
			}{
				{backendID: "req-42", serverName: "weather-server", sessionID: "upstream-session-1"},
			},
			expectFound: true,
		},
		{
			name: "int64 backend ID",
			entries: []struct {
				backendID  any
				serverName string
				sessionID  string
			}{
				{backendID: int64(42), serverName: "time-server", sessionID: "upstream-session-2"},
			},
			expectFound: true,
		},
		{
			name: "float64 backend ID",
			entries: []struct {
				backendID  any
				serverName string
				sessionID  string
			}{
				{backendID: float64(3.14), serverName: "calc-server", sessionID: "upstream-session-3"},
			},
			expectFound: true,
		},
		{
			name:        "lookup miss returns zero entry",
			entries:     nil,
			expectFound: false,
		},
		{
			name: "lookup deletes entry",
			entries: []struct {
				backendID  any
				serverName string
				sessionID  string
			}{
				{backendID: "req-1", serverName: "server1", sessionID: "session-1"},
			},
			expectFound:   true,
			expectDeleted: true,
		},
		{
			name: "multiple entries are independent",
			entries: []struct {
				backendID  any
				serverName string
				sessionID  string
			}{
				{backendID: "req-1", serverName: "server1", sessionID: "session-1"},
				{backendID: int64(2), serverName: "server2", sessionID: "session-2"},
				{backendID: float64(3.0), serverName: "server3", sessionID: "session-3"},
			},
			expectFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New()

			if len(tt.entries) == 0 {
				entry, ok := m.Lookup("nonexistent")
				require.False(t, ok)
				require.Equal(t, Entry{}, entry)
				return
			}

			ids := make([]string, len(tt.entries))
			for i, e := range tt.entries {
				ids[i] = m.Store(e.backendID, e.serverName, e.sessionID)
				require.NotEmpty(t, ids[i])
			}

			// verify unique IDs
			for i := range ids {
				for j := i + 1; j < len(ids); j++ {
					require.NotEqual(t, ids[i], ids[j])
				}
			}

			for i, id := range ids {
				entry, ok := m.Lookup(id)
				require.Equal(t, tt.expectFound, ok)

				if tt.expectFound {
					require.IsType(t, tt.entries[i].backendID, entry.BackendID)
					require.Equal(t, tt.entries[i].backendID, entry.BackendID)
					require.Equal(t, tt.entries[i].serverName, entry.ServerName)
					require.Equal(t, tt.entries[i].sessionID, entry.SessionID)
				}
			}

			if tt.expectDeleted {
				for _, id := range ids {
					_, ok := m.Lookup(id)
					require.False(t, ok)
				}
			}
		})
	}
}

func TestRemove(t *testing.T) {
	tests := []struct {
		name    string
		storeID bool
	}{
		{
			name:    "removes existing entry",
			storeID: true,
		},
		{
			name:    "no-op for unknown ID",
			storeID: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New()

			id := "nonexistent"
			if tt.storeID {
				id = m.Store("req-1", "server1", "session-1")
			}

			m.Remove(id)

			_, ok := m.Lookup(id)
			require.False(t, ok)
		})
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := New()
	var wg sync.WaitGroup

	ids := make([]string, 100)
	for i := range ids {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ids[i] = m.Store(int64(i), "server", "session")
		}(i)
	}
	wg.Wait()

	// even indices: lookup and verify values; odd indices: remove
	results := make([]Entry, len(ids))
	found := make([]bool, len(ids))
	for i, id := range ids {
		wg.Add(1)
		if i%2 == 0 {
			go func(i int, id string) {
				defer wg.Done()
				results[i], found[i] = m.Lookup(id)
			}(i, id)
		} else {
			go func(id string) {
				defer wg.Done()
				m.Remove(id)
			}(id)
		}
	}
	wg.Wait()

	for i := 0; i < len(ids); i += 2 {
		require.True(t, found[i], "entry %d should be found", i)
		require.Equal(t, int64(i), results[i].BackendID)
		require.Equal(t, "server", results[i].ServerName)
		require.Equal(t, "session", results[i].SessionID)
	}
}
