package store

import (
	"context"
	"time"

	"github.com/itsLeonB/ungerr"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/yunobar/album/internal/core/config"
)

type StateStore interface {
	Store(ctx context.Context, state string, value string, expiry time.Duration) error
	VerifyAndDelete(ctx context.Context, state string) (string, error)
	Shutdown() error
}

func NewStateStore(js jetstream.JetStream) (StateStore, error) {
	switch config.Global.StateStore {
	case "nats":
		ctx := context.Background()
		kv, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
			Bucket:         config.Global.StateStoreBucket,
			History:        1,
			LimitMarkerTTL: 10 * time.Minute,
		})
		if err != nil {
			return nil, ungerr.Wrap(err, "error creating NATS KV state store bucket")
		}
		return newNATSKVStateStore(kv), nil
	case "inmemory":
		return newInMemoryStateStore(), nil
	default:
		return nil, ungerr.Unknownf("unsupported AUTH_STATE_STORE value: %q", config.Global.StateStore)
	}
}
