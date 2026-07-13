package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/itsLeonB/go-crud"
	"github.com/itsLeonB/ungerr"
	"github.com/yunobar/album/internal/core/otel"
	"github.com/yunobar/album/internal/domain/dto"
	"github.com/yunobar/album/internal/domain/entity"
	"github.com/yunobar/album/internal/domain/mapper"
)

type GroupService interface {
	Create(ctx context.Context, profileID uuid.UUID, request dto.CreateGroupRequest) (dto.GroupResponse, error)
	Get(ctx context.Context, profileID, groupID uuid.UUID) (dto.GroupResponse, error)
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

func (gs *groupServiceImpl) membersOf(ctx context.Context, groupID uuid.UUID) ([]entity.GroupMember, error) {
	spec := crud.Specification[entity.GroupMember]{PreloadRelations: []string{"Profile"}}
	spec.Model.GroupID = groupID

	return gs.groupMemberRepo.FindAll(ctx, spec)
}
