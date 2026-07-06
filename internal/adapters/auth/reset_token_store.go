package authadapter

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/itsLeonB/go-authkit"
	"github.com/itsLeonB/go-crud"
	"github.com/yunobar/album/internal/domain/entity"
)

type resetTokenStoreAdapter struct {
	repo crud.Repository[entity.PasswordResetToken]
}

func NewResetTokenStore(repo crud.Repository[entity.PasswordResetToken]) authkit.ResetTokenStore {
	return &resetTokenStoreAdapter{repo}
}

func (a *resetTokenStoreAdapter) Create(ctx context.Context, userID, selector, verifierHash string, expiresAt time.Time) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}

	_, err = a.repo.Insert(ctx, entity.PasswordResetToken{
		UserID:       uid,
		Selector:     selector,
		VerifierHash: verifierHash,
		ExpiresAt:    expiresAt,
	})
	return err
}

func (a *resetTokenStoreAdapter) FindBySelector(ctx context.Context, selector string) (authkit.ResetToken, error) {
	spec := crud.Specification[entity.PasswordResetToken]{}
	spec.Model.Selector = selector
	rt, err := a.repo.FindFirst(ctx, spec)
	if err != nil {
		return authkit.ResetToken{}, err
	}
	if rt.IsZero() {
		return authkit.ResetToken{}, authkit.ErrTokenNotFound
	}
	return authkit.ResetToken{
		UserID:       rt.UserID.String(),
		Selector:     rt.Selector,
		VerifierHash: rt.VerifierHash,
		ExpiresAt:    rt.ExpiresAt,
	}, nil
}

func (a *resetTokenStoreAdapter) DeleteByUser(ctx context.Context, userID string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}

	spec := crud.Specification[entity.PasswordResetToken]{}
	spec.Model.UserID = uid
	tokens, err := a.repo.FindAll(ctx, spec)
	if err != nil {
		return err
	}
	if len(tokens) == 0 {
		return nil
	}
	return a.repo.DeleteMany(ctx, tokens)
}
