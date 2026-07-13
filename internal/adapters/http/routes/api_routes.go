package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/kroma-labs/sentinel-go/httpserver"
	sentinelGin "github.com/kroma-labs/sentinel-go/httpserver/adapters/gin"
	"github.com/yunobar/album/internal/adapters/http/handler"
	"github.com/yunobar/album/internal/appconstant"
	"golang.org/x/time/rate"
)

func RegisterAPIRoutes(router *gin.Engine, handlers *handler.Handlers, authMiddleware gin.HandlerFunc) {
	apiRoutes := router.Group("/api")
	{
		v1 := apiRoutes.Group("/v1")
		{
			authRoutes := v1.Group("/auth")
			authRoutes.Use(sentinelGin.RateLimit(httpserver.RateLimitConfig{
				Limit:   rate.Limit(20.0 / 60),
				Burst:   5,
				KeyFunc: httpserver.KeyFuncByIP(),
			}))
			{
				authRoutes.POST("/register", handlers.Auth.Register())
				authRoutes.POST("/login", handlers.Auth.Login())
				authRoutes.PUT("/refresh", handlers.Auth.RefreshToken())
				authRoutes.GET("/:provider", handlers.Auth.OAuthLogin())
				authRoutes.GET("/:provider/callback", handlers.Auth.OAuthCallback())
				authRoutes.GET("/verify-registration", handlers.Auth.VerifyRegistration())
				authRoutes.POST("/password-reset",
					sentinelGin.RateLimit(httpserver.RateLimitConfig{
						Limit:   rate.Limit(3.0 / 900),
						Burst:   3,
						KeyFunc: httpserver.KeyFuncByIP(),
					}),
					handlers.Auth.SendPasswordReset(),
				)
				authRoutes.PATCH("/reset-password",
					sentinelGin.RateLimit(httpserver.RateLimitConfig{
						Limit:   rate.Limit(5.0 / 900),
						Burst:   5,
						KeyFunc: httpserver.KeyFuncByIP(),
					}),
					handlers.Auth.ResetPassword(),
				)
			}

			protectedRoutes := v1.Group("/", authMiddleware)
			{
				protectedRoutes.DELETE("/auth/logout", handlers.Auth.Logout())

				profileRoutes := protectedRoutes.Group("/profile")
				{
					profileRoutes.GET("", handlers.Profile.HandleProfile())
					profileRoutes.PATCH("", handlers.Profile.HandleUpdate())
				}

				contentRoutes := protectedRoutes.Group("/content")
				contentRoutes.Use(sentinelGin.RateLimit(httpserver.RateLimitConfig{
					Limit:   rate.Limit(5),
					Burst:   10,
					KeyFunc: httpserver.KeyFuncByIP(),
				}))
				{
					contentRoutes.GET("/search", handlers.Content.HandleSearch())
				}

				watchlistRoutes := protectedRoutes.Group("/watchlist")
				{
					watchlistRoutes.GET("", handlers.Watchlist.HandleList())
					watchlistRoutes.POST("", handlers.Watchlist.HandleAdd())
					watchlistRoutes.PATCH("/:"+appconstant.ContextContentID.String(), handlers.Watchlist.HandleUpdate())
					watchlistRoutes.DELETE("/:"+appconstant.ContextContentID.String(), handlers.Watchlist.HandleRemove())
				}

				groupRoutes := protectedRoutes.Group("/groups")
				{
					groupRoutes.POST("", handlers.Group.HandleCreate())
					groupRoutes.GET("", handlers.Group.HandleList())
					groupRoutes.GET("/:"+appconstant.ContextGroupID.String(), handlers.Group.HandleGet())
					groupRoutes.POST("/join/:"+appconstant.ContextToken.String(), handlers.Group.HandleJoin())
					groupRoutes.GET("/:"+appconstant.ContextGroupID.String()+"/watchlist", handlers.Group.HandleMergedWatchlist())
					groupRoutes.POST("/:"+appconstant.ContextGroupID.String()+"/sessions", handlers.DecisionSession.HandleCreate())
				}

				sessionRoutes := protectedRoutes.Group("/sessions")
				{
					sessionRoutes.GET("/:"+appconstant.ContextSessionID.String(), handlers.DecisionSession.HandleGet())
				}
			}
		}
	}
}
