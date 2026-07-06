package authadapter

import (
	"context"

	"github.com/google/uuid"
	"github.com/itsLeonB/go-authkit"
	"github.com/itsLeonB/go-crud"
	"github.com/yunobar/album/internal/domain/entity"
)

type oauthAccountStoreAdapter struct {
	repo crud.Repository[entity.OAuthAccount]
}

func NewOAuthAccountStore(repo crud.Repository[entity.OAuthAccount]) authkit.OAuthAccountStore {
	return &oauthAccountStoreAdapter{repo}
}

func (a *oauthAccountStoreAdapter) FindByProvider(ctx context.Context, provider, providerID string) (authkit.OAuthAccount, error) {
	spec := crud.Specification[entity.OAuthAccount]{}
	spec.Model.Provider = provider
	spec.Model.ProviderID = providerID
	spec.PreloadRelations = []string{"User"}
	account, err := a.repo.FindFirst(ctx, spec)
	if err != nil {
		return authkit.OAuthAccount{}, err
	}
	if account.IsZero() {
		return authkit.OAuthAccount{}, authkit.ErrUserNotFound
	}
	return authkit.OAuthAccount{
		UserID:     account.UserID.String(),
		Provider:   account.Provider,
		ProviderID: account.ProviderID,
		Email:      account.Email,
	}, nil
}

func (a *oauthAccountStoreAdapter) Link(ctx context.Context, userID, provider, providerID, email string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	_, err = a.repo.Insert(ctx, entity.OAuthAccount{
		UserID:     uid,
		Provider:   provider,
		ProviderID: providerID,
		Email:      email,
	})
	return err
}
