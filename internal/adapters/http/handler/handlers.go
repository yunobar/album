package handler

import (
	"time"

	"github.com/itsLeonB/go-authkit/authgin"
	"github.com/yunobar/album/internal/adapters/http/middlewares"
	"github.com/yunobar/album/internal/provider"
)

type Handlers struct {
	Auth            *authgin.Handler
	Profile         *ProfileHandler
	Content         *ContentHandler
	Watchlist       *WatchlistHandler
	Group           *GroupHandler
	DecisionSession *DecisionSessionHandler

	emailLimiter *middlewares.ValueLimiter
}

func (h *Handlers) Shutdown() {
	h.emailLimiter.Stop()
}

func ProvideHandlers(services *provider.Services, coreSvc *provider.CoreServices, transport *authgin.CookieTransport) *Handlers {
	emailLimiter := middlewares.NewValueLimiter(3.0/3600, 3, time.Hour)

	authHandler := authgin.NewHandler(services.AuthKit, transport, authgin.HandlerConfig{
		Captcha: services.Captcha,
		Limiter: emailLimiter,
	})

	return &Handlers{
		Auth:            authHandler,
		Profile:         NewProfileHandler(services.Profile),
		Content:         NewContentHandler(services.Content),
		Watchlist:       NewWatchlistHandler(services.Watchlist),
		Group:           NewGroupHandler(services.Group),
		DecisionSession: NewDecisionSessionHandler(services.DecisionSession, coreSvc.NATSConn),

		emailLimiter: emailLimiter,
	}
}
