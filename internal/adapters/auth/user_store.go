package authadapter

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/itsLeonB/go-authkit"
	"github.com/itsLeonB/go-crud"
	"github.com/yunobar/album/internal/domain/dto"
	"github.com/yunobar/album/internal/domain/entity"
	"github.com/yunobar/album/internal/domain/service"
)

type userStoreAdapter struct {
	transactor crud.Transactor
	userRepo   crud.Repository[entity.User]
	profileSvc service.ProfileService
}

func NewUserStore(transactor crud.Transactor, userRepo crud.Repository[entity.User], profileSvc service.ProfileService) authkit.UserStore {
	return &userStoreAdapter{transactor, userRepo, profileSvc}
}

func (a *userStoreAdapter) FindByID(ctx context.Context, userID string) (authkit.User, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return authkit.User{}, err
	}
	spec := crud.Specification[entity.User]{}
	spec.Model.ID = uid
	user, err := a.userRepo.FindFirst(ctx, spec)
	if err != nil {
		return authkit.User{}, err
	}
	if user.IsZero() {
		return authkit.User{}, authkit.ErrUserNotFound
	}
	return toAuthUser(user), nil
}

func (a *userStoreAdapter) FindByEmail(ctx context.Context, email string) (authkit.User, error) {
	spec := crud.Specification[entity.User]{}
	spec.Model.Email = email
	user, err := a.userRepo.FindFirst(ctx, spec)
	if err != nil {
		return authkit.User{}, err
	}
	if user.IsZero() {
		return authkit.User{}, authkit.ErrUserNotFound
	}
	return toAuthUser(user), nil
}

func (a *userStoreAdapter) Create(ctx context.Context, email, passwordHash string) (authkit.User, error) {
	user, err := a.userRepo.Insert(ctx, entity.User{
		Email:        email,
		PasswordHash: passwordHash,
	})
	if err != nil {
		return authkit.User{}, err
	}
	return toAuthUser(user), nil
}

func (a *userStoreAdapter) CreateOAuth(ctx context.Context, email, name, avatar string) (authkit.User, error) {
	var user entity.User

	err := a.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		var err error
		user, err = a.userRepo.Insert(ctx, entity.User{
			Email: email,
			VerifiedAt: sql.NullTime{
				Time:  time.Now(),
				Valid: true,
			},
		})
		if err != nil {
			return err
		}

		_, err = a.profileSvc.Create(ctx, dto.NewProfileRequest{
			UserID: user.ID,
			Name:   name,
			Avatar: avatar,
		})
		return err
	})
	if err != nil {
		return authkit.User{}, err
	}

	return toAuthUser(user), nil
}

func (a *userStoreAdapter) SetVerified(ctx context.Context, userID string, name, avatar string) (authkit.User, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return authkit.User{}, err
	}

	var updated entity.User

	err = a.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		spec := crud.Specification[entity.User]{}
		spec.Model.ID = uid
		spec.PreloadRelations = []string{"Profile"}
		user, err := a.userRepo.FindFirst(ctx, spec)
		if err != nil {
			return err
		}
		if user.IsZero() {
			return authkit.ErrUserNotFound
		}

		if user.Profile.IsZero() {
			if _, err = a.profileSvc.Create(ctx, dto.NewProfileRequest{
				UserID: user.ID,
				Name:   name,
				Avatar: avatar,
			}); err != nil {
				return err
			}
			user, err = a.userRepo.FindFirst(ctx, spec)
			if err != nil {
				return err
			}
		}

		user.VerifiedAt = sql.NullTime{
			Time:  time.Now(),
			Valid: true,
		}

		updated, err = a.userRepo.Update(ctx, user)
		return err
	})
	if err != nil {
		return authkit.User{}, err
	}

	return toAuthUser(updated), nil
}

func (a *userStoreAdapter) Exists(ctx context.Context, userID string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}

	spec := crud.Specification[entity.User]{}
	spec.Model.ID = uid
	user, err := a.userRepo.FindFirst(ctx, spec)
	if err != nil {
		return err
	}
	if user.IsZero() {
		return authkit.ErrUserNotFound
	}
	return nil
}

func (a *userStoreAdapter) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}

	spec := crud.Specification[entity.User]{}
	spec.Model.ID = uid
	u, err := a.userRepo.FindFirst(ctx, spec)
	if err != nil {
		return err
	}
	if u.IsZero() {
		return authkit.ErrUserNotFound
	}

	u.PasswordHash = passwordHash
	_, err = a.userRepo.Update(ctx, u)
	return err
}

func toAuthUser(u entity.User) authkit.User {
	profileID := ""
	if !u.Profile.IsZero() {
		profileID = u.Profile.ID.String()
	}
	return authkit.User{
		ID:           u.ID.String(),
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		Verified:     u.IsVerified(),
		ProfileID:    profileID,
	}
}
