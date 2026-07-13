package service

import (
	"context"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/itsLeonB/go-crud"
	"github.com/itsLeonB/ungerr"
	"github.com/yunobar/album/internal/appconstant"
	"github.com/yunobar/album/internal/core/otel"
	"github.com/yunobar/album/internal/domain/dto"
	"github.com/yunobar/album/internal/domain/entity"
	"github.com/yunobar/album/internal/domain/mapper"
	"gorm.io/gorm/clause"
)

type GroupService interface {
	Create(ctx context.Context, profileID uuid.UUID, request dto.CreateGroupRequest) (dto.GroupResponse, error)
	List(ctx context.Context, profileID uuid.UUID) (dto.ListGroupsResponse, error)
	Get(ctx context.Context, profileID, groupID uuid.UUID) (dto.GroupResponse, error)
	Join(ctx context.Context, profileID uuid.UUID, token string) (dto.GroupResponse, error)
	GetMergedWatchlist(ctx context.Context, profileID, groupID uuid.UUID, filter string) (dto.MergedWatchlistResponse, error)
}

type groupServiceImpl struct {
	transactor      crud.Transactor
	groupRepo       crud.Repository[entity.Group]
	groupMemberRepo crud.Repository[entity.GroupMember]
	profileRepo     crud.Repository[entity.UserProfile]
}

func NewGroupService(
	transactor crud.Transactor,
	groupRepo crud.Repository[entity.Group],
	groupMemberRepo crud.Repository[entity.GroupMember],
	profileRepo crud.Repository[entity.UserProfile],
) GroupService {
	return &groupServiceImpl{transactor, groupRepo, groupMemberRepo, profileRepo}
}

func (gs *groupServiceImpl) List(ctx context.Context, profileID uuid.UUID) (dto.ListGroupsResponse, error) {
	ctx, span := otel.Tracer.Start(ctx, "GroupService.List")
	defer span.End()

	spec := crud.Specification[entity.GroupMember]{PreloadRelations: []string{"Group"}}
	spec.Model.ProfileID = profileID
	memberships, err := gs.groupMemberRepo.FindAll(ctx, spec)
	if err != nil {
		return dto.ListGroupsResponse{}, err
	}

	// ponytail: one membersOf query per group (N+1) — fine at MVP group
	// counts per user (ADR-0002's "MVP group sizes" reasoning applies here
	// too); replace with a single grouped query if this shows up as a
	// measured hot path.
	summaries := make([]dto.GroupSummaryResponse, 0, len(memberships))
	for _, membership := range memberships {
		members, err := gs.membersOf(ctx, membership.GroupID)
		if err != nil {
			return dto.ListGroupsResponse{}, err
		}
		summaries = append(summaries, mapper.GroupToSummary(membership.Group, members, profileID))
	}

	return dto.ListGroupsResponse{Groups: summaries}, nil
}

func (gs *groupServiceImpl) Create(ctx context.Context, profileID uuid.UUID, request dto.CreateGroupRequest) (dto.GroupResponse, error) {
	ctx, span := otel.Tracer.Start(ctx, "GroupService.Create")
	defer span.End()

	var response dto.GroupResponse

	err := gs.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		group, err := gs.groupRepo.Insert(ctx, entity.Group{
			Name:        request.Name,
			InviteToken: uuid.NewString(),
		})
		if err != nil {
			return err
		}

		profileSpec := crud.Specification[entity.UserProfile]{}
		profileSpec.Model.ID = profileID
		profile, err := gs.profileRepo.FindFirst(ctx, profileSpec)
		if err != nil {
			return err
		}
		if profile.IsZero() {
			return ungerr.NotFoundError(fmt.Sprintf("profile %s is not found", profileID))
		}

		member, err := gs.groupMemberRepo.Insert(ctx, entity.GroupMember{
			GroupID:   group.ID,
			ProfileID: profileID,
		})
		if err != nil {
			return err
		}
		member.Profile = profile

		response = mapper.GroupToResponse(group, []entity.GroupMember{member}, profileID)
		return nil
	})

	return response, err
}

func (gs *groupServiceImpl) Join(ctx context.Context, profileID uuid.UUID, token string) (dto.GroupResponse, error) {
	ctx, span := otel.Tracer.Start(ctx, "GroupService.Join")
	defer span.End()

	var response dto.GroupResponse

	err := gs.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		groupSpec := crud.Specification[entity.Group]{}
		groupSpec.Model.InviteToken = token
		group, err := gs.groupRepo.FindFirst(ctx, groupSpec)
		if err != nil {
			return err
		}
		if group.IsZero() {
			return ungerr.NotFoundError("group for this invite token is not found")
		}

		memberSpec := crud.Specification[entity.GroupMember]{}
		memberSpec.Model.GroupID = group.ID
		memberSpec.Model.ProfileID = profileID
		existing, err := gs.groupMemberRepo.FindFirst(ctx, memberSpec)
		if err != nil {
			return err
		}
		if existing.IsZero() {
			db, err := gs.groupMemberRepo.GetGormInstance(ctx)
			if err != nil {
				return err
			}

			// ON CONFLICT DO NOTHING makes this atomic against a concurrent
			// join for the same (group_id, profile_id): two requests can
			// both see "not yet a member" under READ COMMITTED between the
			// FindFirst above and this insert, but the DB's own unique
			// index resolves the race — the loser silently no-ops instead
			// of erroring, which is exactly what Join's idempotency
			// promises.
			if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&entity.GroupMember{
				GroupID:   group.ID,
				ProfileID: profileID,
			}).Error; err != nil {
				return ungerr.Wrap(err, "error inserting group member")
			}
		}

		members, err := gs.membersOf(ctx, group.ID)
		if err != nil {
			return err
		}

		response = mapper.GroupToResponse(group, members, profileID)
		return nil
	})

	return response, err
}

func (gs *groupServiceImpl) Get(ctx context.Context, profileID, groupID uuid.UUID) (dto.GroupResponse, error) {
	ctx, span := otel.Tracer.Start(ctx, "GroupService.Get")
	defer span.End()

	membership, err := gs.requireMembership(ctx, groupID, profileID)
	if err != nil {
		return dto.GroupResponse{}, err
	}

	members, err := gs.membersOf(ctx, groupID)
	if err != nil {
		return dto.GroupResponse{}, err
	}

	return mapper.GroupToResponse(membership.Group, members, profileID), nil
}

// requireMembership loads the caller's membership row for groupID, preloaded
// with the Group. It returns the same NotFoundError whether the group does
// not exist or the caller is not a member — the API never confirms which.
func (gs *groupServiceImpl) requireMembership(ctx context.Context, groupID, profileID uuid.UUID) (entity.GroupMember, error) {
	spec := crud.Specification[entity.GroupMember]{PreloadRelations: []string{"Group"}}
	spec.Model.GroupID = groupID
	spec.Model.ProfileID = profileID

	membership, err := gs.groupMemberRepo.FindFirst(ctx, spec)
	if err != nil {
		return membership, err
	}
	if membership.IsZero() {
		return membership, ungerr.NotFoundError(fmt.Sprintf("group %s is not found", groupID))
	}

	return membership, nil
}

// membersOf returns the group's members in join order (oldest first) — go-
// crud's FindAll unconditionally orders "created_at DESC", which is the
// wrong direction for join order, so it's reversed here. Sorted by ID rather
// than CreatedAt: IDs are uuidv7 (time-ordered, like CreatedAt) but always
// unique, so there's no tie-break ambiguity if two members' CreatedAt values
// ever land in the same tick. Join order is what name derivation
// concatenates by, and what the spec defines Round Robin ordering on (see
// group-merge-backend.md).
func (gs *groupServiceImpl) membersOf(ctx context.Context, groupID uuid.UUID) ([]entity.GroupMember, error) {
	spec := crud.Specification[entity.GroupMember]{PreloadRelations: []string{"Profile"}}
	spec.Model.GroupID = groupID

	members, err := gs.groupMemberRepo.FindAll(ctx, spec)
	if err != nil {
		return nil, err
	}

	sort.Slice(members, func(i, j int) bool {
		return members[i].ID.String() < members[j].ID.String()
	})

	return members, nil
}

// mergedWatchlistRow is one (member, active watchlist item, content) row —
// deliberately unaggregated: array_agg/jsonb_object_agg would need a
// Postgres-specific scan type with no precedent in this codebase, whereas
// grouping in Go after a plain JOIN is a few lines of stdlib.
type mergedWatchlistRow struct {
	ContentID   uuid.UUID
	ProfileID   uuid.UUID
	Priority    string
	ContentType string
	Title       string
	ReleaseYear *int
	PosterURL   string
}

func (gs *groupServiceImpl) GetMergedWatchlist(ctx context.Context, profileID, groupID uuid.UUID, filter string) (dto.MergedWatchlistResponse, error) {
	ctx, span := otel.Tracer.Start(ctx, "GroupService.GetMergedWatchlist")
	defer span.End()

	if _, err := gs.requireMembership(ctx, groupID, profileID); err != nil {
		return dto.MergedWatchlistResponse{}, err
	}

	if filter == "" {
		filter = "all"
	}

	db, err := gs.groupMemberRepo.GetGormInstance(ctx)
	if err != nil {
		return dto.MergedWatchlistResponse{}, err
	}

	var rows []mergedWatchlistRow
	err = db.Raw(`
		SELECT wi.content_id  AS content_id,
		       wi.profile_id  AS profile_id,
		       wi.priority    AS priority,
		       c.content_type AS content_type,
		       c.title        AS title,
		       c.release_year AS release_year,
		       c.poster_url   AS poster_url
		FROM group_members gm
		JOIN watchlist_items wi ON wi.profile_id = gm.profile_id AND wi.status = ?
		JOIN content c ON c.id = wi.content_id
		WHERE gm.group_id = ?
		  AND (? = 'all' OR c.content_type = ?)
	`, appconstant.WatchlistStatusActive, groupID, filter, filter).Scan(&rows).Error
	if err != nil {
		return dto.MergedWatchlistResponse{}, ungerr.Wrap(err, "error querying merged watchlist")
	}

	return dto.MergedWatchlistResponse{Filter: filter, Items: mergeRows(rows)}, nil
}

// mergeRows groups per-member rows by content_id (the dedup anchor — see
// ADR-0002) and orders the result by interested_count desc, then title.
func mergeRows(rows []mergedWatchlistRow) []dto.MergedItemResponse {
	type accumulator struct {
		content    dto.MergedContentResponse
		members    []uuid.UUID
		priorities map[string]string
	}

	order := make([]uuid.UUID, 0, len(rows))
	byContent := make(map[uuid.UUID]*accumulator, len(rows))

	for _, row := range rows {
		acc, ok := byContent[row.ContentID]
		if !ok {
			acc = &accumulator{
				content: dto.MergedContentResponse{
					ID:          row.ContentID,
					ContentType: row.ContentType,
					Title:       row.Title,
					ReleaseYear: row.ReleaseYear,
					PosterUrl:   row.PosterURL,
				},
				priorities: make(map[string]string),
			}
			byContent[row.ContentID] = acc
			order = append(order, row.ContentID)
		}
		acc.members = append(acc.members, row.ProfileID)
		acc.priorities[row.ProfileID.String()] = row.Priority
	}

	items := make([]dto.MergedItemResponse, 0, len(order))
	for _, id := range order {
		acc := byContent[id]
		items = append(items, dto.MergedItemResponse{
			Content:         acc.content,
			InterestedCount: len(acc.members),
			Members:         acc.members,
			Priorities:      acc.priorities,
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].InterestedCount != items[j].InterestedCount {
			return items[i].InterestedCount > items[j].InterestedCount
		}
		return items[i].Content.Title < items[j].Content.Title
	})

	return items
}
