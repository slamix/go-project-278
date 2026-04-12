package helper

import (
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
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
	case errors.Is(err, service.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
	case errors.Is(err, service.ErrConflict):
		c.JSON(http.StatusConflict, gin.H{
			"error": err.Error(),
		})
	default:
		log.Printf("internal server error: %v", err)
		middleware.CaptureException(c, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "internal server error",
		})
	}
}
