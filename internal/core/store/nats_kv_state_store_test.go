package store

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/yunobar/album/internal/mocks"
)

func TestNATSKVStateStore_Store(t *testing.T) {
	kv := mocks.NewMockKeyValue(t)
	s := newNATSKVStateStore(kv)

	kv.On("Create", mock.Anything, "state.abc123", []byte("session-data"), mock.Anything).
		Return(uint64(1), nil)

	err := s.Store(context.Background(), "abc123", "session-data", 5*time.Minute)
	assert.NoError(t, err)
}

func TestNATSKVStateStore_Store_Duplicate(t *testing.T) {
	kv := mocks.NewMockKeyValue(t)
	s := newNATSKVStateStore(kv)

	kv.On("Create", mock.Anything, "state.abc123", []byte("session-data"), mock.Anything).
		Return(uint64(0), jetstream.ErrKeyExists)

	err := s.Store(context.Background(), "abc123", "session-data", 5*time.Minute)
	assert.Error(t, err)
}

type mockEntry struct {
	jetstream.KeyValueEntry
	revision uint64
	value    []byte
}

func (e *mockEntry) Revision() uint64 { return e.revision }
func (e *mockEntry) Value() []byte    { return e.value }

func TestNATSKVStateStore_VerifyAndDelete(t *testing.T) {
	kv := mocks.NewMockKeyValue(t)
	s := newNATSKVStateStore(kv)

	entry := &mockEntry{revision: 1, value: []byte("session-data")}
	kv.On("Get", mock.Anything, "state.abc123").Return(entry, nil)
	kv.On("Delete", mock.Anything, "state.abc123", mock.Anything).Return(nil)

	value, err := s.VerifyAndDelete(context.Background(), "abc123")
	assert.NoError(t, err)
	assert.Equal(t, "session-data", value)
}

func TestNATSKVStateStore_VerifyAndDelete_NotFound(t *testing.T) {
	kv := mocks.NewMockKeyValue(t)
	s := newNATSKVStateStore(kv)

	kv.On("Get", mock.Anything, "state.nonexistent").Return(nil, jetstream.ErrKeyNotFound)

	_, err := s.VerifyAndDelete(context.Background(), "nonexistent")
	assert.Error(t, err)
}

func TestNATSKVStateStore_Shutdown(t *testing.T) {
	kv := mocks.NewMockKeyValue(t)
	s := newNATSKVStateStore(kv)

	assert.NoError(t, s.Shutdown())
}
