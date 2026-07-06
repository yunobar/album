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

type ProfileService interface {
	Create(ctx context.Context, request dto.NewProfileRequest) (dto.ProfileResponse, error)
	GetByID(ctx context.Context, id uuid.UUID) (dto.ProfileResponse, error)
	GetProfileIDByUserID(ctx context.Context, userID uuid.UUID) (uuid.UUID, error)
	Update(ctx context.Context, req dto.UpdateProfileRequest) (dto.ProfileResponse, error)
}

type profileServiceImpl struct {
	transactor  crud.Transactor
	profileRepo crud.Repository[entity.UserProfile]
	userRepo    crud.Repository[entity.User]
}

func NewProfileService(
	transactor crud.Transactor,
	profileRepo crud.Repository[entity.UserProfile],
	userRepo crud.Repository[entity.User],
) ProfileService {
	return &profileServiceImpl{
		transactor,
		profileRepo,
		userRepo,
	}
}

func (ps *profileServiceImpl) Create(ctx context.Context, request dto.NewProfileRequest) (dto.ProfileResponse, error) {
	ctx, span := otel.Tracer.Start(ctx, "ProfileService.Create")
	defer span.End()

	var response dto.ProfileResponse

	err := ps.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		if request.UserID != uuid.Nil {
			spec := crud.Specification[entity.UserProfile]{}
			spec.Model.UserID = uuid.NullUUID{UUID: request.UserID, Valid: true}
			existing, err := ps.profileRepo.FindFirst(ctx, spec)
			if err != nil {
				return err
			}
			if !existing.IsZero() {
				response = mapper.ProfileToResponse(existing, "")
				return nil
			}
		}

		newProfile := entity.UserProfile{
			UserID: uuid.NullUUID{
				UUID:  request.UserID,
				Valid: request.UserID != uuid.Nil,
			},
			Name:   request.Name,
			Avatar: request.Avatar,
		}

		insertedProfile, err := ps.profileRepo.Insert(ctx, newProfile)
		if err != nil {
			return err
		}

		response = mapper.ProfileToResponse(insertedProfile, "")

		return nil
	})

	return response, err
}

func (ps *profileServiceImpl) GetByID(ctx context.Context, id uuid.UUID) (dto.ProfileResponse, error) {
	ctx, span := otel.Tracer.Start(ctx, "ProfileService.GetByID")
	defer span.End()

	spec := crud.Specification[entity.UserProfile]{}
	spec.Model.ID = id
	profile, err := ps.profileRepo.FindFirst(ctx, spec)
	if err != nil {
		return dto.ProfileResponse{}, err
	}
	if profile.IsZero() {
		return dto.ProfileResponse{}, ungerr.NotFoundError(fmt.Sprintf("profile with ID: %s is not found", id))
	}

	var email string
	if profile.IsReal() {
		entitypec := crud.Specification[entity.User]{}
		entitypec.Model.ID = profile.UserID.UUID
		user, err := ps.userRepo.FindFirst(ctx, entitypec)
		if err != nil {
			return dto.ProfileResponse{}, err
		}
		email = user.Email
	}

	return mapper.ProfileToResponse(profile, email), nil
}

func (ps *profileServiceImpl) GetProfileIDByUserID(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	ctx, span := otel.Tracer.Start(ctx, "ProfileService.GetProfileIDByUserID")
	defer span.End()

	spec := crud.Specification[entity.UserProfile]{}
	spec.Model.UserID = uuid.NullUUID{UUID: userID, Valid: true}
	profile, err := ps.profileRepo.FindFirst(ctx, spec)
	if err != nil {
		return uuid.Nil, err
	}
	if profile.IsZero() {
		return uuid.Nil, ungerr.NotFoundError(fmt.Sprintf("profile for user %s not found", userID))
	}
	return profile.ID, nil
}

func (ps *profileServiceImpl) Update(ctx context.Context, req dto.UpdateProfileRequest) (dto.ProfileResponse, error) {
	ctx, span := otel.Tracer.Start(ctx, "ProfileService.Update")
	defer span.End()

	var response dto.ProfileResponse
	err := ps.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		spec := crud.Specification[entity.UserProfile]{}
		spec.Model.ID = req.ID
		spec.ForUpdate = true
		profile, err := ps.profileRepo.FindFirst(ctx, spec)
		if err != nil {
			return err
		}
		if profile.IsZero() {
			return ungerr.NotFoundError(fmt.Sprintf("profile ID: %s is not found", req.ID))
		}

		if req.Name != "" {
			profile.Name = req.Name
		}

		updatedProfile, err := ps.profileRepo.Update(ctx, profile)
		if err != nil {
			return err
		}

		response = mapper.ProfileToResponse(updatedProfile, "")
		return nil
	})
	return response, err
}
