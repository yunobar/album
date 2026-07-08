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
	"github.com/yunobar/album/internal/domain/client"
	"github.com/yunobar/album/internal/domain/service"
)

type Services struct {
	// Auth
	AuthKit *authkit.AuthKit
	Captcha client.TurnstileClient

	// Users
	Profile service.ProfileService

	// Content
	Content service.ContentService

	// Watchlist
	Watchlist service.WatchlistService
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

	tmdbClient := client.NewTMDBClient(config.Global.BaseUrl, config.Global.TMDB.ApiKey)
	content := service.NewContentService(repos.Transactor, repos.Content, tmdbClient)

	watchlist := service.NewWatchlistService(repos.Transactor, repos.Watchlist, repos.Content)

	return &Services{
		AuthKit: kit,
		Captcha: client.NewTurnstileClient(config.Global.TurnstileSecretKey),

		Profile: profile,

		Content: content,

		Watchlist: watchlist,
	}, nil
}

func provideAuthKit(repos *Repositories, profile service.ProfileService, coreSvc *CoreServices) (*authkit.AuthKit, error) {
	authConfig := config.Global.Auth
	appConfig := config.Global.App
	oauthConfig := config.Global.OAuthProviders

	// Auth adapters bridge authkit store interfaces to existing repos/infra.
	userStore := authadapter.NewUserStore(repos.Transactor, repos.User, profile)
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
