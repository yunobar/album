package tests

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yunobar/album/internal/domain/entity"
	"github.com/yunobar/album/internal/testhelpers"
)

func TestDecisionSessionSchema(t *testing.T) {
	testhelpers.RequireTestDB(t, testDB)

	t.Run("insert one row per new table round-trips and FKs hold", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		content := seedTestContent(t)
		content2 := seedTestContent(t)

		group := entity.Group{InviteToken: uuid.NewString()}
		require.NoError(t, testDB.Create(&group).Error)
		assert.Equal(t, 0, group.RoundRobinPointer, "round_robin_pointer defaults to 0")

		session := entity.DecisionSession{
			GroupID:    group.ID,
			Method:     "priority",
			Status:     "voting",
			RandomSeed: 42,
		}
		require.NoError(t, testDB.Create(&session).Error)

		require.NoError(t, testDB.Create(&entity.SessionParticipant{
			SessionID: session.ID,
			ProfileID: testProfileID,
		}).Error)

		require.NoError(t, testDB.Create(&entity.SessionCandidate{
			SessionID: session.ID,
			ContentID: content.ID,
		}).Error)

		require.NoError(t, testDB.Create(&entity.SessionVote{
			SessionID: session.ID,
			ProfileID: testProfileID,
			ContentID: content.ID,
		}).Error)

		require.NoError(t, testDB.Create(&entity.SessionRanking{
			SessionID: session.ID,
			ProfileID: testProfileID,
			ContentID: content.ID,
			Rank:      1,
		}).Error)

		require.NoError(t, testDB.Create(&entity.SessionPrioritySnapshot{
			SessionID: session.ID,
			ProfileID: testProfileID,
			ContentID: content.ID,
			Priority:  "high",
		}).Error)

		// A different content_id for the same participant succeeds — the
		// unique constraint is the full (session_id, profile_id,
		// content_id) triple, not just (session_id, profile_id).
		require.NoError(t, testDB.Create(&entity.SessionRanking{
			SessionID: session.ID,
			ProfileID: testProfileID,
			ContentID: content2.ID,
			Rank:      2,
		}).Error)
	})

	t.Run("session_votes enforces UNIQUE(session_id, profile_id)", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		content := seedTestContent(t)
		content2 := seedTestContent(t)

		group := entity.Group{InviteToken: uuid.NewString()}
		require.NoError(t, testDB.Create(&group).Error)
		session := entity.DecisionSession{GroupID: group.ID, Method: "majority", Status: "voting", RandomSeed: 1}
		require.NoError(t, testDB.Create(&session).Error)

		require.NoError(t, testDB.Create(&entity.SessionVote{
			SessionID: session.ID, ProfileID: testProfileID, ContentID: content.ID,
		}).Error)

		err := testDB.Create(&entity.SessionVote{
			SessionID: session.ID, ProfileID: testProfileID, ContentID: content2.ID,
		}).Error
		assert.Error(t, err, "a second vote for the same (session_id, profile_id) must violate the unique constraint")
	})

	t.Run("session_rankings enforces UNIQUE(session_id, profile_id, content_id)", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		content := seedTestContent(t)

		group := entity.Group{InviteToken: uuid.NewString()}
		require.NoError(t, testDB.Create(&group).Error)
		session := entity.DecisionSession{GroupID: group.ID, Method: "ranked", Status: "voting", RandomSeed: 1}
		require.NoError(t, testDB.Create(&session).Error)

		require.NoError(t, testDB.Create(&entity.SessionRanking{
			SessionID: session.ID, ProfileID: testProfileID, ContentID: content.ID, Rank: 1,
		}).Error)

		err := testDB.Create(&entity.SessionRanking{
			SessionID: session.ID, ProfileID: testProfileID, ContentID: content.ID, Rank: 2,
		}).Error
		assert.Error(t, err, "a second ranking row for the same (session_id, profile_id, content_id) must violate the unique constraint")
	})
}
