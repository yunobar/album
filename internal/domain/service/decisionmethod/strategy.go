// Package decisionmethod implements the per-decision-method behavior
// (majority, ranked, priority, round_robin, random) behind a Strategy
// interface, so decisionSessionServiceImpl can dispatch on
// session.Method without a hand-rolled switch at every call site.
package decisionmethod

import (
	"context"

	"github.com/google/uuid"
	"github.com/itsLeonB/go-crud"
	"github.com/yunobar/album/internal/appconstant"
	"github.com/yunobar/album/internal/domain/entity"
)

// Strategy is the per-method behavior behind a decision session. All
// methods receive the already-loaded session; callers are responsible for
// any locking/transaction semantics their context requires (round_robin's
// Resolve, in particular, must be called with the session and its group
// already locked inside the caller's transaction).
type Strategy interface {
	// OnCreate runs method-specific setup right after the session,
	// participants, and candidates are inserted. Runs inside Create's txn.
	OnCreate(ctx context.Context, session entity.DecisionSession, participantIDs, candidateIDs []uuid.UUID) error
	// Tally computes the live tally for GET and post-submission responses.
	// Returns nil for methods with no tally (priority, random).
	Tally(ctx context.Context, session entity.DecisionSession) (any, error)
	// Chooser returns the current chooser profile, or nil for methods
	// without one (everything but round_robin).
	Chooser(ctx context.Context, session entity.DecisionSession) (*uuid.UUID, error)
	// Resolve picks the winner at finalize time and performs any
	// method-specific side effects (round_robin advances the group pointer).
	// MUST be called inside Finalize's transaction with the session locked.
	Resolve(ctx context.Context, session entity.DecisionSession) (uuid.UUID, error)
}

// base supplies no-op defaults for the methods most strategies don't need
// to override. Resolve has deliberately no default — every method must
// resolve a winner, so omitting it is a compile error for any strategy that
// forgets to implement it.
type base struct{}

func (base) OnCreate(context.Context, entity.DecisionSession, []uuid.UUID, []uuid.UUID) error {
	return nil
}

func (base) Tally(context.Context, entity.DecisionSession) (any, error) {
	return nil, nil
}

func (base) Chooser(context.Context, entity.DecisionSession) (*uuid.UUID, error) {
	return nil, nil
}

// NewRegistry builds the method -> Strategy map, keyed by the DB enum value
// stored on decision_sessions.method (see mapper.MethodToDB).
func NewRegistry(
	groupRepo crud.Repository[entity.Group],
	groupMemberRepo crud.Repository[entity.GroupMember],
	watchlistRepo crud.Repository[entity.WatchlistItem],
	sessionVoteRepo crud.Repository[entity.SessionVote],
	sessionRankingRepo crud.Repository[entity.SessionRanking],
	sessionPrioritySnapshotRepo crud.Repository[entity.SessionPrioritySnapshot],
) map[string]Strategy {
	return map[string]Strategy{
		appconstant.SessionMethodMajority:   &majorityStrategy{sessionVoteRepo: sessionVoteRepo},
		appconstant.SessionMethodRanked:     &rankedStrategy{sessionRankingRepo: sessionRankingRepo},
		appconstant.SessionMethodPriority:   &priorityStrategy{watchlistRepo: watchlistRepo, snapshotRepo: sessionPrioritySnapshotRepo},
		appconstant.SessionMethodRoundRobin: &roundRobinStrategy{groupRepo: groupRepo, groupMemberRepo: groupMemberRepo, sessionVoteRepo: sessionVoteRepo},
		appconstant.SessionMethodRandom:     &randomStrategy{},
	}
}
