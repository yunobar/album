package decisionmethod

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/itsLeonB/go-crud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/yunobar/album/internal/domain/entity"
	"github.com/yunobar/album/internal/mocks"
)

func TestResolveRoundRobin(t *testing.T) {
	newStrategy := func(t *testing.T, chooserVote entity.SessionVote) (*roundRobinStrategy, entity.DecisionSession) {
		t.Helper()
		profileID := uuid.New()
		groupID := uuid.New()

		members := []entity.GroupMember{{GroupID: groupID, ProfileID: profileID}}
		participants := []entity.SessionParticipant{{ProfileID: profileID}}
		group := entity.Group{RoundRobinPointer: 0}

		groupRepo := mocks.NewMockRepository[entity.Group](t)
		groupMemberRepo := mocks.NewMockRepository[entity.GroupMember](t)
		sessionVoteRepo := mocks.NewMockRepository[entity.SessionVote](t)

		groupMemberRepo.EXPECT().FindAll(mock.Anything, mock.Anything).Return(members, nil)
		groupRepo.EXPECT().FindFirst(mock.Anything, mock.Anything).Return(group, nil)
		sessionVoteRepo.EXPECT().FindFirst(mock.Anything, mock.Anything).Return(chooserVote, nil)
		groupRepo.EXPECT().Update(mock.Anything, mock.Anything).Return(entity.Group{}, nil)

		strat := &roundRobinStrategy{groupRepo: groupRepo, groupMemberRepo: groupMemberRepo, sessionVoteRepo: sessionVoteRepo}
		session := entity.DecisionSession{
			GroupID:      groupID,
			Participants: participants,
		}
		return strat, session
	}

	t.Run("chooser's vote wins", func(t *testing.T) {
		a, b := uuid.New(), uuid.New()
		strat, session := newStrategy(t, entity.SessionVote{BaseEntity: crud.BaseEntity{ID: uuid.New()}, ContentID: b})
		session.Candidates = []entity.SessionCandidate{{ContentID: a}, {ContentID: b}}
		session.RandomSeed = 1

		winner, err := strat.Resolve(context.Background(), session)

		assert.NoError(t, err)
		assert.Equal(t, b, winner)
	})

	t.Run("nil chooser vote falls to a deterministic seeded pick", func(t *testing.T) {
		a, b := uuid.New(), uuid.New()

		strat1, session1 := newStrategy(t, entity.SessionVote{})
		session1.Candidates = []entity.SessionCandidate{{ContentID: a}, {ContentID: b}}
		session1.RandomSeed = 5
		first, err := strat1.Resolve(context.Background(), session1)
		assert.NoError(t, err)

		strat2, session2 := newStrategy(t, entity.SessionVote{})
		session2.Candidates = []entity.SessionCandidate{{ContentID: a}, {ContentID: b}}
		session2.RandomSeed = 5
		second, err := strat2.Resolve(context.Background(), session2)
		assert.NoError(t, err)

		assert.Equal(t, first, second)
		assert.True(t, first == a || first == b)
	})
}

func TestChooserFromGroup(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	members := []entity.GroupMember{{ProfileID: a}, {ProfileID: b}, {ProfileID: c}}
	participants := []entity.SessionParticipant{{ProfileID: a}, {ProfileID: b}, {ProfileID: c}}

	t.Run("picks the member at the pointer index", func(t *testing.T) {
		group := entity.Group{RoundRobinPointer: 0}

		chooser := chooserFromGroup(group, members, participants)

		require.NotNil(t, chooser)
	})

	t.Run("pointer past the ordered length wraps via modulo", func(t *testing.T) {
		group := entity.Group{RoundRobinPointer: 0}
		first := chooserFromGroup(group, members, participants)

		wrapped := entity.Group{RoundRobinPointer: len(participants)}
		second := chooserFromGroup(wrapped, members, participants)

		require.NotNil(t, first)
		require.NotNil(t, second)
		assert.Equal(t, *first, *second)
	})

	t.Run("no member is a session participant yields nil", func(t *testing.T) {
		group := entity.Group{RoundRobinPointer: 0}

		chooser := chooserFromGroup(group, members, nil)

		assert.Nil(t, chooser)
	})
}
