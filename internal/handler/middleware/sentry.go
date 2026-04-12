package middleware

import (
	"fmt"

	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
)

func InitSentry(dsn string) {
	if dsn == "" {
		return
	}

	if err := sentry.Init(sentry.ClientOptions{
		Dsn:        dsn,
		EnableLogs: true,
	}); err != nil {
		fmt.Printf("Sentry initialization failed: %v\n", err)
	}
}

func Sentry() gin.HandlerFunc {
	return sentrygin.New(sentrygin.Options{})
}

func CaptureException(c *gin.Context, err error) {
	if hub := sentrygin.GetHubFromContext(c); hub != nil {
		hub.CaptureException(err)
		return
	}

	sentry.CaptureException(err)
}
