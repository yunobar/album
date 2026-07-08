package provider

import (
	"github.com/itsLeonB/go-crud"
	"github.com/yunobar/album/internal/domain/entity"
	"gorm.io/gorm"
)

type Repositories struct {
	Transactor crud.Transactor

	// Users
	User               crud.Repository[entity.User]
	Profile            crud.Repository[entity.UserProfile]
	PasswordResetToken crud.Repository[entity.PasswordResetToken]
	OAuthAccount       crud.Repository[entity.OAuthAccount]
	Session            crud.Repository[entity.Session]
	RefreshToken       crud.Repository[entity.RefreshToken]

	// Content
	Content crud.Repository[entity.Content]

	// Watchlist
	Watchlist crud.Repository[entity.WatchlistItem]
}

func ProvideRepositories(db *gorm.DB) *Repositories {
	return &Repositories{
		Transactor: crud.NewTransactor(db),

		User:               crud.NewRepository[entity.User](db),
		Profile:            crud.NewRepository[entity.UserProfile](db),
		PasswordResetToken: crud.NewRepository[entity.PasswordResetToken](db),
		OAuthAccount:       crud.NewRepository[entity.OAuthAccount](db),
		Session:            crud.NewRepository[entity.Session](db),
		RefreshToken:       crud.NewRepository[entity.RefreshToken](db),

		Content: crud.NewRepository[entity.Content](db),

		Watchlist: crud.NewRepository[entity.WatchlistItem](db),
	}
}
