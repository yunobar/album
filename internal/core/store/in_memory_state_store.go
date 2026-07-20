package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/itsLeonB/ungerr"
)

type stateEntry struct {
	value     string
	expiresAt time.Time
}

type inMemoryStateStore struct {
	data   *sync.Map
	stopCh chan struct{}
	wg     sync.WaitGroup
	once   sync.Once
}

func newInMemoryStateStore() *inMemoryStateStore {
	store := &inMemoryStateStore{
		data:   new(sync.Map),
		stopCh: make(chan struct{}),
	}
	store.startCleanup()
	return store
}

func (vss *inMemoryStateStore) Store(ctx context.Context, state string, value string, expiry time.Duration) error {
	key := vss.constructKey(state)
	entry := stateEntry{
		value:     value,
		expiresAt: time.Now().Add(expiry),
	}
	vss.data.Store(key, entry)
	return nil
}

func (vss *inMemoryStateStore) VerifyAndDelete(ctx context.Context, state string) (string, error) {
	key := vss.constructKey(state)
	raw, loaded := vss.data.LoadAndDelete(key)
	if !loaded {
		return "", ungerr.BadRequestError("invalid state")
	}

	entry := raw.(stateEntry)
	if time.Now().After(entry.expiresAt) {
		return "", ungerr.BadRequestError("invalid state")
	}

	return entry.value, nil
}

func (vss *inMemoryStateStore) startCleanup() {
	vss.wg.Go(func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				vss.cleanup()
			case <-vss.stopCh:
				return
			}
		}
	})
}

func (vss *inMemoryStateStore) cleanup() {
	now := time.Now()
	vss.data.Range(func(key, value any) bool {
		entry := value.(stateEntry)
		if now.After(entry.expiresAt) {
			vss.data.Delete(key)
		}
		return true
	})
}

func (vss *inMemoryStateStore) Shutdown() error {
	vss.once.Do(func() {
		close(vss.stopCh)
	})
	vss.wg.Wait()
	return nil
}

func (vss *inMemoryStateStore) constructKey(state string) string {
	return fmt.Sprintf("state:%s", state)
}
