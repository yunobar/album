package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/itsLeonB/go-crud"
	"github.com/itsLeonB/ungerr"
	"github.com/yunobar/album/internal/appconstant"
	"github.com/yunobar/album/internal/domain/entity"
	"gorm.io/gorm"
)

// GroupRepository extends the generic CRUD repository with group-specific
// queries that don't fit go-crud's Specification shape (joins across
// tables).
type GroupRepository interface {
	crud.Repository[entity.Group]
	// FindActiveSession is the caller-scoped group->session resolution path
	// (ADR-0006): it joins session_participants on the caller's profile_id,
	// not just group_id, so a member who wasn't picked as a participant gets
	// nil rather than learning the session exists. Returns nil, nil when
	// there's no match.
	FindActiveSession(ctx context.Context, groupID, profileID uuid.UUID) (*ActiveSession, error)
	// FindMergedContentIDs returns the DISTINCT content IDs actively
	// watchlisted by any member of the group — the candidate universe a
	// decision session may draw from. Just the id set, no aggregation (cf.
	// the fuller GetMergedWatchlist projection).
	FindMergedContentIDs(ctx context.Context, groupID uuid.UUID) ([]uuid.UUID, error)
}

// ActiveSession is FindActiveSession's result — a frozen subset of
// decision_sessions, not the full entity (the query never needs the rest of
// the row).
type ActiveSession struct {
	ID     uuid.UUID
	Method string
}

type groupRepositoryImpl struct {
	crud.Repository[entity.Group]
}

func NewGroupRepository(db *gorm.DB) GroupRepository {
	return &groupRepositoryImpl{
		Repository: crud.NewRepository[entity.Group](db),
	}
}

func (gr *groupRepositoryImpl) FindActiveSession(ctx context.Context, groupID, profileID uuid.UUID) (*ActiveSession, error) {
	db, err := gr.GetGormInstance(ctx)
	if err != nil {
		return nil, err
	}

	var row ActiveSession
	err = db.Raw(`
		SELECT ds.id AS id, ds.method AS method
		FROM decision_sessions ds
		JOIN session_participants sp ON sp.session_id = ds.id AND sp.profile_id = ?
		WHERE ds.group_id = ? AND ds.status = ?
		LIMIT 1
	`, profileID, groupID, appconstant.SessionStatusVoting).Scan(&row).Error
	if err != nil {
		return nil, ungerr.Wrap(err, "error querying active session")
	}
	if row.ID == uuid.Nil {
		return nil, nil
	}

	return &row, nil
}

func (gr *groupRepositoryImpl) FindMergedContentIDs(ctx context.Context, groupID uuid.UUID) ([]uuid.UUID, error) {
	db, err := gr.GetGormInstance(ctx)
	if err != nil {
		return nil, err
	}

	var ids []uuid.UUID
	err = db.Raw(`
		SELECT DISTINCT wi.content_id
		FROM group_members gm
		JOIN watchlist_items wi ON wi.profile_id = gm.profile_id AND wi.status = ?
		WHERE gm.group_id = ?
	`, appconstant.WatchlistStatusActive, groupID).Scan(&ids).Error
	if err != nil {
		return nil, ungerr.Wrap(err, "error querying merged watchlist content ids")
	}

	return ids, nil
}
