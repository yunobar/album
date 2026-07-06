package authadapter

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/itsLeonB/go-authkit"
	"github.com/itsLeonB/go-crud"
	"github.com/yunobar/album/internal/domain/entity"
)

type sessionStoreAdapter struct {
	repo crud.Repository[entity.Session]
}

func NewSessionStore(repo crud.Repository[entity.Session]) authkit.SessionStore {
	return &sessionStoreAdapter{repo}
}

func (a *sessionStoreAdapter) Create(ctx context.Context, userID string) (authkit.Session, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return authkit.Session{}, err
	}

	session, err := a.repo.Insert(ctx, entity.Session{
		UserID:     uid,
		LastUsedAt: time.Now(),
	})
	if err != nil {
		return authkit.Session{}, err
	}
	return toAuthSession(session), nil
}

func (a *sessionStoreAdapter) GetByID(ctx context.Context, id string) (authkit.Session, error) {
	session, err := a.findByID(ctx, id)
	if err != nil {
		return authkit.Session{}, err
	}
	if session.IsZero() {
		return authkit.Session{}, authkit.ErrSessionNotFound
	}
	return toAuthSession(session), nil
}

func (a *sessionStoreAdapter) Delete(ctx context.Context, id string) error {
	session, err := a.findByID(ctx, id)
	if err != nil {
		return err
	}
	if session.IsZero() {
		return nil
	}
	return a.repo.Delete(ctx, session)
}

func (a *sessionStoreAdapter) Touch(ctx context.Context, id string) error {
	session, err := a.findByID(ctx, id)
	if err != nil {
		return err
	}
	if session.IsZero() {
		return nil
	}

	session.LastUsedAt = time.Now()
	_, err = a.repo.Update(ctx, session)
	return err
}

func (a *sessionStoreAdapter) findByID(ctx context.Context, id string) (entity.Session, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return entity.Session{}, err
	}

	spec := crud.Specification[entity.Session]{}
	spec.Model.ID = uid
	return a.repo.FindFirst(ctx, spec)
}

func toAuthSession(s entity.Session) authkit.Session {
	return authkit.Session{
		ID:     s.ID.String(),
		UserID: s.UserID.String(),
	}
}
