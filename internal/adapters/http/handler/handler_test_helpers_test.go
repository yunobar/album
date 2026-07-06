package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/itsLeonB/ungerr"
	"github.com/yunobar/album/internal/appconstant"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// testErrorMiddleware is a minimal error handler for tests that converts AppError to proper status codes.
func testErrorMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Next()
		if ginErr := ctx.Errors.Last(); ginErr != nil {
			err := ginErr.Err
			if appErr, ok := err.(ungerr.AppError); ok {
				ctx.JSON(appErr.HttpStatus(), gin.H{"error": appErr.Error()})
				return
			}
			// UnknownError wrapping a validation/bind error → 422, matching the real error middleware
			if cause := ungerr.Unwrap(err); cause != nil {
				if _, ok := cause.(validator.ValidationErrors); ok {
					ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": cause.Error()})
					return
				}
			}
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": ginErr.Error()})
		}
	}
}

func injectProfileID(profileID uuid.UUID) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Set(appconstant.ContextProfileID.String(), profileID.String())
		ctx.Next()
	}
}
