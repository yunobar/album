package tests

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yunobar/album/internal/domain/entity"
	"github.com/yunobar/album/internal/testhelpers"
)

func TestGroupSchema(t *testing.T) {
	testhelpers.RequireTestDB(t, testDB)

	t.Run("group_members enforces UNIQUE(group_id, profile_id)", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)

		group := entity.Group{InviteToken: uuid.NewString()}
		require.NoError(t, testDB.Create(&group).Error)

		require.NoError(t, testDB.Create(&entity.GroupMember{
			GroupID:   group.ID,
			ProfileID: testProfileID,
		}).Error)

		err := testDB.Create(&entity.GroupMember{
			GroupID:   group.ID,
			ProfileID: testProfileID,
		}).Error
		assert.Error(t, err, "duplicate (group_id, profile_id) must violate the unique constraint")
	})

	t.Run("groups.invite_token is unique", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)

		token := uuid.NewString()
		require.NoError(t, testDB.Create(&entity.Group{InviteToken: token}).Error)

		err := testDB.Create(&entity.Group{InviteToken: token}).Error
		assert.Error(t, err, "duplicate invite_token must violate the unique constraint")
	})
}
