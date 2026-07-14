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

		// Insert via raw SQL, omitting round_robin_pointer entirely, so the
		// assertion below proves the DB's DEFAULT 0 — not just that a fresh
		// Go struct's int field starts at zero (which it would, DB default
		// broken or not, since testDB.Create already leaves that field 0
		// before it's ever reloaded). Scanning into a struct (not a bare
		// uuid.UUID) so GORM routes the column through uuid.UUID's own
		// sql.Scanner instead of a raw-value conversion that doesn't apply.
		var raw struct{ ID uuid.UUID }
		require.NoError(t, testDB.Raw(
			`INSERT INTO groups (invite_token) VALUES (?) RETURNING id`,
			uuid.NewString(),
		).Scan(&raw).Error)

		var reloaded entity.Group
		require.NoError(t, testDB.First(&reloaded, "id = ?", raw.ID).Error)
		assert.Equal(t, 0, reloaded.RoundRobinPointer, "round_robin_pointer defaults to 0")

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

		// content2's own candidate row — needed for the second ranking
		// insert below, now that session_rankings.content_id composite-FKs
		// to session_candidates rather than contents directly.
		require.NoError(t, testDB.Create(&entity.SessionCandidate{
			SessionID: session.ID,
			ContentID: content2.ID,
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
		require.NoError(t, testDB.Create(&entity.SessionParticipant{SessionID: session.ID, ProfileID: testProfileID}).Error)
		require.NoError(t, testDB.Create(&entity.SessionCandidate{SessionID: session.ID, ContentID: content.ID}).Error)
		require.NoError(t, testDB.Create(&entity.SessionCandidate{SessionID: session.ID, ContentID: content2.ID}).Error)

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
		require.NoError(t, testDB.Create(&entity.SessionParticipant{SessionID: session.ID, ProfileID: testProfileID}).Error)
		require.NoError(t, testDB.Create(&entity.SessionCandidate{SessionID: session.ID, ContentID: content.ID}).Error)

		require.NoError(t, testDB.Create(&entity.SessionRanking{
			SessionID: session.ID, ProfileID: testProfileID, ContentID: content.ID, Rank: 1,
		}).Error)

		err := testDB.Create(&entity.SessionRanking{
			SessionID: session.ID, ProfileID: testProfileID, ContentID: content.ID, Rank: 2,
		}).Error
		assert.Error(t, err, "a second ranking row for the same (session_id, profile_id, content_id) must violate the unique constraint")
	})

	t.Run("session_rankings enforces UNIQUE(session_id, profile_id, rank)", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		content := seedTestContent(t)
		content2 := seedTestContent(t)

		group := entity.Group{InviteToken: uuid.NewString()}
		require.NoError(t, testDB.Create(&group).Error)
		session := entity.DecisionSession{GroupID: group.ID, Method: "ranked", Status: "voting", RandomSeed: 1}
		require.NoError(t, testDB.Create(&session).Error)
		require.NoError(t, testDB.Create(&entity.SessionParticipant{SessionID: session.ID, ProfileID: testProfileID}).Error)
		require.NoError(t, testDB.Create(&entity.SessionCandidate{SessionID: session.ID, ContentID: content.ID}).Error)
		require.NoError(t, testDB.Create(&entity.SessionCandidate{SessionID: session.ID, ContentID: content2.ID}).Error)

		require.NoError(t, testDB.Create(&entity.SessionRanking{
			SessionID: session.ID, ProfileID: testProfileID, ContentID: content.ID, Rank: 1,
		}).Error)

		// A different content_id at the same rank for the same ballot is a
		// malformed/ambiguous ranking (which candidate is actually first?)
		// — must be rejected, not just deduped at the application layer.
		err := testDB.Create(&entity.SessionRanking{
			SessionID: session.ID, ProfileID: testProfileID, ContentID: content2.ID, Rank: 1,
		}).Error
		assert.Error(t, err, "a second ranking row for the same (session_id, profile_id, rank) must violate the unique constraint")
	})

	t.Run("session_votes rejects a profile_id that isn't a session participant", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		content := seedTestContent(t)

		group := entity.Group{InviteToken: uuid.NewString()}
		require.NoError(t, testDB.Create(&group).Error)
		session := entity.DecisionSession{GroupID: group.ID, Method: "majority", Status: "voting", RandomSeed: 1}
		require.NoError(t, testDB.Create(&session).Error)
		require.NoError(t, testDB.Create(&entity.SessionCandidate{SessionID: session.ID, ContentID: content.ID}).Error)
		// Deliberately no SessionParticipant row for testProfileID.

		err := testDB.Create(&entity.SessionVote{
			SessionID: session.ID, ProfileID: testProfileID, ContentID: content.ID,
		}).Error
		assert.Error(t, err, "a vote for a profile_id with no session_participants row must violate the composite FK")
	})

	t.Run("session_votes rejects a content_id that isn't a session candidate", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		content := seedTestContent(t)

		group := entity.Group{InviteToken: uuid.NewString()}
		require.NoError(t, testDB.Create(&group).Error)
		session := entity.DecisionSession{GroupID: group.ID, Method: "majority", Status: "voting", RandomSeed: 1}
		require.NoError(t, testDB.Create(&session).Error)
		require.NoError(t, testDB.Create(&entity.SessionParticipant{SessionID: session.ID, ProfileID: testProfileID}).Error)
		// Deliberately no SessionCandidate row for content.

		err := testDB.Create(&entity.SessionVote{
			SessionID: session.ID, ProfileID: testProfileID, ContentID: content.ID,
		}).Error
		assert.Error(t, err, "a vote for a content_id with no session_candidates row must violate the composite FK")
	})
}
