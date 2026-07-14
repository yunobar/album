package tests

import (
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/itsLeonB/ginkgo/pkg/middleware"
	"github.com/itsLeonB/go-crud"
	"github.com/itsLeonB/ungerr"
	"github.com/nats-io/nats.go"
	"github.com/yunobar/album/internal/adapters/http/handler"
	"github.com/yunobar/album/internal/appconstant"
	"github.com/yunobar/album/internal/core/config"
	"github.com/yunobar/album/internal/core/logger"
	"github.com/yunobar/album/internal/core/pubsub"
	"github.com/yunobar/album/internal/domain/dto"
	"github.com/yunobar/album/internal/domain/entity"
	"github.com/yunobar/album/internal/domain/service"
	"github.com/yunobar/album/internal/testhelpers"
	"gorm.io/gorm"
)

var (
	testDB     *gorm.DB
	testNATS   *nats.Conn
	testRouter *gin.Engine

	testProfileID = uuid.MustParse("00000000-0000-0000-0000-000000000002")
	testUserID    = uuid.MustParse("00000000-0000-0000-0000-000000000001")

	// currentContentService is swapped per-subtest so each test case can inject
	// its own mock TMDBClient without re-registering routes.
	currentContentService service.ContentService
)

// testClientOrigin is the only origin the WS upgrader (decision_session_handler.go)
// accepts in tests. It stands in for config.Global.ClientUrls, matching the
// frontend origin used elsewhere in .env.example (e.g. OAUTH_GOOGLE_REDIRECT_URL).
const testClientOrigin = "http://localhost:5173"

func TestMain(m *testing.M) {
	db, cleanup, err := testhelpers.SetupTestDB("../../.env.test")
	if err != nil {
		fmt.Fprintf(os.Stderr, "skipping feature tests: %v\n", err)
		os.Exit(0)
	}
	testDB = db
	defer cleanup()

	nc, natsCleanup := testhelpers.SetupTestNATS()
	testNATS = nc
	defer natsCleanup()

	// ponytail: this test binary never calls config.Load() (it wires repos/
	// services directly against testDB/testNATS, not the full env-loaded
	// config.Global), so config.Global is otherwise nil here. The WS upgrader
	// needs config.Global.ClientUrls for its Origin allowlist, so set just
	// that field directly rather than pulling in config.Load()'s full set of
	// required env vars (DB/MAIL/OAUTH/OTEL/TMDB) for one field.
	config.Global = &config.Config{App: config.App{ClientUrls: []string{testClientOrigin}}}

	logger.Init("album-test")
	testRouter = setupTestRouter(db)

	m.Run()
}

func setupTestRouter(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	errMW := middleware.NewMiddlewareProvider(logger.Global).NewErrorMiddleware()
	r.Use(errMW)
	r.Use(fakeAuthMiddleware())

	registerTestRoutes(r, db)
	return r
}

// testProfileIDHeader lets a test request as a different profile than the
// default testProfileID, to exercise profile-scoping behavior end-to-end.
const testProfileIDHeader = "X-Test-Profile-ID"

func fakeAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		profileID := testProfileID.String()
		if override := c.GetHeader(testProfileIDHeader); override != "" {
			profileID = override
		}
		c.Set(appconstant.ContextProfileID.String(), profileID)
		c.Set("userID", testUserID.String())
		c.Set("sessionID", "test-session")
		c.Next()
	}
}

func registerTestRoutes(r *gin.Engine, db *gorm.DB) {
	transactor := crud.NewTransactor(db)

	// Repos
	profileRepo := crud.NewRepository[entity.UserProfile](db)
	userRepo := crud.NewRepository[entity.User](db)

	// Services
	profileSvc := service.NewProfileService(transactor, profileRepo, userRepo)
	watchlistSvc := service.NewWatchlistService(transactor, crud.NewRepository[entity.WatchlistItem](db), crud.NewRepository[entity.Content](db))
	groupSvc := service.NewGroupService(transactor, crud.NewRepository[entity.Group](db), crud.NewRepository[entity.GroupMember](db), profileRepo)
	decisionSessionSvc := service.NewDecisionSessionService(
		transactor,
		crud.NewRepository[entity.DecisionSession](db),
		crud.NewRepository[entity.SessionParticipant](db),
		crud.NewRepository[entity.SessionCandidate](db),
		crud.NewRepository[entity.GroupMember](db),
		crud.NewRepository[entity.Group](db),
		crud.NewRepository[entity.WatchlistItem](db),
		crud.NewRepository[entity.SessionPrioritySnapshot](db),
		crud.NewRepository[entity.SessionVote](db),
		crud.NewRepository[entity.SessionRanking](db),
		// The real NATS-backed publisher, not a mock — nats-go's Publish
		// gracefully returns ErrInvalidConnection on a nil *nats.Conn (see
		// nats.go's (*Conn).publish nil check), so every non-WS feature test
		// keeps passing even when testNATS is nil (no local NATS server).
		pubsub.NewPublisher(testNATS),
	)

	// Routes
	api := r.Group("/api/v1")

	// Profile
	api.GET("/profile", thinHandler(func(c *gin.Context) (any, error) {
		profileID := getTestProfileID(c)
		return profileSvc.GetByID(c.Request.Context(), profileID)
	}))
	api.PATCH("/profile", func(c *gin.Context) {
		profileID := getTestProfileID(c)
		var req struct {
			Name string `json:"name" binding:"required,min=3,max=255"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			_ = c.Error(ungerr.Wrap(err, "validation"))
			return
		}
		resp, err := profileSvc.Update(c.Request.Context(), dto.UpdateProfileRequest{ID: profileID, Name: req.Name})
		if err != nil {
			_ = c.Error(err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": resp})
	})

	// Content
	api.GET("/content/search", func(c *gin.Context) {
		var req struct {
			Query string `form:"q" binding:"required"`
		}
		if err := c.ShouldBindQuery(&req); err != nil {
			_ = c.Error(ungerr.Wrap(err, "validation"))
			return
		}
		resp, err := currentContentService.Search(c.Request.Context(), req.Query)
		if err != nil {
			_ = c.Error(err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": resp})
	})

	// Watchlist
	api.GET("/watchlist", thinHandler(func(c *gin.Context) (any, error) {
		return watchlistSvc.List(c.Request.Context(), getTestProfileID(c))
	}))
	api.POST("/watchlist", func(c *gin.Context) {
		var req dto.AddWatchlistItemRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			_ = c.Error(ungerr.Wrap(err, "validation"))
			return
		}
		resp, err := watchlistSvc.Add(c.Request.Context(), getTestProfileID(c), req)
		if err != nil {
			_ = c.Error(err)
			return
		}
		c.JSON(http.StatusCreated, gin.H{"data": resp})
	})
	api.PATCH("/watchlist/:contentId", func(c *gin.Context) {
		contentID, err := uuid.Parse(c.Param("contentId"))
		if err != nil {
			_ = c.Error(ungerr.BadRequestError("invalid contentId"))
			return
		}
		var req dto.UpdateWatchlistItemRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			_ = c.Error(ungerr.Wrap(err, "validation"))
			return
		}
		resp, err := watchlistSvc.Update(c.Request.Context(), getTestProfileID(c), contentID, req)
		if err != nil {
			_ = c.Error(err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": resp})
	})
	api.DELETE("/watchlist/:contentId", func(c *gin.Context) {
		contentID, err := uuid.Parse(c.Param("contentId"))
		if err != nil {
			_ = c.Error(ungerr.BadRequestError("invalid contentId"))
			return
		}
		if err := watchlistSvc.Remove(c.Request.Context(), getTestProfileID(c), contentID); err != nil {
			_ = c.Error(err)
			return
		}
		c.Status(http.StatusNoContent)
	})

	// Groups
	api.POST("/groups", func(c *gin.Context) {
		var req dto.CreateGroupRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			_ = c.Error(ungerr.Wrap(err, "validation"))
			return
		}
		resp, err := groupSvc.Create(c.Request.Context(), getTestProfileID(c), req)
		if err != nil {
			_ = c.Error(err)
			return
		}
		c.JSON(http.StatusCreated, gin.H{"data": resp})
	})
	api.GET("/groups", func(c *gin.Context) {
		resp, err := groupSvc.List(c.Request.Context(), getTestProfileID(c))
		if err != nil {
			_ = c.Error(err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": resp})
	})
	api.GET("/groups/:groupID", func(c *gin.Context) {
		groupID, err := uuid.Parse(c.Param("groupID"))
		if err != nil {
			_ = c.Error(ungerr.BadRequestError("invalid groupID"))
			return
		}
		resp, err := groupSvc.Get(c.Request.Context(), getTestProfileID(c), groupID)
		if err != nil {
			_ = c.Error(err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": resp})
	})
	api.POST("/groups/join/:token", func(c *gin.Context) {
		resp, err := groupSvc.Join(c.Request.Context(), getTestProfileID(c), c.Param("token"))
		if err != nil {
			_ = c.Error(err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": resp})
	})
	api.GET("/groups/:groupID/watchlist", func(c *gin.Context) {
		groupID, err := uuid.Parse(c.Param("groupID"))
		if err != nil {
			_ = c.Error(ungerr.BadRequestError("invalid groupID"))
			return
		}
		resp, err := groupSvc.GetMergedWatchlist(c.Request.Context(), getTestProfileID(c), groupID, c.Query("filter"))
		if err != nil {
			_ = c.Error(err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": resp})
	})

	// Decision Sessions
	api.POST("/groups/:groupID/sessions", func(c *gin.Context) {
		groupID, err := uuid.Parse(c.Param("groupID"))
		if err != nil {
			_ = c.Error(ungerr.BadRequestError("invalid groupID"))
			return
		}
		var req dto.CreateSessionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			_ = c.Error(ungerr.Wrap(err, "validation"))
			return
		}
		resp, err := decisionSessionSvc.Create(c.Request.Context(), getTestProfileID(c), groupID, req)
		if err != nil {
			_ = c.Error(err)
			return
		}
		c.JSON(http.StatusCreated, gin.H{"data": resp})
	})
	api.GET("/sessions/:sessionID", func(c *gin.Context) {
		sessionID, err := uuid.Parse(c.Param("sessionID"))
		if err != nil {
			_ = c.Error(ungerr.BadRequestError("invalid sessionID"))
			return
		}
		resp, err := decisionSessionSvc.Get(c.Request.Context(), getTestProfileID(c), sessionID)
		if err != nil {
			_ = c.Error(err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": resp})
	})
	api.POST("/sessions/:sessionID/votes", func(c *gin.Context) {
		sessionID, err := uuid.Parse(c.Param("sessionID"))
		if err != nil {
			_ = c.Error(ungerr.BadRequestError("invalid sessionID"))
			return
		}
		var req dto.CastVoteRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			_ = c.Error(ungerr.Wrap(err, "validation"))
			return
		}
		resp, err := decisionSessionSvc.CastVote(c.Request.Context(), getTestProfileID(c), sessionID, req)
		if err != nil {
			_ = c.Error(err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": resp})
	})
	api.POST("/sessions/:sessionID/rankings", func(c *gin.Context) {
		sessionID, err := uuid.Parse(c.Param("sessionID"))
		if err != nil {
			_ = c.Error(ungerr.BadRequestError("invalid sessionID"))
			return
		}
		var req dto.SubmitRankingRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			_ = c.Error(ungerr.Wrap(err, "validation"))
			return
		}
		resp, err := decisionSessionSvc.SubmitRanking(c.Request.Context(), getTestProfileID(c), sessionID, req)
		if err != nil {
			_ = c.Error(err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": resp})
	})
	api.POST("/sessions/:sessionID/select", func(c *gin.Context) {
		sessionID, err := uuid.Parse(c.Param("sessionID"))
		if err != nil {
			_ = c.Error(ungerr.BadRequestError("invalid sessionID"))
			return
		}
		var req dto.CastVoteRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			_ = c.Error(ungerr.Wrap(err, "validation"))
			return
		}
		resp, err := decisionSessionSvc.Select(c.Request.Context(), getTestProfileID(c), sessionID, req)
		if err != nil {
			_ = c.Error(err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": resp})
	})
	api.POST("/sessions/:sessionID/finalize", func(c *gin.Context) {
		sessionID, err := uuid.Parse(c.Param("sessionID"))
		if err != nil {
			_ = c.Error(ungerr.BadRequestError("invalid sessionID"))
			return
		}
		resp, err := decisionSessionSvc.Finalize(c.Request.Context(), getTestProfileID(c), sessionID)
		if err != nil {
			_ = c.Error(err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": resp})
	})

	// Live updates (Task 5) — registered via the real handler, not a thin
	// inline closure, since this is the one endpoint that needs the actual
	// WS-upgrade/NATS-subscribe/forward code under test, using the real
	// testNATS connection (nil-safe, see NewDecisionSessionService call
	// above) rather than a mock.
	decisionSessionHandler := handler.NewDecisionSessionHandler(decisionSessionSvc, testNATS)
	api.GET("/sessions/:sessionID/live", decisionSessionHandler.HandleLive())
}

func getTestProfileID(c *gin.Context) uuid.UUID {
	val, _ := c.Get(appconstant.ContextProfileID.String())
	id, _ := uuid.Parse(val.(string))
	return id
}

func thinHandler(fn func(c *gin.Context) (any, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		data, err := fn(c)
		if err != nil {
			_ = c.Error(err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": data})
	}
}
