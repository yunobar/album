package provider

import (
	"github.com/itsLeonB/go-crud"
	"github.com/yunobar/album/internal/domain/entity"
	"github.com/yunobar/album/internal/domain/repository"
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

	// Groups
	Group       repository.GroupRepository
	GroupMember crud.Repository[entity.GroupMember]

	// Decision Engine
	DecisionSession         crud.Repository[entity.DecisionSession]
	SessionParticipant      crud.Repository[entity.SessionParticipant]
	SessionCandidate        crud.Repository[entity.SessionCandidate]
	SessionVote             crud.Repository[entity.SessionVote]
	SessionRanking          crud.Repository[entity.SessionRanking]
	SessionPrioritySnapshot crud.Repository[entity.SessionPrioritySnapshot]
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

		Group:       repository.NewGroupRepository(db),
		GroupMember: crud.NewRepository[entity.GroupMember](db),

		DecisionSession:         crud.NewRepository[entity.DecisionSession](db),
		SessionParticipant:      crud.NewRepository[entity.SessionParticipant](db),
		SessionCandidate:        crud.NewRepository[entity.SessionCandidate](db),
		SessionVote:             crud.NewRepository[entity.SessionVote](db),
		SessionRanking:          crud.NewRepository[entity.SessionRanking](db),
		SessionPrioritySnapshot: crud.NewRepository[entity.SessionPrioritySnapshot](db),
	}
}
