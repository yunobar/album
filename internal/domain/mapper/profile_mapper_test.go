package mapper

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/itsLeonB/go-crud"
	"github.com/stretchr/testify/assert"
	"github.com/yunobar/album/internal/domain/entity"
)

func TestProfileToResponse(t *testing.T) {
	tests := []struct {
		name  string
		email string
	}{
		{"with email", "test@example.com"},
		{"empty email", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := uuid.New()
			userID := uuid.New()
			profile := entity.UserProfile{
				BaseEntity: crud.BaseEntity{ID: id, CreatedAt: time.Now(), UpdatedAt: time.Now()},
				UserID:     uuid.NullUUID{UUID: userID, Valid: true},
				Name:       "Alice",
				Avatar:     "avatar.png",
			}

			got := ProfileToResponse(profile, tt.email)

			assert.Equal(t, id, got.ID)
			assert.Equal(t, userID, got.UserID)
			assert.Equal(t, "Alice", got.Name)
			assert.Equal(t, "avatar.png", got.Avatar)
			assert.Equal(t, tt.email, got.Email)
		})
	}
}
