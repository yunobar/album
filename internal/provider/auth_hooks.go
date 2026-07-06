package provider

import (
	"context"

	"github.com/google/uuid"
	"github.com/itsLeonB/go-authkit"
	"github.com/yunobar/album/internal/appconstant"
	"github.com/yunobar/album/internal/domain/service"
)

// newAuthKitHooks builds authkit.Hooks wiring Cashus business logic.
func newAuthKitHooks(
	profileService service.ProfileService,
) authkit.Hooks {
	return authkit.Hooks{
		ClaimsBuilder: func(ctx context.Context, userID string, baseClaims map[string]any) (map[string]any, error) {
			uid, err := uuid.Parse(userID)
			if err != nil {
				return nil, err
			}
			profileID, err := profileService.GetProfileIDByUserID(ctx, uid)
			if err != nil {
				return nil, err
			}
			baseClaims[appconstant.ContextProfileID.String()] = profileID
			return baseClaims, nil
		},
	}
}
