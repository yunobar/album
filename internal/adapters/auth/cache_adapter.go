package authadapter

import (
	"github.com/google/uuid"
	"github.com/itsLeonB/go-authkit"
	"github.com/yunobar/album/internal/core/cache"
)

type sessionCacheAdapter struct {
	inner cache.Cache[uuid.UUID]
}

func NewSessionCacheAdapter(inner cache.Cache[uuid.UUID]) authkit.SessionCache {
	return &sessionCacheAdapter{inner}
}

func (a *sessionCacheAdapter) Get(sessionID string, loader func(string) (string, bool)) (string, bool) {
	userID, hit := a.inner.Get(sessionID, func(sid string) (uuid.UUID, bool) {
		userIDStr, ok := loader(sid)
		if !ok {
			return uuid.Nil, false
		}
		uid, err := uuid.Parse(userIDStr)
		if err != nil {
			return uuid.Nil, false
		}
		return uid, true
	})
	if !hit {
		return "", false
	}
	return userID.String(), true
}

func (a *sessionCacheAdapter) Delete(sessionID string) {
	a.inner.Delete(sessionID)
}

func (a *sessionCacheAdapter) Shutdown() error {
	return a.inner.Shutdown()
}
