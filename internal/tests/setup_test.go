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
	"github.com/yunobar/album/internal/appconstant"
	"github.com/yunobar/album/internal/core/logger"
	"github.com/yunobar/album/internal/domain/dto"
	"github.com/yunobar/album/internal/domain/entity"
	"github.com/yunobar/album/internal/domain/service"
	"github.com/yunobar/album/internal/testhelpers"
	"gorm.io/gorm"
)

var (
	testDB     *gorm.DB
	testRouter *gin.Engine

	testProfileID = uuid.MustParse("00000000-0000-0000-0000-000000000002")
	testUserID    = uuid.MustParse("00000000-0000-0000-0000-000000000001")
)

func TestMain(m *testing.M) {
	db, cleanup, err := testhelpers.SetupTestDB("../../.env.test")
	if err != nil {
		fmt.Fprintf(os.Stderr, "skipping feature tests: %v\n", err)
		os.Exit(0)
	}
	testDB = db
	defer cleanup()

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

func fakeAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(appconstant.ContextProfileID.String(), testProfileID.String())
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
