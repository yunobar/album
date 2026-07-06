package provider

import (
	"errors"

	"github.com/google/uuid"
	"github.com/itsLeonB/go-authkit"
	"github.com/markbates/goth/providers/google"
	authadapter "github.com/yunobar/album/internal/adapters/auth"
	"github.com/yunobar/album/internal/core/cache"
	"github.com/yunobar/album/internal/core/config"
	"github.com/yunobar/album/internal/core/otel"
	"github.com/yunobar/album/internal/domain/service"
)

type Services struct {
	// Auth
	AuthKit *authkit.AuthKit
	Captcha service.CaptchaService

	// Users
	Profile service.ProfileService
}

func (s *Services) Shutdown() error {
	return errors.Join(s.AuthKit.Shutdown())
}

func ProvideServices(
	repos *Repositories,
	coreSvc *CoreServices,
) (*Services, error) {
	profile := service.NewProfileService(repos.Transactor, repos.Profile, repos.User)

	kit, err := provideAuthKit(repos, profile, coreSvc)
	if err != nil {
		return nil, err
	}

	return &Services{
		AuthKit: kit,
		Captcha: service.NewTurnstileService(config.Global.TurnstileSecretKey),

		Profile: profile,
	}, nil
}

func provideAuthKit(repos *Repositories, profile service.ProfileService, coreSvc *CoreServices) (*authkit.AuthKit, error) {
	authConfig := config.Global.Auth
	appConfig := config.Global.App
	oauthConfig := config.Global.OAuthProviders

	// Auth adapters bridge authkit store interfaces to existing repos/infra.
	userStore := authadapter.NewUserStore(repos.User, profile)
	sessionStore := authadapter.NewSessionStore(repos.Session)
	refreshTokenStore := authadapter.NewRefreshTokenStore(repos.RefreshToken)
	resetTokenStore := authadapter.NewResetTokenStore(repos.PasswordResetToken)
	oauthAccountStore := authadapter.NewOAuthAccountStore(repos.OAuthAccount)
	mailAdapter := authadapter.NewMailAdapter(coreSvc.Mail)
	sessionCache := cache.NewInMemoryCache[uuid.UUID](authConfig.TokenDuration)
	cacheAdapter := authadapter.NewSessionCacheAdapter(sessionCache)

	hooks := newAuthKitHooks(profile)

	authKitCfg := authkit.Config{
		RequireFingerprint: new(false),
		VerificationURL:    appConfig.RegisterVerificationUrl,
		ResetPasswordURL:   appConfig.ResetPasswordUrl,
		RefreshTokenTTL:    authConfig.RefreshTokenDuration,
		JWTIssuer:          authConfig.Issuer,
		JWTSecret:          authConfig.SecretKey,
		JWTDuration:        authConfig.TokenDuration,
		Tracer:             otel.Tracer,
	}

	authKitDeps := authkit.Deps{
		Tx:       repos.Transactor,
		Users:    userStore,
		Sessions: sessionStore,
		Refresh:  refreshTokenStore,
		Resets:   resetTokenStore,
		OAuth:    oauthAccountStore,
		Mail:     mailAdapter,
		Cache:    cacheAdapter,
		State:    coreSvc.State,
		Providers: []authkit.ProviderConfig{
			{Provider: google.New(oauthConfig.Google.ClientID, oauthConfig.Google.ClientSecret, oauthConfig.Google.RedirectUrl, "email", "profile"), Trusted: true},
		},
	}

	return authkit.New(authKitCfg, authKitDeps, hooks)
}
