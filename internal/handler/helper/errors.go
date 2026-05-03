package helper

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"link-shortener/internal/handler/middleware"
	"link-shortener/internal/service"
)

func RespondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		RespondValidationErrors(c, ValidationErrors{
			"request": err.Error(),
		})
	case errors.Is(err, service.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
	case errors.Is(err, service.ErrConflict):
		RespondValidationErrors(c, ValidationErrors{
			"short_name": "short name already in use",
		})
	case errors.Is(err, context.Canceled):
		log.Printf("request canceled: %v", err)
	case errors.Is(err, context.DeadlineExceeded):
		log.Printf("request timeout: %v", err)
		c.JSON(http.StatusGatewayTimeout, gin.H{
			"error": "request timeout",
		})
	default:
		log.Printf("internal server error: %v", err)
		middleware.CaptureException(c, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "internal server error",
		})
	}
}
