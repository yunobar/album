package authadapter

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/itsLeonB/go-authkit"
	"github.com/itsLeonB/go-crud"
	"github.com/yunobar/album/internal/domain/entity"
)

type refreshTokenStoreAdapter struct {
	repo crud.Repository[entity.RefreshToken]
}

func NewRefreshTokenStore(repo crud.Repository[entity.RefreshToken]) authkit.RefreshTokenStore {
	return &refreshTokenStoreAdapter{repo}
}

func (a *refreshTokenStoreAdapter) Create(ctx context.Context, sessionID, tokenHash string, expiresAt time.Time) error {
	sid, err := uuid.Parse(sessionID)
	if err != nil {
		return err
	}

	_, err = a.repo.Insert(ctx, entity.RefreshToken{
		SessionID: sid,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	})
	return err
}

func (a *refreshTokenStoreAdapter) FindByHash(ctx context.Context, hash string) (authkit.RefreshToken, error) {
	spec := crud.Specification[entity.RefreshToken]{}
	spec.Model.TokenHash = hash
	rt, err := a.repo.FindFirst(ctx, spec)
	if err != nil {
		return authkit.RefreshToken{}, err
	}
	if rt.IsZero() {
		return authkit.RefreshToken{}, authkit.ErrTokenNotFound
	}
	return toAuthRefreshToken(rt), nil
}

func (a *refreshTokenStoreAdapter) Delete(ctx context.Context, sessionID string, tokenHash string) error {
	sid, err := uuid.Parse(sessionID)
	if err != nil {
		return err
	}

	spec := crud.Specification[entity.RefreshToken]{}
	spec.Model.SessionID = sid
	spec.Model.TokenHash = tokenHash
	rt, err := a.repo.FindFirst(ctx, spec)
	if err != nil {
		return err
	}
	if rt.IsZero() {
		return nil
	}
	return a.repo.Delete(ctx, rt)
}

func (a *refreshTokenStoreAdapter) DeleteBySession(ctx context.Context, sessionID string) error {
	sid, err := uuid.Parse(sessionID)
	if err != nil {
		return err
	}

	spec := crud.Specification[entity.RefreshToken]{}
	spec.Model.SessionID = sid
	tokens, err := a.repo.FindAll(ctx, spec)
	if err != nil {
		return err
	}
	if len(tokens) == 0 {
		return nil
	}
	return a.repo.DeleteMany(ctx, tokens)
}

func toAuthRefreshToken(rt entity.RefreshToken) authkit.RefreshToken {
	return authkit.RefreshToken{
		ID:        rt.ID.String(),
		SessionID: rt.SessionID.String(),
		TokenHash: rt.TokenHash,
		ExpiresAt: rt.ExpiresAt,
		CreatedAt: rt.CreatedAt,
	}
}
